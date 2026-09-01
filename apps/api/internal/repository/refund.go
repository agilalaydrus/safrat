package repository

import (
	"context"
	"errors"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RefundRepository records refunds and reads whether an order has already had
// one. Every write method takes the caller's transaction: a refund is
// only correct if the record, the pilgrim's credit, the agent's reversal and
// the order's status all land together.
type RefundRepository struct {
	pool *pgxpool.Pool
}

func NewRefundRepository(pool *pgxpool.Pool) *RefundRepository {
	return &RefundRepository{pool: pool}
}

// LockOrderForRefund reads the order under a row lock, together with what has
// already been refunded against it.
//
// FOR UPDATE, not a plain read: two refund requests arriving at once would
// otherwise both see an unrefunded order and each approve a refund. The lock
// makes the second wait and see that the first already happened — by then the
// order is REFUNDED, which the caller rejects.
func (r *RefundRepository) LockOrderForRefund(ctx context.Context, tx pgx.Tx, operatorID, orderID string) (*domain.RefundableOrder, error) {
	opID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	ordID, err := pgUUID(orderID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, db.New(tx), opID)
	if err != nil {
		return nil, err
	}
	var order domain.RefundableOrder
	var agentID, buyerAgentID *string
	err = tx.QueryRow(ctx, `
		SELECT o.id::text, COALESCE(o.pilgrim_id::text, ''), o.buyer_kind,
		       o.buyer_agent_id::text, o.agent_id::text, o.total_price_idr,
		       o.agent_commission_idr, o.status,
		       COALESCE((SELECT SUM(amount_idr) FROM order_refunds WHERE order_id = o.id), 0)::bigint,
		       COALESCE((SELECT SUM(commission_reversed_idr) FROM order_refunds WHERE order_id = o.id), 0)::bigint
		FROM orders o
		WHERE o.id = $1 AND o.operator_id = $2
		  AND ($3::uuid IS NULL OR o.branch_id = $3)
		FOR UPDATE OF o`, ordID, opID, scope).
		Scan(&order.ID, &order.PilgrimID, &order.BuyerKind, &buyerAgentID, &agentID, &order.TotalPriceIDR,
			&order.AgentCommissionIDR, &order.Status, &order.RefundedIDR, &order.CommissionReversed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if agentID != nil {
		order.AgentID = *agentID
	}
	if buyerAgentID != nil {
		order.BuyerAgentID = *buyerAgentID
	}
	return &order, nil
}

// FindRefundByKeyTx returns the refund already recorded under this idempotency
// key, or nil if there is none.
//
// Looked up before any precondition is applied, because a replay is an advice
// about a refund that already happened — and the state it must not be judged
// against is precisely the state that refund created.
func (r *RefundRepository) FindRefundByKeyTx(ctx context.Context, tx pgx.Tx, orderID, idempotencyKey string) (*domain.OrderRefund, error) {
	ordID, err := pgUUID(orderID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	refund, err := scanRefund(tx.QueryRow(ctx, `
		SELECT id::text, operator_id::text, order_id::text, amount_idr,
		       commission_reversed_idr, reason, COALESCE(created_by_user_id, ''), created_at
		FROM order_refunds WHERE order_id = $1 AND idempotency_key = $2`, ordID, idempotencyKey))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return refund, nil
}

// RefundParams describes one refund to record.
type RefundParams struct {
	OperatorID            string
	OrderID               string
	AmountIDR             int64
	CommissionReversedIDR int64
	Reason                string
	CreatedByUserID       string
	IdempotencyKey        string
}

// CreateRefundTx records a refund. A replay of the same idempotency key
// returns the refund that was already recorded, rather than recording a second
// one or failing — the caller asked for that refund to exist, and it does.
func (r *RefundRepository) CreateRefundTx(ctx context.Context, tx pgx.Tx, params RefundParams) (*domain.OrderRefund, bool, error) {
	opID, err := pgUUID(params.OperatorID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	ordID, err := pgUUID(params.OrderID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	if params.AmountIDR <= 0 {
		return nil, false, apperror.ErrValidation
	}
	// ON CONFLICT DO NOTHING rather than catching the unique violation: inside
	// a transaction a failed statement poisons the whole transaction, so the
	// read that recovers the existing refund would itself fail. Letting
	// Postgres decline the insert quietly keeps the transaction usable.
	refund, err := scanRefund(tx.QueryRow(ctx, `
		INSERT INTO order_refunds
			(operator_id, order_id, amount_idr, commission_reversed_idr, reason, created_by_user_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
		ON CONFLICT (order_id, idempotency_key) WHERE idempotency_key <> '' DO NOTHING
		RETURNING id::text, operator_id::text, order_id::text, amount_idr,
		          commission_reversed_idr, reason, COALESCE(created_by_user_id, ''), created_at`,
		opID, ordID, params.AmountIDR, params.CommissionReversedIDR, params.Reason,
		params.CreatedByUserID, params.IdempotencyKey))
	if err == nil {
		return refund, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	// No row came back, so this key was already used. That earlier refund is
	// what the caller is asking about.
	existing, err := scanRefund(tx.QueryRow(ctx, `
		SELECT id::text, operator_id::text, order_id::text, amount_idr,
		       commission_reversed_idr, reason, COALESCE(created_by_user_id, ''), created_at
		FROM order_refunds WHERE order_id = $1 AND idempotency_key = $2`, ordID, params.IdempotencyKey))
	if err != nil {
		return nil, false, err
	}
	return existing, false, nil
}

// MarkOrderRefundedTx flips the order to REFUNDED. Every refund reaches this,
// because every refund returns the whole transaction.
func (r *RefundRepository) MarkOrderRefundedTx(ctx context.Context, tx pgx.Tx, orderID string) error {
	ordID, err := pgUUID(orderID)
	if err != nil {
		return apperror.ErrValidation
	}
	_, err = tx.Exec(ctx, `UPDATE orders SET status = 'REFUNDED', updated_at = NOW() WHERE id = $1`, ordID)
	return err
}

// ListByOrder returns an order's refund history, newest first.
func (r *RefundRepository) ListByOrder(ctx context.Context, operatorID, orderID string) ([]*domain.OrderRefund, error) {
	opID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	ordID, err := pgUUID(orderID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, db.New(r.pool), opID)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, operator_id::text, order_id::text, amount_idr,
		       commission_reversed_idr, reason, COALESCE(created_by_user_id, ''), created_at
		FROM order_refunds r
		WHERE r.operator_id = $1 AND r.order_id = $2
		  AND EXISTS (
		    SELECT 1 FROM orders o
		    WHERE o.id = r.order_id AND o.operator_id = r.operator_id
		      AND ($3::uuid IS NULL OR o.branch_id = $3)
		  )
		ORDER BY r.created_at DESC`, opID, ordID, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	refunds := make([]*domain.OrderRefund, 0)
	for rows.Next() {
		refund, err := scanRefund(rows)
		if err != nil {
			return nil, err
		}
		refunds = append(refunds, refund)
	}
	return refunds, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRefund(row rowScanner) (*domain.OrderRefund, error) {
	var refund domain.OrderRefund
	if err := row.Scan(&refund.ID, &refund.OperatorID, &refund.OrderID, &refund.AmountIDR,
		&refund.CommissionReversedIDR, &refund.Reason, &refund.CreatedByUserID, &refund.CreatedAt); err != nil {
		return nil, err
	}
	return &refund, nil
}
