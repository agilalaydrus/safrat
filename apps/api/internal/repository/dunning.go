package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
)

// DunningSettings are the numbers a person changes, read from
// platform_settings rather than compiled in. A commercial figure that needs a
// release to correct stays wrong for months.
type DunningSettings struct {
	ReminderDays     []int
	SuspendAfterDays int
	TrialDays        int
	GracePeriodDays  int
}

var defaultDunningSettings = DunningSettings{ReminderDays: []int{1, 7, 14}, SuspendAfterDays: 21, TrialDays: 10, GracePeriodDays: 0}

// Settings reads the platform settings, falling back to defaults for anything
// missing or unparseable. A malformed row must not stop billing from running.
func (r *SubscriptionRepository) Settings(ctx context.Context) (DunningSettings, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, value FROM platform_settings`)
	if err != nil {
		return defaultDunningSettings, err
	}
	defer rows.Close()

	settings := defaultDunningSettings
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return defaultDunningSettings, err
		}
		switch key {
		case "dunning_days":
			if days := parseDayList(value); len(days) > 0 {
				settings.ReminderDays = days
			}
		case "suspend_after_days":
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n > 0 {
				settings.SuspendAfterDays = n
			}
		case "trial_days":
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n > 0 {
				settings.TrialDays = n
			}
		case "grace_period_days":
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n >= 0 && n <= 90 {
				settings.GracePeriodDays = n
			}
		}
	}
	return settings, rows.Err()
}

type TrialDaysChange struct {
	Days           int32
	Reason         string
	Confirmation   string
	ActorUserID    string
	IdempotencyKey string
}

// SetTrialDays changes the offer for subscriptions created after this
// transaction. Existing access_until values are deliberately untouched, so a
// shorter offer cannot take days away from a trial already accepted.
func (r *SubscriptionRepository) SetTrialDays(ctx context.Context, change TrialDaysChange) (int32, error) {
	if change.Days < 1 || change.Days > 90 || strings.TrimSpace(change.Reason) == "" ||
		!strings.EqualFold(strings.TrimSpace(change.Confirmation), "TRIAL") ||
		strings.TrimSpace(change.ActorUserID) == "" || strings.TrimSpace(change.IdempotencyKey) == "" {
		return 0, apperror.ErrValidation
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return 0, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('platform-setting:trial-days',0))`); err != nil {
		return 0, databaseError(err)
	}
	fingerprint, _, err := requestFingerprint(change)
	if err != nil {
		return 0, err
	}
	duplicate, err := checkPlatformMutationKey(ctx, tx, change.ActorUserID, "trial_days_changed", change.IdempotencyKey, fingerprint)
	if err != nil {
		return 0, err
	}
	if !duplicate {
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform_settings (key,value,updated_by,updated_at)
			VALUES ('trial_days',$1,$2,NOW())
			ON CONFLICT (key) DO UPDATE SET
			  value=EXCLUDED.value, updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			strconv.Itoa(int(change.Days)), change.ActorUserID); err != nil {
			return 0, databaseError(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO audit_logs (operator_id,user_id,action,entity_type,entity_id,metadata)
			VALUES (NULL,$1,'trial_days_changed','platform_setting','trial_days',$2)`,
			change.ActorUserID, map[string]any{
				"message": change.Reason, "idempotency_key": change.IdempotencyKey,
				"request_fingerprint": fingerprint, "trial_days": change.Days,
			}); err != nil {
			return 0, databaseError(err)
		}
	}
	var effective int32
	if err := tx.QueryRow(ctx, `SELECT platform_trial_days()`).Scan(&effective); err != nil {
		return 0, databaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, databaseError(err)
	}
	return effective, nil
}

func parseDayList(value string) []int {
	days := make([]int, 0, 4)
	for _, part := range strings.Split(value, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && n > 0 {
			days = append(days, n)
		}
	}
	return days
}

type GracePeriodChange struct {
	OperatorID     string
	Days           *int32
	UseDefault     bool
	Reason         string
	Confirmation   string
	ActorUserID    string
	IdempotencyKey string
}

type GracePeriodResult struct {
	OperatorID    string
	EffectiveDays int32
	OverrideDays  *int32
}

func (r *SubscriptionRepository) SetGracePeriod(ctx context.Context, change GracePeriodChange) (GracePeriodResult, error) {
	if strings.TrimSpace(change.Reason) == "" || strings.TrimSpace(change.ActorUserID) == "" ||
		strings.TrimSpace(change.IdempotencyKey) == "" || (change.Days != nil && (*change.Days < 0 || *change.Days > 90)) {
		return GracePeriodResult{}, apperror.ErrValidation
	}
	global := strings.TrimSpace(change.OperatorID) == ""
	if global && (change.UseDefault || change.Days == nil || !strings.EqualFold(strings.TrimSpace(change.Confirmation), "GLOBAL")) {
		return GracePeriodResult{}, apperror.ErrValidation
	}
	if !global && ((!change.UseDefault && change.Days == nil) || (change.UseDefault && change.Days != nil)) {
		return GracePeriodResult{}, apperror.ErrValidation
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return GracePeriodResult{}, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	scope := "global"
	var operator any
	if !global {
		id, parseErr := pgUUID(change.OperatorID)
		if parseErr != nil {
			return GracePeriodResult{}, apperror.ErrValidation
		}
		operator = id
		scope = change.OperatorID
		var name string
		if err := tx.QueryRow(ctx, `SELECT name FROM operators WHERE id=$1 FOR UPDATE`, id).Scan(&name); errors.Is(err, pgx.ErrNoRows) {
			return GracePeriodResult{}, apperror.ErrNotFound
		} else if err != nil {
			return GracePeriodResult{}, databaseError(err)
		} else if !strings.EqualFold(strings.TrimSpace(change.Confirmation), strings.TrimSpace(name)) {
			return GracePeriodResult{}, apperror.ErrValidation
		}
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "subscription-grace:"+scope); err != nil {
		return GracePeriodResult{}, databaseError(err)
	}
	fingerprint, _, err := requestFingerprint(change)
	if err != nil {
		return GracePeriodResult{}, err
	}
	duplicate, err := checkPlatformMutationKey(ctx, tx, change.ActorUserID, "subscription_grace_changed", change.IdempotencyKey, fingerprint)
	if err != nil {
		return GracePeriodResult{}, err
	}
	if duplicate {
		return currentGracePeriod(ctx, tx, change.OperatorID)
	}

	if global {
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform_settings (key,value,updated_by,updated_at)
			VALUES ('grace_period_days',$1,$2,NOW())
			ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			strconv.Itoa(int(*change.Days)), change.ActorUserID); err != nil {
			return GracePeriodResult{}, databaseError(err)
		}
	} else {
		var value any
		if !change.UseDefault {
			value = *change.Days
		}
		command, err := tx.Exec(ctx, `UPDATE subscriptions SET grace_period_days=$2, updated_at=NOW() WHERE operator_id=$1`, operator, value)
		if err != nil {
			return GracePeriodResult{}, databaseError(err)
		}
		if command.RowsAffected() == 0 {
			return GracePeriodResult{}, apperror.ErrNotFound
		}
	}
	metadata := map[string]any{
		"message": change.Reason, "idempotency_key": change.IdempotencyKey,
		"request_fingerprint": fingerprint, "scope": scope,
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (operator_id,user_id,action,entity_type,entity_id,metadata)
		VALUES ($1,$2,'subscription_grace_changed','subscription_grace',$3,$4)`,
		operator, change.ActorUserID, scope, metadata); err != nil {
		return GracePeriodResult{}, databaseError(err)
	}
	result, err := currentGracePeriod(ctx, tx, change.OperatorID)
	if err != nil {
		return GracePeriodResult{}, err
	}
	return result, databaseError(tx.Commit(ctx))
}

