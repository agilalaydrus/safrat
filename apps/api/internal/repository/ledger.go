package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LedgerRepository appends to the append-only money ledgers and reads the
// balances they imply.
//
// A balance is always the sum of entries, never a stored figure: a cached
// total can drift out of step with its history, and then neither number can be
// trusted. Reversals are new negative entries — the database refuses to let an
// entry be edited at all.
type LedgerRepository struct {
	pool *pgxpool.Pool
}

func NewLedgerRepository(pool *pgxpool.Pool) *LedgerRepository {
	return &LedgerRepository{pool: pool}
}

// ledgerExecutor is satisfied by both *pgxpool.Pool and pgx.Tx. A refund has
// to record the reversal, the pilgrim's credit and the order's new status
// together or not at all, so those appends must be able to join a caller's
// transaction; a standalone append uses the pool and is its own transaction.
type ledgerExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// CommissionEntry is one movement in an agent's commission balance.
type CommissionEntry struct {
	OperatorID      string
	AgentID         string
	AmountIDR       int64
	Kind            string
	OrderID         string
	Note            string
	CreatedByUserID string
	IdempotencyKey  string
}

// AppendCommission records a commission movement. Re-appending with the same
// idempotency key is an advice about the same entry, not a second one, so a
// retried webhook or a replayed job cannot credit an agent twice. The same
// holds for the one-EARNED-per-order rule; reversals may repeat, because a
// refund may be partial.
//
// Duplicates are declined by ON CONFLICT DO NOTHING rather than raised and
// caught. Catching a unique violation works against the pool but not inside a
// caller's transaction, where a failed statement poisons every statement that
// follows — and this append has to be able to join the refund transaction.
// The table's only unique constraints are those two idempotency guards, so an
// untargeted DO NOTHING can only mean "already recorded".
func (r *LedgerRepository) AppendCommission(ctx context.Context, entry CommissionEntry) error {
	return appendCommission(ctx, r.pool, entry)
}

// AppendCommissionTx is AppendCommission inside a caller's transaction.
func (r *LedgerRepository) AppendCommissionTx(ctx context.Context, tx pgx.Tx, entry CommissionEntry) error {
	return appendCommission(ctx, tx, entry)
}

func appendCommission(ctx context.Context, exec ledgerExecutor, entry CommissionEntry) error {
	operatorID, err := pgUUID(entry.OperatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	agentID, err := pgUUID(entry.AgentID)
	if err != nil {
		return apperror.ErrValidation
	}
	if entry.AmountIDR == 0 || entry.Kind == "" {
		return apperror.ErrValidation
	}
	var orderID any
	if strings.TrimSpace(entry.OrderID) != "" {
		parsed, err := pgUUID(entry.OrderID)
		if err != nil {
			return apperror.ErrValidation
		}
		orderID = parsed
	}
	_, err = exec.Exec(ctx, `
		INSERT INTO agent_commission_entries
			(operator_id, agent_id, amount_idr, kind, order_id, note, created_by_user_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
		ON CONFLICT DO NOTHING`,
		operatorID, agentID, entry.AmountIDR, entry.Kind, orderID, entry.Note, entry.CreatedByUserID, entry.IdempotencyKey)
	return err
}

// CommissionBalance is the total an agent has earned net of reversals. It does
// not subtract payouts — the caller combines the two, as it always has.
func (r *LedgerRepository) CommissionBalance(ctx context.Context, agentID string) (int64, error) {
	id, err := pgUUID(agentID)
	if err != nil {
		return 0, apperror.ErrValidation
	}
	var total int64
	err = r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_idr), 0) FROM agent_commission_entries WHERE agent_id = $1`, id).Scan(&total)
	return total, err
}

// BalanceEntry is one movement in a pilgrim's deposit balance.
type BalanceEntry struct {
	OperatorID      string
	PilgrimID       string
	AmountIDR       int64
	Kind            string
	OrderID         string
	Note            string
	CreatedByUserID string
	IdempotencyKey  string
}

// AppendBalance records a movement in a pilgrim's deposit. Same idempotency
// contract, and the same conflict handling, as AppendCommission: a repeat is
// an advice about the same entry.
func (r *LedgerRepository) AppendBalance(ctx context.Context, entry BalanceEntry) error {
	return appendBalance(ctx, r.pool, entry)
}

// AppendBalanceTx is AppendBalance inside a caller's transaction.
func (r *LedgerRepository) AppendBalanceTx(ctx context.Context, tx pgx.Tx, entry BalanceEntry) error {
	return appendBalance(ctx, tx, entry)
}

func appendBalance(ctx context.Context, exec ledgerExecutor, entry BalanceEntry) error {
	operatorID, err := pgUUID(entry.OperatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	pilgrimID, err := pgUUID(entry.PilgrimID)
	if err != nil {
		return apperror.ErrValidation
	}
	if entry.AmountIDR == 0 || entry.Kind == "" {
		return apperror.ErrValidation
	}
	var orderID any
	if strings.TrimSpace(entry.OrderID) != "" {
		parsed, err := pgUUID(entry.OrderID)
		if err != nil {
			return apperror.ErrValidation
		}
		orderID = parsed
	}
	_, err = exec.Exec(ctx, `
		INSERT INTO pilgrim_balance_entries
			(operator_id, pilgrim_id, amount_idr, kind, order_id, note, created_by_user_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
		ON CONFLICT DO NOTHING`,
		operatorID, pilgrimID, entry.AmountIDR, entry.Kind, orderID, entry.Note, entry.CreatedByUserID, entry.IdempotencyKey)
	return err
}

// PilgrimBalance is what the operator currently holds on a pilgrim's behalf.
func (r *LedgerRepository) PilgrimBalance(ctx context.Context, pilgrimID string) (int64, error) {
	id, err := pgUUID(pilgrimID)
	if err != nil {
		return 0, apperror.ErrValidation
	}
	var total int64
	err = r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_idr), 0) FROM pilgrim_balance_entries WHERE pilgrim_id = $1`, id).Scan(&total)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return total, err
}
