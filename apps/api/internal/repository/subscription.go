package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
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
	// transferSuffixMax bounds the unique amount suffix. Since amounts are
	// unique per day and not merely among unpaid invoices, this is also the
	// ceiling on transfers issuable for one plan in one day. 999 is far beyond
	// current volume, but it is a real ceiling rather than a theoretical one —
	// exhaustion returns ErrTransferAmountUnavailable so the caller can offer
	// the gateway instead of reusing a code.
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
		// Either index rejecting means this suffix is taken — by a live invoice,
		// or by one already issued today. Both are retryable; anything else is a
		// real fault and must not be swallowed by another attempt.
		// Another request for this operator won the race and already holds the
		// only pending invoice. Hand back theirs rather than failing or
		// retrying: the caller asked for an invoice and there is one.
		if IsUniqueViolation(err, "subscription_invoices_one_pending_idx") {
			return r.PendingInvoice(ctx, operatorID)
		}
		// A suffix already taken — by a live invoice, or by one issued today.
		// Both are retryable; anything else is a real fault and must not be
		// swallowed by another attempt.
		if IsUniqueViolation(err, "subscription_invoices_transfer_amount_idx") ||
			IsUniqueViolation(err, "subscription_invoices_transfer_daily_idx") {
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
	invoice, err := r.insertInvoice(ctx, id, plan, "GATEWAY", base, base, externalID, checkoutURL)
	if IsUniqueViolation(err, "subscription_invoices_one_pending_idx") {
		return r.PendingInvoice(ctx, operatorID)
	}
	return invoice, err
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
// The operator name comes back too, so a caller confirming a payment can say
// whose it was. Read here rather than looked up afterwards: once the invoice is
// settled it is no longer pending, and a second query would find nothing.
func (r *SubscriptionRepository) FindPayableByAmount(ctx context.Context, amount int64) (invoiceID, operatorID, operatorName string, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT i.id::text, i.operator_id::text, o.name
		FROM subscription_invoices i
		JOIN operators o ON o.id = i.operator_id
		WHERE i.amount_idr = $1 AND i.status = 'PENDING' AND i.channel = 'BANK_TRANSFER'`, amount).
		Scan(&invoiceID, &operatorID, &operatorName)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", apperror.ErrNotFound
	}
	return invoiceID, operatorID, operatorName, err
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
	if err := extendAccess(ctx, tx, operatorID, plan); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// extendAccess grants what a payment bought.
//
// Shared by both settlement paths — a person confirming an amount and the
// matcher settling a bank credit — because they are the same event reached two
// ways. Two copies would drift, and the half that drifted would be the one
// granting access nobody paid for or withholding access somebody did.
//
// Extension is from whichever is later, now or the current expiry, so paying
// early adds to the remaining time instead of discarding it.
func extendAccess(ctx context.Context, tx pgx.Tx, operatorID, plan string) error {
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
	// Payment ends a suspension wherever it lands, at any stage, including
	// after the sequence ran to the end. An agency that has paid must not wait
	// for somebody here to notice — and this sits inside extendAccess for the
	// same reason the rest of it does: both settlement paths reach it, and a
	// second copy is the one that would drift.
	return clearSuspension(ctx, tx, operatorID)
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

// IsUniqueViolation reports whether err is a unique-constraint failure on the
// named index. Matching the specific index matters: any other unique violation
// is a real bug and must not be silently retried.
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// PendingInvoice returns the operator's outstanding invoice, if any. One at a
// time: a second would put two unique amounts in play for the same operator,
// and a transfer against the older one would look unmatched.
func (r *SubscriptionRepository) PendingInvoice(ctx context.Context, operatorID string) (Invoice, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return Invoice{}, apperror.ErrValidation
	}
	var invoice Invoice
	err = r.pool.QueryRow(ctx, `
		SELECT id::text, plan::text, status::text, channel::text, base_amount_idr, amount_idr,
		       period_start, period_end, due_at, COALESCE(checkout_url, '')
		FROM subscription_invoices
		WHERE operator_id = $1 AND status = 'PENDING' AND due_at > NOW()
		ORDER BY created_at DESC LIMIT 1`, id).
		Scan(&invoice.ID, &invoice.Plan, &invoice.Status, &invoice.Channel, &invoice.BaseAmount,
			&invoice.Amount, &invoice.PeriodStart, &invoice.PeriodEnd, &invoice.DueAt, &invoice.CheckoutURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, apperror.ErrNotFound
	}
	return invoice, err
}

// ListInvoices returns the operator's billing history, newest first.
func (r *SubscriptionRepository) ListInvoices(ctx context.Context, operatorID string, limit int32) ([]Invoice, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if limit <= 0 || limit > 100 {
		limit = 24
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, plan::text, status::text, channel::text, base_amount_idr, amount_idr,
		       period_start, period_end, due_at, COALESCE(checkout_url, '')
		FROM subscription_invoices WHERE operator_id = $1
		ORDER BY created_at DESC LIMIT $2`, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invoices := make([]Invoice, 0, limit)
	for rows.Next() {
		var invoice Invoice
		if err := rows.Scan(&invoice.ID, &invoice.Plan, &invoice.Status, &invoice.Channel, &invoice.BaseAmount,
			&invoice.Amount, &invoice.PeriodStart, &invoice.PeriodEnd, &invoice.DueAt, &invoice.CheckoutURL); err != nil {
			return nil, err
		}
		invoices = append(invoices, invoice)
	}
	return invoices, rows.Err()
}

// GetAccessByOrgID resolves access from a Better Auth organization id, which is
// what the auth interceptor holds. Subscriptions are keyed by operators.id, so
// the join belongs here rather than forcing every caller to look the operator
// up first.
func (r *SubscriptionRepository) GetAccessByOrgID(ctx context.Context, orgID string) (Access, error) {
	if strings.TrimSpace(orgID) == "" {
		return Access{}, apperror.ErrValidation
	}
	var access Access
	err := r.pool.QueryRow(ctx, `
		SELECT s.plan::text, s.status::text, s.access_until, s.access_until > NOW()
		FROM subscriptions s
		JOIN operators o ON o.id = s.operator_id
		WHERE o.better_auth_org_id = $1`, orgID).
		Scan(&access.Plan, &access.Status, &access.AccessUntil, &access.Allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return Access{}, apperror.ErrNotFound
	}
	return access, err
}

// MarkPaidByExternalID settles the invoice a gateway payment belongs to. The
// gateway identifies the payment by its own id, which is stored on the invoice
// when it is issued.
func (r *SubscriptionRepository) MarkPaidByExternalID(ctx context.Context, externalID string) error {
	if strings.TrimSpace(externalID) == "" {
		return apperror.ErrValidation
	}
	var invoiceID string
	err := r.pool.QueryRow(ctx, `SELECT id::text FROM subscription_invoices WHERE external_id = $1`, externalID).Scan(&invoiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.ErrNotFound
	}
	if err != nil {
		return err
	}
	return r.MarkPaid(ctx, invoiceID)
}

// CloseByExternalID marks a gateway invoice expired or failed. It never touches
// a settled invoice: a late "expired" delivery must not undo a payment.
func (r *SubscriptionRepository) CloseByExternalID(ctx context.Context, externalID, status string) error {
	if strings.TrimSpace(externalID) == "" || (status != "EXPIRED" && status != "CANCELLED") {
		return apperror.ErrValidation
	}
	command, err := r.pool.Exec(ctx, `
		UPDATE subscription_invoices SET status = $2::invoice_status, updated_at = NOW()
		WHERE external_id = $1 AND status = 'PENDING'`, externalID, status)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// IsForeignKeyViolation reports whether err is a foreign key failure on the
// named constraint, so callers can turn it into an explanation rather than an
// internal error.
func IsForeignKeyViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23503" && pgErr.ConstraintName == constraint
}

// PendingTransfer is an invoice waiting for a bank transfer to arrive.
type PendingTransfer struct {
	InvoiceID    string
	OperatorID   string
	OperatorName string
	Plan         string
	AmountIDR    int64
	IssuedAt     time.Time
	ExpiresAt    *time.Time
}

// ListPendingTransfers is what somebody reconciling a bank statement needs on
// screen: the exact amounts still outstanding, oldest first.
//
// The amount is the whole mechanism — each is made unique per day so an
// incoming mutation identifies its invoice unambiguously — so this is really a
// list of what to look for in the statement.
func (r *SubscriptionRepository) ListPendingTransfers(ctx context.Context) ([]*PendingTransfer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id::text, i.operator_id::text, o.name, i.plan, i.amount_idr,
		       i.created_at, i.expires_at
		FROM subscription_invoices i
		JOIN operators o ON o.id = i.operator_id
		WHERE i.status = 'PENDING' AND i.channel = 'BANK_TRANSFER'
		ORDER BY i.created_at ASC
		LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	transfers := make([]*PendingTransfer, 0)
	for rows.Next() {
		var t PendingTransfer
		if err := rows.Scan(&t.InvoiceID, &t.OperatorID, &t.OperatorName, &t.Plan,
			&t.AmountIDR, &t.IssuedAt, &t.ExpiresAt); err != nil {
			return nil, err
		}
		transfers = append(transfers, &t)
	}
	return transfers, rows.Err()
}

// BankMutation is one credit that arrived in the account.
type BankMutation struct {
	ID           string
	ExternalID   string
	Source       string
	AmountIDR    int64
	Description  string
	OccurredAt   time.Time
	Status       string
	MatchedInvoiceID string
	Note         string
}

// RecordMutation stores an incoming credit and reports whether this call is the
// one that stored it.
//
// ON CONFLICT DO NOTHING rather than a prior read: feeds redeliver and scrapers
// re-read the same page, and two deliveries arriving together would both pass a
// check-then-insert. The unique index is what makes redelivery a no-op, and a
// no-op is the correct response — the caller gets the existing row.
func (r *SubscriptionRepository) RecordMutation(ctx context.Context, m BankMutation) (*BankMutation, bool, error) {
	if m.AmountIDR <= 0 || strings.TrimSpace(m.ExternalID) == "" {
		return nil, false, apperror.ErrValidation
	}
	var stored BankMutation
	err := r.pool.QueryRow(ctx, `
		INSERT INTO bank_mutations (external_id, source, amount_idr, description, occurred_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (source, external_id) DO NOTHING
		RETURNING id::text, external_id, source, amount_idr, description, occurred_at, status,
		          COALESCE(matched_invoice_id::text, ''), note`,
		strings.TrimSpace(m.ExternalID), m.Source, m.AmountIDR, m.Description, m.OccurredAt).
		Scan(&stored.ID, &stored.ExternalID, &stored.Source, &stored.AmountIDR,
			&stored.Description, &stored.OccurredAt, &stored.Status,
			&stored.MatchedInvoiceID, &stored.Note)
	if err == nil {
		return &stored, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	existing, err := r.getMutationByExternalID(ctx, m.Source, strings.TrimSpace(m.ExternalID))
	return existing, false, err
}

func (r *SubscriptionRepository) getMutationByExternalID(ctx context.Context, source, externalID string) (*BankMutation, error) {
	var m BankMutation
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, external_id, source, amount_idr, description, occurred_at, status,
		       COALESCE(matched_invoice_id::text, ''), note
		FROM bank_mutations WHERE source = $1 AND external_id = $2`, source, externalID).
		Scan(&m.ID, &m.ExternalID, &m.Source, &m.AmountIDR, &m.Description,
			&m.OccurredAt, &m.Status, &m.MatchedInvoiceID, &m.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	return &m, err
}

// SettleInvoiceWithMutation ties a credit to an invoice and pays it, in one
// transaction.
//
// One transaction because the two halves are the same fact: the money arrived
// and it bought something. Split apart, a failure between them leaves either a
// paid invoice no credit accounts for, or a credit marked spent on an invoice
// still waiting.
//
// Both updates are conditional. Whoever moves the rows does the settlement;
// a second attempt changes nothing and says so, which is what makes an
// automatic matcher and a person clicking safe to run at the same time.
func (r *SubscriptionRepository) SettleInvoiceWithMutation(ctx context.Context, mutationID, invoiceID, userID, note string) (bool, error) {
	mutation, err := pgUUID(mutationID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	invoice, err := pgUUID(invoiceID)
	if err != nil {
		return false, apperror.ErrValidation
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The mutation moves first. It carries the unique index on
	// matched_invoice_id, so this is the statement that makes two settlements
	// of the same credit impossible.
	tag, err := tx.Exec(ctx, `
		UPDATE bank_mutations
		SET status = 'MATCHED', matched_invoice_id = $2, matched_at = NOW(),
		    matched_by_user_id = $3, note = $4, updated_at = NOW()
		WHERE id = $1 AND status = 'UNMATCHED'`, mutation, invoice, userID, note)
	if err != nil {
		return false, databaseError(err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	var operatorID, plan string
	err = tx.QueryRow(ctx, `
		UPDATE subscription_invoices SET status = 'PAID', paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'PENDING'
		RETURNING operator_id::text, plan::text`, invoice).Scan(&operatorID, &plan)
	if errors.Is(err, pgx.ErrNoRows) {
		// The invoice was settled by something else between the two statements.
		// Rolling back leaves the credit unmatched, which is the honest state:
		// the money is still unaccounted for even though that invoice is paid.
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := extendAccess(ctx, tx, operatorID, plan); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// ListMutations returns credits for the reconciliation screen.
func (r *SubscriptionRepository) ListMutations(ctx context.Context, unmatchedOnly bool) ([]*BankMutation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, external_id, source, amount_idr, description, occurred_at, status,
		       COALESCE(matched_invoice_id::text, ''), note
		FROM bank_mutations
		WHERE ($1::bool = false OR status = 'UNMATCHED')
		ORDER BY occurred_at DESC
		LIMIT 200`, unmatchedOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	mutations := make([]*BankMutation, 0)
	for rows.Next() {
		var m BankMutation
		if err := rows.Scan(&m.ID, &m.ExternalID, &m.Source, &m.AmountIDR, &m.Description,
			&m.OccurredAt, &m.Status, &m.MatchedInvoiceID, &m.Note); err != nil {
			return nil, err
		}
		mutations = append(mutations, &m)
	}
	return mutations, rows.Err()
}

// IgnoreMutation records that a credit is not a subscription payment.
//
// Not a delete: the money still arrived, and the record of it arriving has to
// outlive the decision that it was something else.
func (r *SubscriptionRepository) IgnoreMutation(ctx context.Context, mutationID, userID, note string) error {
	id, err := pgUUID(mutationID)
	if err != nil {
		return apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE bank_mutations
		SET status = 'IGNORED', matched_by_user_id = $2, note = $3, updated_at = NOW()
		WHERE id = $1 AND status = 'UNMATCHED'`, id, userID, note)
	if err != nil {
		return databaseError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrFailedPrecondition
	}
	return nil
}

// RenewalDue is a subscription approaching its expiry with nothing billed for
// the next period.
type RenewalDue struct {
	OperatorID string
	Plan       string
}

// ListDueForRenewal finds subscriptions that need an invoice.
//
// Access already runs out on its own — the sweep marks those PAST_DUE — but
// nothing ever billed for the next period, so a subscription simply stopped
// and nobody was asked to pay. Revenue ending quietly is worse than a customer
// declining to renew, because nobody notices either way.
//
// Excludes operators that already have a pending invoice. Only one may be
// outstanding at a time: two would put two unique amounts in play for the same
// operator, and a transfer against the older one would arrive looking
// unmatched.
//
// Cancelled subscriptions are left alone. Somebody who has cancelled should not
// receive a bill.
func (r *SubscriptionRepository) ListDueForRenewal(ctx context.Context, within time.Duration) ([]RenewalDue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.operator_id::text, s.plan::text
		FROM subscriptions s
		WHERE s.status IN ('ACTIVE', 'PAST_DUE')
		  AND s.cancelled_at IS NULL
		  AND s.access_until < NOW() + make_interval(secs => $1::int)
		  AND NOT EXISTS (
		    SELECT 1 FROM subscription_invoices i
		    WHERE i.operator_id = s.operator_id AND i.status = 'PENDING'
		  )
		ORDER BY s.access_until ASC
		LIMIT 200`, int32(within.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	due := make([]RenewalDue, 0)
	for rows.Next() {
		var item RenewalDue
		if err := rows.Scan(&item.OperatorID, &item.Plan); err != nil {
			return nil, err
		}
		due = append(due, item)
	}
	return due, rows.Err()
}

// CountStaleUnmatched reports credits that have sat unaccounted for too long.
//
// Money arrived and nothing claimed it. That usually means a travel paid an
// amount nobody is expecting — a stale invoice, a typo, a fee deducted in
// transit — and every day it sits is a day that travel believes they have paid
// and the system disagrees.
func (r *SubscriptionRepository) CountStaleUnmatched(ctx context.Context, olderThan time.Duration) (int, int64, error) {
	var count int
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(amount_idr), 0)
		FROM bank_mutations
		WHERE status = 'UNMATCHED' AND occurred_at < NOW() - make_interval(secs => $1::int)`,
		int32(olderThan.Seconds())).Scan(&count, &total)
	return count, total, err
}

// PendingInvoiceSubject reads who an invoice belongs to and for how much.
//
// Read before settling, never after: once the invoice is PAID it is no longer
// pending, and a lookup that ran afterwards would find nothing — which is a
// mistake already made once in this file's callers.
func (r *SubscriptionRepository) PendingInvoiceSubject(ctx context.Context, invoiceID string) (operatorID string, amountIDR int64, err error) {
	id, err := pgUUID(invoiceID)
	if err != nil {
		return "", 0, apperror.ErrValidation
	}
	err = r.pool.QueryRow(ctx, `
		SELECT operator_id::text, amount_idr FROM subscription_invoices
		WHERE id = $1 AND status = 'PENDING'`, id).Scan(&operatorID, &amountIDR)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, apperror.ErrNotFound
	}
	return operatorID, amountIDR, err
}
