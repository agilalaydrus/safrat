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
}

var defaultDunningSettings = DunningSettings{ReminderDays: []int{1, 7, 14}, SuspendAfterDays: 21, TrialDays: 10}

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
		}
	}
	return settings, rows.Err()
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
		SELECT s.operator_id::text, o.name, COALESCE(o.email, ''), s.access_until,
		       FLOOR(EXTRACT(EPOCH FROM (NOW() - s.access_until)) / 86400)::int AS days_overdue,
		       COALESCE((
		         SELECT i.amount_idr FROM subscription_invoices i
		         WHERE i.operator_id = s.operator_id AND i.status = 'PENDING'
		         ORDER BY i.created_at DESC LIMIT 1
		       ), 0)
		FROM subscriptions s
		JOIN operators o ON o.id = s.operator_id
		WHERE s.cancelled_at IS NULL
		  AND s.access_until < NOW()
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
