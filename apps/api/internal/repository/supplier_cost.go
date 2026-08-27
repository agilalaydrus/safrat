package repository

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SupplierCostRepository records what suppliers actually charge, and keeps the
// figure a product is validated against up to date.
type SupplierCostRepository struct {
	pool *pgxpool.Pool
}

func NewSupplierCostRepository(pool *pgxpool.Pool) *SupplierCostRepository {
	return &SupplierCostRepository{pool: pool}
}

// SetManualCost records a cost somebody entered by hand.
//
// It refuses to overwrite an observed cost. What a supplier actually charged
// outranks what somebody typed, and letting a stale manual figure quietly
// replace it would defeat the point of observing costs at all.
// SetManualCost records what a product costs to supply.
//
// An empty operatorID means the product belongs to the platform, which is the
// normal case for anything with a supplier: digital products are
// platform-owned and carry no operator at all. Requiring one here rejected
// every product that actually has a cost to record.
func (r *SupplierCostRepository) SetManualCost(ctx context.Context, operatorID, productID string, costIDR int64) error {
	opUUID := pgtype.UUID{}
	if strings.TrimSpace(operatorID) != "" {
		parsed, err := pgUUID(operatorID)
		if err != nil {
			return apperror.ErrValidation
		}
		opUUID = parsed
	}
	productUUID, err := pgUUID(productID)
	if err != nil {
		return apperror.ErrValidation
	}
	if costIDR < 0 {
		return apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE products
		SET supplier_cost_idr = $3, supplier_cost_source = 'MANUAL', supplier_cost_updated_at = NOW()
		-- Scoped by product, plus "this operator's, or the platform's". Supplier
		-- cost belongs to the product, and digital products are platform-owned:
		-- an operator_id predicate alone never matches NULL, so the cost of
		-- every product that actually has a supplier would silently fail to
		-- store.
		-- IS NOT DISTINCT FROM rather than =, so an empty operator matches a
		-- platform product's NULL. A plain equality never matches NULL, which
		-- is how this silently refused every digital product.
		WHERE id = $1 AND (operator_id IS NOT DISTINCT FROM $2 OR operator_id IS NULL) AND supplier_cost_source <> 'OBSERVED'`,
		productUUID, opUUID, costIDR)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Either the product is not this operator's, or it already carries an
		// observed cost. Both mean the caller must not proceed as if the value
		// were stored.
		return apperror.ErrFailedPrecondition
	}
	return nil
}

// RecordObservation stores what a supplier charged for one fulfilment and
// promotes it to the product's current cost.
//
// Both writes happen in one transaction: a product whose cost disagreed with
// its own latest observation would be worse than one with no cost at all,
// because it would look authoritative.
//
// Re-recording the same order is a no-op, so a retried fulfilment reports the
// same purchase rather than inventing a second one.
func (r *SupplierCostRepository) RecordObservation(ctx context.Context, operatorID, productID, orderID string, costIDR int64, supplierReference string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	productUUID, err := pgUUID(productID)
	if err != nil {
		return apperror.ErrValidation
	}
	if costIDR < 0 {
		return apperror.ErrValidation
	}
	var orderUUID any
	if orderID != "" {
		parsed, err := pgUUID(orderID)
		if err != nil {
			return apperror.ErrValidation
		}
		orderUUID = parsed
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO supplier_cost_observations
			(operator_id, product_id, order_id, cost_idr, supplier_reference)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`,
		opUUID, productUUID, orderUUID, costIDR, supplierReference)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// This fulfilment was already observed. Nothing to promote.
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE products
		SET supplier_cost_idr = $3, supplier_cost_source = 'OBSERVED', supplier_cost_updated_at = NOW()
		-- Same widening as SetManualCost. An observed cost is what the supplier
		-- actually charged, and it is the platform's products that have one.
		WHERE id = $1 AND (operator_id = $2 OR operator_id IS NULL)`, productUUID, opUUID, costIDR); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
