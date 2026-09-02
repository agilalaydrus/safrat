package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
)

var ErrNoProration = errors.New("subscription has no billable time remaining")

type PlanChangePreview struct {
	OperatorID       string
	OperatorName     string
	CurrentPlan      string
	NewPlan          string
	CurrentMonthly   int64
	NewMonthly       int64
	RemainingSeconds int32
	AdjustmentIDR    int64
	CreditBalanceIDR int64
	AccessUntil      time.Time
}

type PlanChange struct {
	OperatorID         string
	NewPlan            string
	ExpectedAdjustment int64
	Reason             string
	Confirmation       string
	ActorUserID        string
	IdempotencyKey     string
}

type PlanChangeResult struct {
	AdjustmentID     string
	OperatorID       string
	FromPlan         string
	ToPlan           string
	AdjustmentIDR    int64
	InvoiceID        string
	InvoiceAmountIDR int64
	Status           string
	CreditBalanceIDR int64
}

func proratedAmount(priceDelta int64, remainingSeconds int32) int64 {
	// Commercial proration is per calendar day. Besides being explainable on
	// an invoice, this keeps an approved preview stable while a person reads
	// and confirms it; a per-second amount could change between two clicks.
	remainingDays := int64((remainingSeconds + 86399) / 86400)
	if remainingDays > BillingPeriodDays {
		remainingDays = BillingPeriodDays
	}
	period := int64(BillingPeriodDays)
	numerator := priceDelta * remainingDays
	if numerator >= 0 {
		return (numerator + period/2) / period
	}
	return -((-numerator + period/2) / period)
}

func (r *SubscriptionRepository) PreviewPlanChange(ctx context.Context, operatorID, newPlan string) (PlanChangePreview, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return PlanChangePreview{}, apperror.ErrValidation
	}
	var result PlanChangePreview
	err = r.pool.QueryRow(ctx, `
		SELECT s.operator_id::text, o.name, s.plan::text, $2::plan::text,
		       old_price.monthly_idr, new_price.monthly_idr,
		       CEIL(EXTRACT(EPOCH FROM (s.access_until - NOW())))::int,
		       s.credit_balance_idr, s.access_until
		FROM subscriptions s
		JOIN operators o ON o.id=s.operator_id
		JOIN plan_prices old_price ON old_price.plan=s.plan
		JOIN plan_prices new_price ON new_price.plan=$2::plan
		WHERE s.operator_id=$1 AND s.plan<>$2::plan AND s.cancelled_at IS NULL
		  AND s.suspended_at IS NULL AND s.access_until>NOW()
		  AND NOT EXISTS (SELECT 1 FROM subscription_invoices i WHERE i.operator_id=s.operator_id AND i.status='PENDING')`, id, newPlan).
		Scan(&result.OperatorID, &result.OperatorName, &result.CurrentPlan, &result.NewPlan,
			&result.CurrentMonthly, &result.NewMonthly, &result.RemainingSeconds,
			&result.CreditBalanceIDR, &result.AccessUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanChangePreview{}, ErrNoProration
	}
	if err != nil {
		return PlanChangePreview{}, databaseError(err)
	}
	maxSeconds := int32(BillingPeriodDays * 24 * 60 * 60)
	if result.RemainingSeconds > maxSeconds {
		result.RemainingSeconds = maxSeconds
	}
	result.AdjustmentIDR = proratedAmount(result.NewMonthly-result.CurrentMonthly, result.RemainingSeconds)
	if result.AdjustmentIDR == 0 {
		return PlanChangePreview{}, ErrNoProration
	}
	return result, nil
}

