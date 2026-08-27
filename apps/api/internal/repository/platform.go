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

// PlatformAccess is what the platform gate needs to know about a caller.
type PlatformAccess struct {
	// Granted is a row in platform_admins.
	Granted bool
	// TwoFactorEnabled comes from Better Auth. It matters because the plugin
	// only issues a session once the second factor is verified, so an enrolled
	// admin's session is one that passed TOTP. An admin who never enrolled has
	// a session backed by a password alone.
	TwoFactorEnabled bool
}

// PlatformAccessFor reports whether a Better Auth user holds platform access,
// and whether their account is protected by a second factor.
//
// Deliberately a plain lookup with no caching. This is the widest privilege in
// the system, and a revocation that takes effect on the next request is worth
// far more than the microseconds a cache would save.
func (r *PlatformRepository) PlatformAccessFor(ctx context.Context, userID string) (PlatformAccess, error) {
	if userID == "" {
		return PlatformAccess{}, nil
	}
	var access PlatformAccess
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM platform_admins WHERE user_id = $1),
		       COALESCE((SELECT u."twoFactorEnabled" FROM "user" u WHERE u.id = $1), false)`,
		userID).Scan(&access.Granted, &access.TwoFactorEnabled)
	return access, err
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

// ListOperators returns tenants, most urgent first, up to limit.
func (r *PlatformRepository) ListOperators(ctx context.Context, limit int32) ([]*PlatformOperator, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
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
		-- Held transactions first: money has arrived and is waiting on somebody,
		-- which is the only thing on this screen that is actually urgent.
		-- Newest next, so a tenant that just signed up is easy to find.
		ORDER BY COALESCE(h.count, 0) DESC, o.created_at DESC
		-- Bounded. Without a limit this page rendered every operator in the
		-- database on one screen — several hundred of them in a local database
		-- full of test rows, and unusable for a platform with real tenants.
		-- Raise it alongside a search box, not on its own.
		LIMIT $1`, limit)
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

// PlatformTransaction is one transaction as the platform sees it, with both
// halves of its life: whether it was paid, and whether what was paid for
// arrived.
type PlatformTransaction struct {
	OrderID           string
	ReceiptNumber     string
	OperatorName      string
	PilgrimName       string
	ProductName       string
	Category          string
	AmountIDR         int64
	PaidAmountIDR     *int64
	NetPaidIDR        int64
	Status            string
	HeldReason        string
	FulfilmentStatus  string
	SupplierName      string
	SupplierReference string
	FulfilmentError   string
	CreatedAt         time.Time
	PaidAt            *time.Time
}

