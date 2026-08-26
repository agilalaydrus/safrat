package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// TrialDays is how long a new operator can use the dashboard before paying.
	TrialDays = 3
	// BillingPeriodDays is the length a paid invoice buys.
	BillingPeriodDays = 30
	// InvoiceDueDays is how long a pending invoice stays payable. A bank
	// transfer needs time to clear, so this is not the same as the access
	// deadline.
	InvoiceDueDays = 7
	// transferSuffixMax bounds the unique amount suffix. 999 concurrent unpaid
	// transfers at the same price is far beyond plausible; the retry below
	// turns the remaining collision risk into a clean error rather than a
	// misattributed payment.
	transferSuffixMax = 999
	transferAttempts  = 24
)

// ErrTransferAmountUnavailable means no unique amount could be found. It is a
// signal to fall back to the payment gateway, never to reuse an amount: the
// suffix is the only thing tying a bank mutation to an invoice.
var ErrTransferAmountUnavailable = errors.New("no unique transfer amount available")

type SubscriptionRepository struct {
	pool *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{pool: pool}
}

// Access is the single answer to "may this operator use the dashboard?".
type Access struct {
	Plan        string
	Status      string
	AccessUntil time.Time
	// Allowed is derived from time, never from status alone: a status left
	// stale by a missed sweep must not hand out free access.
	Allowed bool
}

type Invoice struct {
	ID          string
	Plan        string
	Status      string
	Channel     string
	BaseAmount  int64
	Amount      int64
	PeriodStart time.Time
	PeriodEnd   time.Time
	DueAt       time.Time
	CheckoutURL string
}

// EnsureForOperator starts a trial the first time an operator is seen, and is
// a no-op afterwards so it can be called from any entry point without
// resetting an existing subscription.
func (r *SubscriptionRepository) EnsureForOperator(ctx context.Context, operatorID string) error {
	id, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO subscriptions (operator_id, plan, status, access_until)
		SELECT id, plan, 'TRIALING', NOW() + make_interval(days => $2::int)
		FROM operators WHERE id = $1
		ON CONFLICT (operator_id) DO NOTHING`, id, TrialDays)
	return err
}

func (r *SubscriptionRepository) GetAccess(ctx context.Context, operatorID string) (Access, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return Access{}, apperror.ErrValidation
	}
	var access Access
	err = r.pool.QueryRow(ctx, `
		SELECT plan::text, status::text, access_until, access_until > NOW()
		FROM subscriptions WHERE operator_id = $1`, id).
		Scan(&access.Plan, &access.Status, &access.AccessUntil, &access.Allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return Access{}, apperror.ErrNotFound
	}
	return access, err
}

// PlanPrice reads the monthly price. Prices live in the database so they can be
// corrected without a deploy.
func (r *SubscriptionRepository) PlanPrice(ctx context.Context, plan string) (int64, error) {
	var amount int64
	err := r.pool.QueryRow(ctx, `SELECT monthly_idr FROM plan_prices WHERE plan::text = $1`, plan).Scan(&amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperror.ErrNotFound
	}
	return amount, err
}

// IssueBankTransferInvoice creates an invoice whose amount carries a unique
// suffix, so an incoming bank mutation identifies exactly one invoice.
//
// Uniqueness is enforced by a partial unique index rather than by checking
// first and inserting after: two operators asking at the same moment would
// both see the amount as free. Here the database rejects the loser and we try
// another suffix.
func (r *SubscriptionRepository) IssueBankTransferInvoice(ctx context.Context, operatorID, plan string) (Invoice, error) {
	base, err := r.PlanPrice(ctx, plan)
	if err != nil {
		return Invoice{}, err
	}
	id, err := pgUUID(operatorID)
	if err != nil {
		return Invoice{}, apperror.ErrValidation
	}
	for attempt := 0; attempt < transferAttempts; attempt++ {
		suffix, err := randomSuffix()
		if err != nil {
			return Invoice{}, err
		}
		invoice, err := r.insertInvoice(ctx, id, plan, "BANK_TRANSFER", base, base+suffix, "", "")
		if err == nil {
			return invoice, nil
		}
		if isUniqueViolation(err, "subscription_invoices_transfer_amount_idx") {
			continue
		}
		return Invoice{}, err
	}
	return Invoice{}, ErrTransferAmountUnavailable
}

// IssueGatewayInvoice records an invoice paid through Xendit's hosted
// checkout, which covers QRIS, cards, virtual accounts and e-wallets.
func (r *SubscriptionRepository) IssueGatewayInvoice(ctx context.Context, operatorID, plan, externalID, checkoutURL string) (Invoice, error) {
	base, err := r.PlanPrice(ctx, plan)
	if err != nil {
		return Invoice{}, err
	}
	id, err := pgUUID(operatorID)
	if err != nil {
		return Invoice{}, apperror.ErrValidation
	}
	// No unique suffix: the gateway identifies the payment by its own id, so
	// charging a round figure is clearer for the payer.
	return r.insertInvoice(ctx, id, plan, "GATEWAY", base, base, externalID, checkoutURL)
}

func (r *SubscriptionRepository) insertInvoice(ctx context.Context, operatorID any, plan, channel string, base, amount int64, externalID, checkoutURL string) (Invoice, error) {
	var invoice Invoice
	var external, checkout *string
	if externalID != "" {
		external = &externalID
	}
	if checkoutURL != "" {
		checkout = &checkoutURL
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO subscription_invoices
			(operator_id, plan, channel, base_amount_idr, amount_idr, period_start, period_end, due_at, external_id, checkout_url)
		VALUES ($1, $2::plan, $3::payment_channel, $4, $5,
			NOW(), NOW() + make_interval(days => $6::int), NOW() + make_interval(days => $7::int), $8, $9)
		RETURNING id::text, plan::text, status::text, channel::text, base_amount_idr, amount_idr,
			period_start, period_end, due_at, COALESCE(checkout_url, '')`,
		operatorID, plan, channel, base, amount, BillingPeriodDays, InvoiceDueDays, external, checkout).
		Scan(&invoice.ID, &invoice.Plan, &invoice.Status, &invoice.Channel, &invoice.BaseAmount,
			&invoice.Amount, &invoice.PeriodStart, &invoice.PeriodEnd, &invoice.DueAt, &invoice.CheckoutURL)
	return invoice, err
}