func (r *SubscriptionRepository) ApplyPlanChange(ctx context.Context, change PlanChange) (PlanChangeResult, error) {
	operator, err := pgUUID(change.OperatorID)
	if err != nil || strings.TrimSpace(change.ActorUserID) == "" || strings.TrimSpace(change.Reason) == "" || strings.TrimSpace(change.IdempotencyKey) == "" {
		return PlanChangeResult{}, apperror.ErrValidation
	}
	fingerprint, _, err := requestFingerprint(change)
	if err != nil {
		return PlanChangeResult{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PlanChangeResult{}, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "subscription-plan:"+change.OperatorID); err != nil {
		return PlanChangeResult{}, databaseError(err)
	}

	if existing, found, err := existingPlanChange(ctx, tx, change.ActorUserID, change.IdempotencyKey, fingerprint); err != nil {
		return PlanChangeResult{}, err
	} else if found {
		return existing, tx.Commit(ctx)
	}

	var operatorName, currentPlan string
	var oldMonthly, newMonthly, credit int64
	var accessUntil time.Time
	var remaining int32
	err = tx.QueryRow(ctx, `
		SELECT o.name, s.plan::text, old_price.monthly_idr, new_price.monthly_idr,
		       s.credit_balance_idr, s.access_until,
		       LEAST($3::int, CEIL(EXTRACT(EPOCH FROM (s.access_until-NOW())))::int)
		FROM subscriptions s
		JOIN operators o ON o.id=s.operator_id
		JOIN plan_prices old_price ON old_price.plan=s.plan
		JOIN plan_prices new_price ON new_price.plan=$2::plan
		WHERE s.operator_id=$1 AND s.plan<>$2::plan AND s.cancelled_at IS NULL
		  AND s.suspended_at IS NULL AND s.access_until>NOW()
		  AND NOT EXISTS (SELECT 1 FROM subscription_invoices i WHERE i.operator_id=s.operator_id AND i.status='PENDING')
		FOR UPDATE OF s,o`, operator, change.NewPlan, BillingPeriodDays*24*60*60).
		Scan(&operatorName, &currentPlan, &oldMonthly, &newMonthly, &credit, &accessUntil, &remaining)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanChangeResult{}, ErrNoProration
	}
	if err != nil {
		return PlanChangeResult{}, databaseError(err)
	}
	if !strings.EqualFold(strings.TrimSpace(change.Confirmation), strings.TrimSpace(operatorName)) {
		return PlanChangeResult{}, apperror.ErrValidation
	}
	adjustment := proratedAmount(newMonthly-oldMonthly, remaining)
	if adjustment == 0 {
		return PlanChangeResult{}, ErrNoProration
	}
	if adjustment != change.ExpectedAdjustment {
		return PlanChangeResult{}, ErrBillingPreviewChanged
	}

	result := PlanChangeResult{OperatorID: change.OperatorID, FromPlan: currentPlan,
		ToPlan: change.NewPlan, AdjustmentIDR: adjustment, CreditBalanceIDR: credit}
	var invoiceID any
	if adjustment > 0 {
		invoice, err := issueProrationInvoice(ctx, tx, operator, change.NewPlan, adjustment, accessUntil)
		if err != nil {
			return PlanChangeResult{}, err
		}
		result.InvoiceID, result.InvoiceAmountIDR, result.Status = invoice.ID, invoice.Amount, "PENDING_PAYMENT"
		invoiceID = invoice.ID
	} else {
		creditAdded := -adjustment
		if _, err := tx.Exec(ctx, `UPDATE subscriptions SET plan=$2::plan, credit_balance_idr=credit_balance_idr+$3, updated_at=NOW() WHERE operator_id=$1`, operator, change.NewPlan, creditAdded); err != nil {
			return PlanChangeResult{}, databaseError(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE operators SET plan=$2::plan WHERE id=$1`, operator, change.NewPlan); err != nil {
			return PlanChangeResult{}, databaseError(err)
		}
		result.CreditBalanceIDR += creditAdded
		result.Status = "APPLIED"
	}
	kind := "PRORATION_DEBIT"
	if adjustment < 0 {
		kind = "PRORATION_CREDIT"
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO subscription_adjustments
		  (operator_id,invoice_id,kind,from_plan,to_plan,amount_idr,access_until_snapshot,
		   remaining_seconds,period_seconds,reason,requested_by,idempotency_key,request_fingerprint)
		VALUES ($1,$2,$3,$4::plan,$5::plan,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id::text`, operator, invoiceID, kind, currentPlan, change.NewPlan, adjustment,
		accessUntil, remaining, BillingPeriodDays*24*60*60, change.Reason, change.ActorUserID,
		change.IdempotencyKey, fingerprint).Scan(&result.AdjustmentID)
	if err != nil {
		return PlanChangeResult{}, databaseError(err)
	}
	if err := insertPlatformMutationAudit(ctx, tx, change.OperatorID, change.ActorUserID,
		"subscription_plan_change_requested", "subscription_adjustment", result.AdjustmentID,
		change.Reason, change.IdempotencyKey, fingerprint); err != nil {
		return PlanChangeResult{}, err
	}
	return result, databaseError(tx.Commit(ctx))
}

func issueProrationInvoice(ctx context.Context, tx pgx.Tx, operator any, plan string, base int64, accessUntil time.Time) (Invoice, error) {
	for attempt := 0; attempt < transferAttempts; attempt++ {
		suffix, err := randomSuffix()
		if err != nil {
			return Invoice{}, err
		}
		savepoint, err := tx.Begin(ctx)
		if err != nil {
			return Invoice{}, err
		}
		dueAt := time.Now().Add(InvoiceDueDays * 24 * time.Hour)
		if accessUntil.Before(dueAt) {
			dueAt = accessUntil
		}
		var invoice Invoice
		err = savepoint.QueryRow(ctx, `
			INSERT INTO subscription_invoices
			  (operator_id,plan,channel,base_amount_idr,amount_idr,period_start,period_end,due_at,purpose)
			VALUES ($1,$2::plan,'BANK_TRANSFER',$3,$4,NOW(),$5,$6,'PRORATION')
			RETURNING id::text,plan::text,status::text,channel::text,base_amount_idr,amount_idr,
			          period_start,period_end,due_at,COALESCE(checkout_url,'')`,
			operator, plan, base, base+suffix, accessUntil, dueAt).
			Scan(&invoice.ID, &invoice.Plan, &invoice.Status, &invoice.Channel, &invoice.BaseAmount,
				&invoice.Amount, &invoice.PeriodStart, &invoice.PeriodEnd, &invoice.DueAt, &invoice.CheckoutURL)
		if err == nil {
			return invoice, savepoint.Commit(ctx)
		}
		_ = savepoint.Rollback(ctx)
		if IsUniqueViolation(err, "subscription_invoices_one_pending_idx") {
			return Invoice{}, ErrPendingInvoice
		}
		if IsUniqueViolation(err, "subscription_invoices_transfer_amount_idx") || IsUniqueViolation(err, "subscription_invoices_transfer_daily_idx") {
			continue
		}
		return Invoice{}, databaseError(err)
	}
	return Invoice{}, ErrTransferAmountUnavailable
}

func existingPlanChange(ctx context.Context, tx pgx.Tx, actor, key, fingerprint string) (PlanChangeResult, bool, error) {
	var result PlanChangeResult
	var existingFingerprint string
	var invoiceID *string
	err := tx.QueryRow(ctx, `
		SELECT a.id::text,a.operator_id::text,a.from_plan::text,a.to_plan::text,a.amount_idr,
		       a.invoice_id::text,a.request_fingerprint,
		       COALESCE(i.amount_idr,0),s.credit_balance_idr
		FROM subscription_adjustments a
		JOIN subscriptions s ON s.operator_id=a.operator_id
		LEFT JOIN subscription_invoices i ON i.id=a.invoice_id
		WHERE a.requested_by=$1 AND a.idempotency_key=$2`, actor, key).
		Scan(&result.AdjustmentID, &result.OperatorID, &result.FromPlan, &result.ToPlan,
			&result.AdjustmentIDR, &invoiceID, &existingFingerprint, &result.InvoiceAmountIDR, &result.CreditBalanceIDR)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanChangeResult{}, false, nil
	}
	if err != nil {
		return PlanChangeResult{}, false, databaseError(err)
	}
	if existingFingerprint != fingerprint {
		return PlanChangeResult{}, false, apperror.ErrConflict
	}
	if invoiceID != nil {
		result.InvoiceID = *invoiceID
		result.Status = "PENDING_PAYMENT"
	} else {
		result.Status = "APPLIED"
	}
	return result, true, nil
}

func (p PlanChangePreview) String() string {
	return fmt.Sprintf("%s %s→%s %d", p.OperatorID, p.CurrentPlan, p.NewPlan, p.AdjustmentIDR)
}