func currentGracePeriod(ctx context.Context, tx pgx.Tx, operatorID string) (GracePeriodResult, error) {
	if strings.TrimSpace(operatorID) == "" {
		var days int32
		if err := tx.QueryRow(ctx, `SELECT platform_grace_period_days()`).Scan(&days); err != nil {
			return GracePeriodResult{}, databaseError(err)
		}
		return GracePeriodResult{EffectiveDays: days}, nil
	}
	id, err := pgUUID(operatorID)
	if err != nil {
		return GracePeriodResult{}, apperror.ErrValidation
	}
	result := GracePeriodResult{OperatorID: operatorID}
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(grace_period_days, platform_grace_period_days()), grace_period_days
		FROM subscriptions WHERE operator_id=$1`, id).Scan(&result.EffectiveDays, &result.OverrideDays); errors.Is(err, pgx.ErrNoRows) {
		return GracePeriodResult{}, apperror.ErrNotFound
	} else if err != nil {
		return GracePeriodResult{}, databaseError(err)
	}
	return result, nil
}

// DunningStep is one reminder that is due and has not been sent.
type DunningStep struct {
	OperatorID   string
	OperatorName string
	Email        string
	LapsedAt     time.Time
	Stage        string
	DaysOverdue  int
	AmountIDR    int64
	Suspend      bool
}

// DueDunning returns the reminders that should go out now.
//
// Anchored on access_until rather than on an invoice: the invoice that billed
// this period expires the moment access does, and the sweep issues a fresh one,
// so an invoice is not stable enough to hang a multi-week sequence from.
//
// Cancelled subscriptions are excluded. Somebody who has already said they are
// leaving does not need three more demands.
func (r *SubscriptionRepository) DueDunning(ctx context.Context, settings DunningSettings) ([]DunningStep, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.operator_id::text, o.name, COALESCE(o.email, ''),
		       subscription_effective_access_until(s.access_until, s.grace_period_days),
		       FLOOR(EXTRACT(EPOCH FROM (
		         NOW() - subscription_effective_access_until(s.access_until, s.grace_period_days)
		       ) / 86400))::int AS days_overdue,
		       COALESCE((
		         SELECT i.amount_idr FROM subscription_invoices i
		         WHERE i.operator_id = s.operator_id AND i.status = 'PENDING'
		         ORDER BY i.created_at DESC LIMIT 1
		       ), 0)
		FROM subscriptions s
		JOIN operators o ON o.id = s.operator_id
		WHERE s.cancelled_at IS NULL
		  AND subscription_effective_access_until(s.access_until, s.grace_period_days) < NOW()
		ORDER BY s.access_until ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		step DunningStep
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.step.OperatorID, &c.step.OperatorName, &c.step.Email,
			&c.step.LapsedAt, &c.step.DaysOverdue, &c.step.AmountIDR); err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	steps := make([]DunningStep, 0, len(candidates))
	for _, c := range candidates {
		stage, suspend := stageFor(c.step.DaysOverdue, settings)
		if stage == "" {
			continue
		}
		c.step.Stage = stage
		c.step.Suspend = suspend
		steps = append(steps, c.step)
	}
	return steps, nil
}

// stageFor picks the furthest stage the elapsed time has reached, not the
// nearest. A worker that was down for a week must not send H+1 today and H+7
// next week — the agency is three weeks overdue and needs to hear that, not a
// reminder that is already stale.
func stageFor(daysOverdue int, settings DunningSettings) (string, bool) {
	if daysOverdue >= settings.SuspendAfterDays {
		return "SUSPEND", true
	}
	stage := ""
	for _, day := range settings.ReminderDays {
		if daysOverdue >= day {
			stage = fmt.Sprintf("H%d", day)
		}
	}
	return stage, false
}

// RecordDunning claims one step and enqueues its message in the same
// transaction.
//
// The primary key on (operator_id, lapsed_at, stage) is the whole guarantee:
// a second run collides, reports false, and sends nothing. Delivery through the
// outbox is at-least-once, so nothing downstream may assume it ran once.
func (r *SubscriptionRepository) RecordDunning(ctx context.Context, step DunningStep) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	operator, err := pgUUID(step.OperatorID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	command, err := tx.Exec(ctx, `
		INSERT INTO dunning_log (operator_id, lapsed_at, stage)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, operator, step.LapsedAt, step.Stage)
	if err != nil {
		return false, databaseError(err)
	}
	if command.RowsAffected() == 0 {
		return false, nil
	}

	if step.Suspend {
		// Suspension works through time, matching how access is granted:
		// access_until is already in the past, so nothing is taken away here.
		// The columns only record that this is deliberate rather than merely
		// unpaid — the two need different conversations.
		if _, err := tx.Exec(ctx, `
			UPDATE subscriptions
			SET suspended_at = NOW(), suspended_reason = $2, updated_at = NOW()
			WHERE operator_id = $1 AND suspended_at IS NULL`,
			operator, fmt.Sprintf("tagihan langganan belum dibayar %d hari", step.DaysOverdue)); err != nil {
			return false, databaseError(err)
		}
	}

	payload := domain.SubscriptionDunningPayload{
		Stage: step.Stage, DaysOverdue: step.DaysOverdue,
		AmountIDR: step.AmountIDR, OperatorName: step.OperatorName, Email: step.Email,
		Suspended: step.Suspend,
	}
	key := fmt.Sprintf("dunning:%s:%d:%s", step.OperatorID, step.LapsedAt.Unix(), step.Stage)
	if _, err := NewOutboxRepository(db.New(r.pool)).EnqueueIdempotentTx(ctx, tx,
		step.OperatorID, domain.EventSubscriptionDunning, "", key, payload); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// ClearSuspension lifts a suspension. Called wherever a payment lands, so an