// ListTransactions returns transactions across every tenant, newest first.
//
// needsAttention narrows to the ones that are actually costing somebody: a
// payment that did not match its bill, or something paid for that never
// arrived. Everything else is history, and history is not what a platform
// operator opens this screen to find.
func (r *PlatformRepository) ListTransactions(ctx context.Context, needsAttention bool, limit int32) ([]*PlatformTransaction, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT o.id::text, o.receipt_number, op.name, p.full_name, pr.name, pr.category,
		       o.total_price_idr, o.paid_amount_idr, COALESCE(pay.net_paid_idr, 0),
		       o.status, o.held_reason,
		       COALESCE(f.status, ''), COALESCE(s.name, ''),
		       COALESCE(f.supplier_reference, ''), COALESCE(f.last_error, ''),
		       o.created_at, o.paid_at
		FROM orders o
		JOIN operators op ON op.id = o.operator_id
		JOIN pilgrims p ON p.id = o.pilgrim_id
		JOIN products pr ON pr.id = o.product_id
		LEFT JOIN order_payments pay ON pay.order_id = o.id
		LEFT JOIN order_fulfilments f ON f.order_id = o.id
		LEFT JOIN suppliers s ON s.id = f.supplier_id
		WHERE NOT $1::bool
		   OR o.status = 'HELD'
		   OR f.status IN ('NEEDS_REVIEW', 'FAILED')
		ORDER BY o.created_at DESC
		LIMIT $2`, needsAttention, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	transactions := make([]*PlatformTransaction, 0)
	for rows.Next() {
		var item PlatformTransaction
		if err := rows.Scan(&item.OrderID, &item.ReceiptNumber, &item.OperatorName, &item.PilgrimName,
			&item.ProductName, &item.Category, &item.AmountIDR, &item.PaidAmountIDR, &item.NetPaidIDR,
			&item.Status, &item.HeldReason, &item.FulfilmentStatus, &item.SupplierName,
			&item.SupplierReference, &item.FulfilmentError, &item.CreatedAt, &item.PaidAt); err != nil {
			return nil, err
		}
		transactions = append(transactions, &item)
	}
	return transactions, rows.Err()
}

// PlatformAccount is one person as the platform sees them, across whatever
// roles they hold.
type PlatformAccount struct {
	UserID           string
	Name             string
	Email            string
	EmailVerified    bool
	TwoFactorEnabled bool
	IsPlatformAdmin  bool
	// OperatorName and OrgRole are empty for an account that belongs to no
	// organisation — a jamaah signing in to see their own schedule.
	OperatorName   string
	OrgRole        string
	ActiveSessions int32
	CreatedAt      time.Time
}

// ListAccounts returns people, most recently created first.
//
// search is matched against name and email, and is required beyond a small
// listing: a platform panel that will happily page through every account in
// every tenant is a data export waiting for a curious employee.
func (r *PlatformRepository) ListAccounts(ctx context.Context, search string, limit int32) ([]*PlatformAccount, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.name, u.email, u."emailVerified",
		       COALESCE(u."twoFactorEnabled", false),
		       EXISTS(SELECT 1 FROM platform_admins pa WHERE pa.user_id = u.id),
		       COALESCE(o.name, ''), COALESCE(m.role, ''),
		       COALESCE(s.count, 0)::int, u."createdAt"
		FROM "user" u
		LEFT JOIN member m ON m."userId" = u.id
		LEFT JOIN operators o ON o.better_auth_org_id = m."organizationId"
		LEFT JOIN (
			SELECT "userId", COUNT(*) AS count FROM session
			WHERE "expiresAt" > NOW() GROUP BY "userId"
		) s ON s."userId" = u.id
		WHERE $1 = '' OR u.name ILIKE '%' || $1 || '%' OR u.email ILIKE '%' || $1 || '%'
		ORDER BY u."createdAt" DESC
		LIMIT $2`, search, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make([]*PlatformAccount, 0)
	for rows.Next() {
		var account PlatformAccount
		if err := rows.Scan(&account.UserID, &account.Name, &account.Email, &account.EmailVerified,
			&account.TwoFactorEnabled, &account.IsPlatformAdmin, &account.OperatorName,
			&account.OrgRole, &account.ActiveSessions, &account.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, &account)
	}
	return accounts, rows.Err()
}

// GrantPlatformAdmin adds platform access.
//
// Idempotent, so granting twice is not an error — the caller asked for the
// access to exist, and it does.
func (r *PlatformRepository) GrantPlatformAdmin(ctx context.Context, userID, note, grantedBy string) error {
	if userID == "" {
		return apperror.ErrValidation
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "user" WHERE id = $1)`, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return apperror.ErrNotFound
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO platform_admins (user_id, note, granted_by) VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET note = EXCLUDED.note`, userID, note, grantedBy)
	return err
}

// RevokePlatformAdmin removes platform access, and reports how many admins are
// left so the caller can refuse to remove the last one.
func (r *PlatformRepository) RevokePlatformAdmin(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, apperror.ErrValidation
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM platform_admins WHERE user_id = $1`, userID); err != nil {
		return 0, err
	}
	var remaining int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM platform_admins`).Scan(&remaining)
	return remaining, err
}

// CountPlatformAdmins is how many hold platform access.
func (r *PlatformRepository) CountPlatformAdmins(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM platform_admins`).Scan(&count)
	return count, err
}

// RevokeSessions signs an account out everywhere, and reports how many sessions
// it ended.
//
// The response to a suspected account takeover: the password can be reset
// afterwards, but nothing stops whoever holds a live session until the session
// itself is gone.
func (r *PlatformRepository) RevokeSessions(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM session WHERE "userId" = $1`, userID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
