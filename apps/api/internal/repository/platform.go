package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PlatformRepository reads across every tenant, which is the whole point of it
// and the reason nothing here takes an operator id to scope by. Authorisation
// happens above it, in the service, and must never be skipped.
type PlatformRepository struct {
	pool *pgxpool.Pool
}

func NewPlatformRepository(pool *pgxpool.Pool) *PlatformRepository {
	return &PlatformRepository{pool: pool}
}

// IsPlatformAdmin reports whether a Better Auth user holds platform access.
//
// Deliberately a plain lookup with no caching. This is the widest privilege in
// the system, and a revocation that takes effect on the next request is worth
// far more than the microseconds a cache would save.
func (r *PlatformRepository) IsPlatformAdmin(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM platform_admins WHERE user_id = $1)`, userID).Scan(&exists)
	return exists, err
}

// PlatformOperator is one tenant as the platform sees it.
type PlatformOperator struct {
	ID                 string
	Name               string
	Slug               string
	Plan               string
	SubscriptionStatus string
	AccessUntil        *time.Time
	PilgrimCount       int32
	ProductCount       int32
	HeldOrderCount     int32
	CreatedAt          time.Time
}

func (r *PlatformRepository) ListOperators(ctx context.Context) ([]*PlatformOperator, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.id::text, o.name, COALESCE(o.slug, ''),
		       COALESCE(s.plan::text, ''), COALESCE(s.status::text, ''), s.access_until,
		       COALESCE(p.count, 0)::int, COALESCE(pr.count, 0)::int, COALESCE(h.count, 0)::int,
		       o.created_at
		FROM operators o
		LEFT JOIN subscriptions s ON s.operator_id = o.id
		LEFT JOIN (SELECT operator_id, COUNT(*) AS count FROM pilgrims WHERE is_substituted = false GROUP BY operator_id) p ON p.operator_id = o.id
		LEFT JOIN (SELECT operator_id, COUNT(*) AS count FROM products GROUP BY operator_id) pr ON pr.operator_id = o.id
		LEFT JOIN (SELECT operator_id, COUNT(*) AS count FROM orders WHERE status = 'HELD' GROUP BY operator_id) h ON h.operator_id = o.id
		ORDER BY o.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	operators := make([]*PlatformOperator, 0)
	for rows.Next() {
		var operator PlatformOperator
		if err := rows.Scan(&operator.ID, &operator.Name, &operator.Slug, &operator.Plan,
			&operator.SubscriptionStatus, &operator.AccessUntil, &operator.PilgrimCount,
			&operator.ProductCount, &operator.HeldOrderCount, &operator.CreatedAt); err != nil {
			return nil, err
		}
		operators = append(operators, &operator)
	}
	return operators, rows.Err()
}

// PlatformProduct is one product with whatever is known about its cost.
type PlatformProduct struct {
	ID                    string
	OperatorID            string
	OperatorName          string
	SeasonName            string
	Name                  string
	Category              string
	PriceIDR              int64
	SupplierCostIDR       *int64
	SupplierCostSource    string
	SupplierCostUpdatedAt *time.Time
}

// ListProducts returns products across every tenant, newest first. With
// includeCosted false it returns only those with no supplier cost recorded —
// the ones selling with no price floor beneath them.
func (r *PlatformRepository) ListProducts(ctx context.Context, includeCosted bool) ([]*PlatformProduct, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.operator_id::text, o.name, COALESCE(s.name, ''), p.name, p.category,
		       p.price_idr, p.supplier_cost_idr, p.supplier_cost_source, p.supplier_cost_updated_at
		FROM products p
		JOIN operators o ON o.id = p.operator_id
		LEFT JOIN seasons s ON s.id = p.season_id
		WHERE $1::bool OR p.supplier_cost_idr IS NULL
		ORDER BY p.supplier_cost_idr IS NOT NULL, p.created_at DESC
		LIMIT 500`, includeCosted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]*PlatformProduct, 0)
	for rows.Next() {
		var product PlatformProduct
		if err := rows.Scan(&product.ID, &product.OperatorID, &product.OperatorName, &product.SeasonName,
			&product.Name, &product.Category, &product.PriceIDR, &product.SupplierCostIDR,
			&product.SupplierCostSource, &product.SupplierCostUpdatedAt); err != nil {
			return nil, err
		}
		products = append(products, &product)
	}
	return products, rows.Err()
}

// GetProduct reads one product across tenants, for reporting back what a cost
// change produced.
func (r *PlatformRepository) GetProduct(ctx context.Context, productID string) (*PlatformProduct, error) {
	id, err := pgUUID(productID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	var product PlatformProduct
	err = r.pool.QueryRow(ctx, `
		SELECT p.id::text, p.operator_id::text, o.name, COALESCE(s.name, ''), p.name, p.category,
		       p.price_idr, p.supplier_cost_idr, p.supplier_cost_source, p.supplier_cost_updated_at
		FROM products p
		JOIN operators o ON o.id = p.operator_id
		LEFT JOIN seasons s ON s.id = p.season_id
		WHERE p.id = $1`, id).
		Scan(&product.ID, &product.OperatorID, &product.OperatorName, &product.SeasonName,
			&product.Name, &product.Category, &product.PriceIDR, &product.SupplierCostIDR,
			&product.SupplierCostSource, &product.SupplierCostUpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	return &product, err
}