// agency that has paid is not left locked out waiting for somebody to notice.
func clearSuspension(ctx context.Context, tx pgx.Tx, operatorID string) error {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE subscriptions SET suspended_at = NULL, suspended_reason = '', updated_at = NOW()
		WHERE operator_id = $1 AND suspended_at IS NOT NULL`, operator)
	return err
}

// VoidInvoice cancels an invoice that should not have been issued, keeping the
// row. Deleting it would leave a hole in the billing history exactly where
// somebody will later look, and CANCELLED already returns the unique transfer
// amount to the pool.
func (r *SubscriptionRepository) VoidInvoice(ctx context.Context, invoiceID, reason, actorUserID string) error {
	target, err := pgUUID(invoiceID)
	if err != nil {
		return apperror.ErrValidation
	}
	if strings.TrimSpace(reason) == "" {
		return apperror.ErrValidation
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var operatorID string
	err = tx.QueryRow(ctx, `
		UPDATE subscription_invoices
		SET status = 'CANCELLED', voided_at = NOW(), voided_reason = $2, updated_at = NOW()
		WHERE id = $1 AND status = 'PENDING'
		RETURNING operator_id::text`, target, strings.TrimSpace(reason)).Scan(&operatorID)
	if errors.Is(err, pgx.ErrNoRows) {
		// A paid invoice is not voidable: money moved, and the record of that
		// must stand. Cancelling one twice is a no-op rather than an error.
		return apperror.ErrFailedPrecondition
	}
	if err != nil {
		return databaseError(err)
	}
	if err := insertPlatformMutationAudit(ctx, tx, operatorID, actorUserID, "subscription_invoice_voided",
		"subscription_invoice", invoiceID, strings.TrimSpace(reason), "", ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SubscriptionInvoiceRow is one billing line, across tenants or within one.
type SubscriptionInvoiceRow struct {
	ID           string
	OperatorID   string
	OperatorName string
	Plan         string
	Status       string
	Channel      string
	AmountIDR    int64
	DueAt        time.Time
	PaidAt       *time.Time
	VoidedAt     *time.Time
	VoidedReason string
	CreatedAt    time.Time
}

// ListSubscriptionInvoices returns billing history. An empty operatorID lists
// across every tenant, which is what the platform screen wants; passing one
// narrows to that tenant's page.
//
// Voided invoices are included rather than filtered out. They are part of the
// record — an invoice that was issued and withdrawn is a thing that happened,
// and hiding it is how billing history stops answering questions.
func (r *SubscriptionRepository) ListSubscriptionInvoices(ctx context.Context, operatorID string, limit int32) ([]SubscriptionInvoiceRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	operator, err := nullableUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, `
		SELECT i.id::text, i.operator_id::text, o.name, i.plan::text, i.status::text,
		       i.channel::text, i.amount_idr, i.due_at, i.paid_at, i.voided_at,
		       i.voided_reason, i.created_at
		FROM subscription_invoices i
		JOIN operators o ON o.id = i.operator_id
		WHERE ($1::uuid IS NULL OR i.operator_id = $1)
		ORDER BY i.created_at DESC
		LIMIT $2`, operator, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invoices := make([]SubscriptionInvoiceRow, 0)
	for rows.Next() {
		var row SubscriptionInvoiceRow
		if err := rows.Scan(&row.ID, &row.OperatorID, &row.OperatorName, &row.Plan, &row.Status,
			&row.Channel, &row.AmountIDR, &row.DueAt, &row.PaidAt, &row.VoidedAt,
			&row.VoidedReason, &row.CreatedAt); err != nil {
			return nil, err
		}
		invoices = append(invoices, row)
	}
	return invoices, rows.Err()
}
