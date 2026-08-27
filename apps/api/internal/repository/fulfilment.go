package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FulfilmentRepository tracks whether a supplier actually delivered what a
// jamaah paid for.
//
// Deliberately separate from order status: "did they pay" and "did it arrive"
// are different questions, and one column carrying both makes "paid but
// undelivered" inexpressible — which is exactly the state that needs to be
// visible.
type FulfilmentRepository struct {
	pool *pgxpool.Pool
}

func NewFulfilmentRepository(pool *pgxpool.Pool) *FulfilmentRepository {
	return &FulfilmentRepository{pool: pool}
}

// Fulfilment is one order's delivery state.
type Fulfilment struct {
	ID                string
	OrderID           string
	OperatorID        string
	SupplierID        string
	SupplierName      string
	ProductName       string
	PilgrimName       string
	Status            string
	SupplierReference string
	Attempts          int32
	LastError         string
	ResolutionNote    string
	SentAt            *time.Time
	DeliveredAt       *time.Time
	CreatedAt         time.Time
}

// Open records that an order now owes a delivery, and reports whether this call
// is the one that created it.
//
// ON CONFLICT DO NOTHING rather than checking first: two workers picking up the
// same paid order both pass a check-then-act, and the result is a jamaah's
// pulsa sent twice at our expense.
func (r *FulfilmentRepository) Open(ctx context.Context, orderID, operatorID, supplierID string) (bool, error) {
	order, err := pgUUID(orderID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	var supplier any
	if supplierID != "" {
		parsed, err := pgUUID(supplierID)
		if err != nil {
			return false, apperror.ErrValidation
		}
		supplier = parsed
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO order_fulfilments (order_id, operator_id, supplier_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (order_id) DO NOTHING`, order, operator, supplier)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Claim moves a fulfilment from PENDING to SENT and counts the attempt,
// returning false if somebody else already claimed it.
//
// The transition is the lock. A worker that reads PENDING and then writes SENT
// as two statements can be overtaken between them; a conditional UPDATE cannot.
func (r *FulfilmentRepository) Claim(ctx context.Context, orderID string) (bool, error) {
	id, err := pgUUID(orderID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET status = 'SENT', attempts = attempts + 1, sent_at = NOW(), updated_at = NOW()
		WHERE order_id = $1 AND status = 'PENDING'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Settle applies a supplier's answer.
//
// Only a fulfilment that is still out moves, so a redelivered callback settles
// nothing a second time, and a supplier answering after a human already
// resolved the case cannot overwrite that decision.
func (r *FulfilmentRepository) Settle(ctx context.Context, orderID, status, reference, failure string) (bool, error) {
	id, err := pgUUID(orderID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET status = $2,
		    supplier_reference = CASE WHEN $3 <> '' THEN $3 ELSE supplier_reference END,
		    last_error = $4,
		    delivered_at = CASE WHEN $2 = 'DELIVERED' THEN NOW() ELSE delivered_at END,
		    updated_at = NOW()
		WHERE order_id = $1 AND status IN ('PENDING', 'SENT')`, id, status, reference, failure)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Resolve is a human deciding what a fulfilment really was. Recorded distinctly
// from a supplier's own answer, so the two are never confused later.
//
// DELIVERED is included among the states this can change, which is worth being
// explicit about. It is the only way to correct a delivery that was recorded
// but never happened — and without it, refusing to refund delivered goods would
// point at a door that does not open, trapping an operator with a jamaah's
// money and no lawful way to return it.
//
// The trade is that somebody could flip a real delivery to failed in order to
// refund it. That is why who decided and why are both stored: the act is
// permitted, but never anonymous.
func (r *FulfilmentRepository) Resolve(ctx context.Context, orderID, status, userID, note string) error {
	id, err := pgUUID(orderID)
	if err != nil {
		return apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET status = $2, resolved_by_user_id = $3, resolution_note = $4,
		    delivered_at = CASE WHEN $2 = 'DELIVERED' THEN NOW() ELSE delivered_at END,
		    updated_at = NOW()
		WHERE order_id = $1 AND status IN ('NEEDS_REVIEW', 'SENT', 'DELIVERED', 'FAILED')`, id, status, userID, note)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrFailedPrecondition
	}
	return nil
}

// ByCallbackToken finds the supplier a callback belongs to. Tokens are unique
// across suppliers, or one supplier's token would settle another's
// transactions.
func (r *FulfilmentRepository) SupplierByCallbackToken(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", apperror.ErrUnauthorized
	}
	var id string
	err := r.pool.QueryRow(ctx,
		`SELECT id::text FROM suppliers WHERE callback_token = $1 AND status = 'ACTIVE'`, token).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.ErrUnauthorized
	}
	return id, err
}

// FindOrderByReference locates the order a callback is about, either by our own
// order id or by the supplier's reference from the original request.
func (r *FulfilmentRepository) FindOrderByReference(ctx context.Context, supplierID, reference string) (string, error) {
	supplier, err := pgUUID(supplierID)
	if err != nil {
		return "", apperror.ErrValidation
	}
	var orderID string
	err = r.pool.QueryRow(ctx, `
		SELECT f.order_id::text FROM order_fulfilments f
		WHERE f.supplier_id = $1 AND (f.order_id::text = $2 OR f.supplier_reference = $2)
		LIMIT 1`, supplier, reference).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.ErrNotFound
	}
	return orderID, err
}

// ListNeedingAttention returns fulfilments a human has to look at: a supplier
// said something unreadable, or never answered at all.
func (r *FulfilmentRepository) ListNeedingAttention(ctx context.Context, stuckAfter time.Duration, limit int32) ([]*Fulfilment, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT f.id::text, f.order_id::text, f.operator_id::text,
		       COALESCE(f.supplier_id::text, ''), COALESCE(s.name, ''),
		       p.name, pil.full_name, f.status, f.supplier_reference, f.attempts,
		       f.last_error, f.resolution_note, f.sent_at, f.delivered_at, f.created_at
		FROM order_fulfilments f
		JOIN orders o ON o.id = f.order_id
		JOIN products p ON p.id = o.product_id
		JOIN pilgrims pil ON pil.id = o.pilgrim_id
		LEFT JOIN suppliers s ON s.id = f.supplier_id
		WHERE f.status = 'NEEDS_REVIEW'
		   OR (f.status = 'SENT' AND f.sent_at < NOW() - make_interval(secs => $1::int))
		ORDER BY f.created_at ASC
		LIMIT $2`, int32(stuckAfter.Seconds()), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fulfilments := make([]*Fulfilment, 0)
	for rows.Next() {
		var item Fulfilment
		if err := rows.Scan(&item.ID, &item.OrderID, &item.OperatorID, &item.SupplierID, &item.SupplierName,
			&item.ProductName, &item.PilgrimName, &item.Status, &item.SupplierReference, &item.Attempts,
			&item.LastError, &item.ResolutionNote, &item.SentAt, &item.DeliveredAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		fulfilments = append(fulfilments, &item)
	}
	return fulfilments, rows.Err()
}

// CountNeedingAttention is the same question without the rows, for a sweep that
// only needs to know whether to raise an alarm.
func (r *FulfilmentRepository) CountNeedingAttention(ctx context.Context, stuckAfter time.Duration) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM order_fulfilments
		WHERE status = 'NEEDS_REVIEW'
		   OR (status = 'SENT' AND sent_at < NOW() - make_interval(secs => $1::int))`,
		int32(stuckAfter.Seconds())).Scan(&count)
	return count, err
}

// StatusFor reports an order's delivery state, or empty when nothing is owed.
func (r *FulfilmentRepository) StatusFor(ctx context.Context, orderID string) (string, error) {
	id, err := pgUUID(orderID)
	if err != nil {
		return "", apperror.ErrValidation
	}
	var status string
	err = r.pool.QueryRow(ctx, `SELECT status FROM order_fulfilments WHERE order_id = $1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return status, err
}

// OpenMissing creates fulfilment rows for paid orders that owe a delivery and
// have none, and reports how many it had to add.
//
// This closes a gap between two writes that are not in one transaction: an
// order is marked paid, and then a fulfilment is opened. A process that dies
// between them leaves a paid order owing a delivery that nothing records — and
// because every dispatch path starts from the fulfilment row, nothing would
// ever notice. The jamaah has paid and no part of the system believes anything
// is owed.
//
// Set-based and keyed by the unique constraint, so it is safe to run
// repeatedly and cannot race the normal path into a second row.
func (r *FulfilmentRepository) OpenMissing(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO order_fulfilments (order_id, operator_id, supplier_id, status, last_error)
		SELECT o.id, o.operator_id, pr.supplier_id,
		       CASE WHEN pr.supplier_id IS NULL THEN 'NEEDS_REVIEW' ELSE 'PENDING' END,
		       CASE WHEN pr.supplier_id IS NULL
		            THEN 'Produk belum punya routing supplier aktif'
		            ELSE '' END
		FROM orders o
		JOIN products p ON p.id = o.product_id
		LEFT JOIN product_routes pr ON pr.product_id = p.id AND pr.is_active
		WHERE o.status = 'PAID'
		  AND p.category <> 'TRAVEL_PACKAGE'
		  AND NOT EXISTS (SELECT 1 FROM order_fulfilments f WHERE f.order_id = o.id)
		ON CONFLICT (order_id) DO NOTHING`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