// FindPayableByAmount identifies the invoice an incoming bank mutation settles.
// It matches only unpaid, unexpired transfers, so a stale amount from a closed
// invoice cannot credit anyone.
func (r *SubscriptionRepository) FindPayableByAmount(ctx context.Context, amount int64) (string, string, error) {
	var invoiceID, operatorID string
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, operator_id::text
		FROM subscription_invoices
		WHERE amount_idr = $1 AND status = 'PENDING' AND channel = 'BANK_TRANSFER'`, amount).
		Scan(&invoiceID, &operatorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", apperror.ErrNotFound
	}
	return invoiceID, operatorID, err
}

// MarkPaid settles an invoice and extends access in the same transaction, so
// money can never be recorded without the access it bought.
//
// Extension is from whichever is later, now or the current expiry, so paying
// early adds to the remaining time instead of discarding it.
func (r *SubscriptionRepository) MarkPaid(ctx context.Context, invoiceID string) error {
	target, err := pgUUID(invoiceID)
	if err != nil {
		return apperror.ErrValidation
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var operatorID string
	var plan string
	err = tx.QueryRow(ctx, `
		UPDATE subscription_invoices SET status = 'PAID', paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'PENDING'
		RETURNING operator_id::text, plan::text`, target).Scan(&operatorID, &plan)
	if errors.Is(err, pgx.ErrNoRows) {
		// Already settled, or cancelled. Treated as success so a webhook or a
		// mutation sweep can safely deliver the same payment twice.
		return nil
	}
	if err != nil {
		return err
	}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE subscriptions
		SET status = 'ACTIVE', plan = $2::plan,
		    access_until = GREATEST(access_until, NOW()) + make_interval(days => $3::int),
		    updated_at = NOW()
		WHERE operator_id = $1`, operator, plan, BillingPeriodDays); err != nil {
		return err
	}
	// Paying for a plan is what grants it — the operators row is what the rest
	// of the system reads for entitlements.
	if _, err := tx.Exec(ctx, `UPDATE operators SET plan = $2::plan WHERE id = $1`, operator, plan); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ExpireOverdueInvoices releases the amounts of invoices nobody paid, so those
// unique suffixes become available again.
func (r *SubscriptionRepository) ExpireOverdueInvoices(ctx context.Context) (int64, error) {
	command, err := r.pool.Exec(ctx, `
		UPDATE subscription_invoices SET status = 'EXPIRED', updated_at = NOW()
		WHERE status = 'PENDING' AND due_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

// MarkLapsed moves subscriptions whose access has run out to PAST_DUE. Access
// itself is already governed by access_until, so this only keeps the status
// readable; it never grants or removes access on its own.
func (r *SubscriptionRepository) MarkLapsed(ctx context.Context) (int64, error) {
	command, err := r.pool.Exec(ctx, `
		UPDATE subscriptions SET status = 'PAST_DUE', updated_at = NOW()
		WHERE access_until < NOW() AND status IN ('TRIALING', 'ACTIVE')`)
	if err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

func randomSuffix() (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(transferSuffixMax))
	if err != nil {
		return 0, fmt.Errorf("generate transfer suffix: %w", err)
	}
	return n.Int64() + 1, nil
}

// isUniqueViolation reports whether err is a unique-constraint failure on the
// named index. Matching the specific index matters: any other unique violation
// is a real bug and must not be silently retried.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
