package repository

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/supplier"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SupplierRepository manages the platform's supplier catalogue: who supplies
// what, how their answers are read, and what they actually said.
//
// Nothing here takes an operator id. Suppliers belong to the platform, not to a
// travel — authorisation is the platform_admins gate above it.
type SupplierRepository struct {
	pool *pgxpool.Pool
}

func NewSupplierRepository(pool *pgxpool.Pool) *SupplierRepository {
	return &SupplierRepository{pool: pool}
}

// Supplier is one provider the platform buys from.
type Supplier struct {
	ID               string
	Name             string
	Code             string
	BaseURL          string
	CredentialEnvVar string
	Status           string
	Notes            string
	RouteCount       int32
	RuleCount        int32
	CreatedAt        time.Time
}

func (r *SupplierRepository) ListSuppliers(ctx context.Context) ([]*Supplier, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id::text, s.name, s.code, s.base_url, s.credential_env_var, s.status, s.notes,
		       COALESCE(routes.count, 0)::int, COALESCE(rules.count, 0)::int, s.created_at
		FROM suppliers s
		LEFT JOIN (SELECT supplier_id, COUNT(*) AS count FROM product_routes GROUP BY supplier_id) routes
		  ON routes.supplier_id = s.id
		LEFT JOIN (SELECT supplier_id, COUNT(*) AS count FROM supplier_response_rules WHERE is_active GROUP BY supplier_id) rules
		  ON rules.supplier_id = s.id
		ORDER BY s.status = 'ACTIVE' DESC, s.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	suppliers := make([]*Supplier, 0)
	for rows.Next() {
		var item Supplier
		if err := rows.Scan(&item.ID, &item.Name, &item.Code, &item.BaseURL, &item.CredentialEnvVar,
			&item.Status, &item.Notes, &item.RouteCount, &item.RuleCount, &item.CreatedAt); err != nil {
			return nil, err
		}
		suppliers = append(suppliers, &item)
	}
	return suppliers, rows.Err()
}

// SaveSupplier creates or updates by `code`, which is the stable identity —
// renaming a supplier for display must not create a second one.
func (r *SupplierRepository) SaveSupplier(ctx context.Context, item Supplier) (*Supplier, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO suppliers (name, code, base_url, credential_env_var, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (code) DO UPDATE SET
			name = EXCLUDED.name, base_url = EXCLUDED.base_url,
			credential_env_var = EXCLUDED.credential_env_var,
			status = EXCLUDED.status, notes = EXCLUDED.notes, updated_at = NOW()
		RETURNING id::text`,
		item.Name, item.Code, item.BaseURL, item.CredentialEnvVar, item.Status, item.Notes).Scan(&id)
	if err != nil {
		return nil, err
	}
	item.ID = id
	return &item, nil
}

// ProductRoute is which supplier fulfils a product.
type ProductRoute struct {
	ID           string
	ProductID    string
	ProductName  string
	OperatorName string
	Category     string
	SupplierID   string
	SupplierName string
	SupplierSKU  string
	IsActive     bool
}

// ListRoutes returns routed products. Digital categories only: a travel package
// is fulfilled by the operator, not bought from anybody.
func (r *SupplierRepository) ListRoutes(ctx context.Context) ([]*ProductRoute, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pr.id::text, p.id::text, p.name, COALESCE(o.name, 'TawafiqHub'), p.category,
		       s.id::text, s.name, pr.supplier_sku, pr.is_active
		FROM product_routes pr
		JOIN products p ON p.id = pr.product_id
		LEFT JOIN operators o ON o.id = p.operator_id
		JOIN suppliers s ON s.id = pr.supplier_id
		ORDER BY pr.is_active DESC, p.name ASC
		LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	routes := make([]*ProductRoute, 0)
	for rows.Next() {
		var route ProductRoute
		if err := rows.Scan(&route.ID, &route.ProductID, &route.ProductName, &route.OperatorName,
			&route.Category, &route.SupplierID, &route.SupplierName, &route.SupplierSKU, &route.IsActive); err != nil {
			return nil, err
		}
		routes = append(routes, &route)
	}
	return routes, rows.Err()
}

// SaveRoute points a product at a supplier. One route per product, so this
// replaces rather than adds — two routes would make "which supplier did this
// sale go to" unanswerable after the fact.
func (r *SupplierRepository) SaveRoute(ctx context.Context, productID, supplierID, sku string, active bool) error {
	product, err := pgUUID(productID)
	if err != nil {
		return apperror.ErrValidation
	}
	supplierUUID, err := pgUUID(supplierID)
	if err != nil {
		return apperror.ErrValidation
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO product_routes (product_id, supplier_id, supplier_sku, is_active)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (product_id) DO UPDATE SET
			supplier_id = EXCLUDED.supplier_id, supplier_sku = EXCLUDED.supplier_sku,
			is_active = EXCLUDED.is_active, updated_at = NOW()`,
		product, supplierUUID, sku, active)
	return err
}

// ActiveRulesFor returns a supplier's rules already ordered the way they must
// be applied — priority, then age, so ordering is total and never depends on
// the order rows happened to come back in.
func (r *SupplierRepository) ActiveRulesFor(ctx context.Context, supplierID string) ([]supplier.Rule, error) {
	id, err := pgUUID(supplierID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, priority, pattern, outcome, reference_group, cost_group
		FROM supplier_response_rules
		WHERE supplier_id = $1 AND is_active
		ORDER BY priority ASC, created_at ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]supplier.Rule, 0)
	for rows.Next() {
		var rule supplier.Rule
		var outcome string
		if err := rows.Scan(&rule.ID, &rule.Priority, &rule.Pattern, &outcome,
			&rule.ReferenceGroup, &rule.CostGroup); err != nil {
			return nil, err
		}
		rule.Outcome = supplier.Outcome(outcome)
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// StoredRule is a rule as the admin panel shows it.
type StoredRule struct {
	supplier.Rule
	SupplierID  string
	Description string
	IsActive    bool
	CreatedAt   time.Time
}

func (r *SupplierRepository) ListRules(ctx context.Context, supplierID string) ([]*StoredRule, error) {
	id, err := pgUUID(supplierID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, supplier_id::text, priority, pattern, outcome,
		       reference_group, cost_group, description, is_active, created_at
		FROM supplier_response_rules
		WHERE supplier_id = $1
		ORDER BY priority ASC, created_at ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]*StoredRule, 0)
	for rows.Next() {
		var stored StoredRule
		var outcome string
		if err := rows.Scan(&stored.ID, &stored.SupplierID, &stored.Priority, &stored.Pattern, &outcome,
			&stored.ReferenceGroup, &stored.CostGroup, &stored.Description, &stored.IsActive, &stored.CreatedAt); err != nil {
			return nil, err
		}
		stored.Outcome = supplier.Outcome(outcome)
		rules = append(rules, &stored)
	}
	return rules, rows.Err()
}

// CreateRule stores a pattern. The caller is expected to have compiled it
// first — a rule that cannot compile should be refused in the panel, not
// discovered over live transactions.
func (r *SupplierRepository) CreateRule(ctx context.Context, stored StoredRule) (string, error) {
	supplierUUID, err := pgUUID(stored.SupplierID)
	if err != nil {
		return "", apperror.ErrValidation
	}
	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO supplier_response_rules
			(supplier_id, priority, pattern, outcome, reference_group, cost_group, description, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id::text`,
		supplierUUID, stored.Priority, stored.Pattern, string(stored.Outcome),
		stored.ReferenceGroup, stored.CostGroup, stored.Description, stored.IsActive).Scan(&id)
	return id, err
}

// SetRuleActive is the only mutation a rule allows. Editing a pattern in place
// would silently change how past logs should have been read; a new rule plus
// deactivating the old one keeps that history legible.
func (r *SupplierRepository) SetRuleActive(ctx context.Context, ruleID string, active bool) error {
	id, err := pgUUID(ruleID)
	if err != nil {
		return apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `UPDATE supplier_response_rules SET is_active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// LogEntry is one exchange with a supplier.
type LogEntry struct {
	ID                string
	SupplierName      string
	OrderID           string
	Direction         string
	Endpoint          string
	RequestBody       string
	ResponseBody      string
	HTTPStatus        *int32
	Outcome           string
	SupplierReference string
	CostIDR           *int64
	Error             string
	CreatedAt         time.Time
}

// ListLogs returns recent exchanges, newest first. unmatchedOnly narrows it to
// responses no rule recognised — the queue that keeps the rules honest, since
// every entry is a supplier saying something nobody taught the system to read.
func (r *SupplierRepository) ListLogs(ctx context.Context, unmatchedOnly bool, limit int32) ([]*LogEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT l.id::text, s.name, COALESCE(l.order_id::text, ''), l.direction, l.endpoint,
		       l.request_body, l.response_body, l.http_status, l.outcome,
		       l.supplier_reference, l.cost_idr, l.error, l.created_at
		FROM supplier_request_logs l
		JOIN suppliers s ON s.id = l.supplier_id
		WHERE NOT $1::bool OR l.outcome = 'UNMATCHED'
		ORDER BY l.created_at DESC
		LIMIT $2`, unmatchedOnly, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := make([]*LogEntry, 0)
	for rows.Next() {
		var entry LogEntry
		if err := rows.Scan(&entry.ID, &entry.SupplierName, &entry.OrderID, &entry.Direction, &entry.Endpoint,
			&entry.RequestBody, &entry.ResponseBody, &entry.HTTPStatus, &entry.Outcome,
			&entry.SupplierReference, &entry.CostIDR, &entry.Error, &entry.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &entry)
	}
	return logs, rows.Err()
}

// RecordExchange appends a log entry. Never updates: a log is evidence, and the
// database refuses to let one be rewritten.
func (r *SupplierRepository) RecordExchange(ctx context.Context, entry LogEntry, supplierID, orderID, ruleID string) error {
	supplierUUID, err := pgUUID(supplierID)
	if err != nil {
		return apperror.ErrValidation
	}
	var order, rule any
	if orderID != "" {
		parsed, err := pgUUID(orderID)
		if err != nil {
			return apperror.ErrValidation
		}
		order = parsed
	}
	if ruleID != "" {
		parsed, err := pgUUID(ruleID)
		if err != nil {
			return apperror.ErrValidation
		}
		rule = parsed
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO supplier_request_logs
			(supplier_id, order_id, direction, endpoint, request_body, response_body,
			 http_status, outcome, matched_rule_id, supplier_reference, cost_idr, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		supplierUUID, order, entry.Direction, entry.Endpoint, entry.RequestBody, entry.ResponseBody,
		entry.HTTPStatus, entry.Outcome, rule, entry.SupplierReference, entry.CostIDR, entry.Error)
	return err
}

// RouteForOrder finds the supplier that should fulfil an order's product.
func (r *SupplierRepository) RouteForOrder(ctx context.Context, orderID string) (*ProductRoute, error) {
	id, err := pgUUID(orderID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	var route ProductRoute
	err = r.pool.QueryRow(ctx, `
		SELECT pr.id::text, p.id::text, p.name, COALESCE(o.name, 'TawafiqHub'), p.category,
		       s.id::text, s.name, pr.supplier_sku, pr.is_active
		FROM orders ord
		JOIN products p ON p.id = ord.product_id
		LEFT JOIN operators o ON o.id = p.operator_id
		JOIN product_routes pr ON pr.product_id = p.id
		JOIN suppliers s ON s.id = pr.supplier_id
		WHERE ord.id = $1 AND pr.is_active AND s.status = 'ACTIVE'`, id).
		Scan(&route.ID, &route.ProductID, &route.ProductName, &route.OperatorName, &route.Category,
			&route.SupplierID, &route.SupplierName, &route.SupplierSKU, &route.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	return &route, err
}

// EndpointFor resolves everything needed to call a supplier, reading the
// credentials from the environment variables the row names.
//
// Credentials are looked up here rather than stored, so a database dump carries
// none and rotating a key touches no data. A supplier configured with an env
// var that is not set comes back with empty credentials — the request will fail
// at the supplier, which is the right place for it to fail and be logged,
// rather than silently sending an unsigned request that looks legitimate.
func (r *SupplierRepository) EndpointFor(ctx context.Context, supplierID string) (supplier.Endpoint, error) {
	id, err := pgUUID(supplierID)
	if err != nil {
		return supplier.Endpoint{}, apperror.ErrValidation
	}
	var (
		endpoint                      supplier.Endpoint
		protocol, method              string
		timeoutSeconds                int32
		usernameEnvVar, credentialEnv string
	)
	err = r.pool.QueryRow(ctx, `
		SELECT protocol, base_url, http_method, request_template, rpc_method,
		       signature_recipe, timeout_seconds, username_env_var, credential_env_var
		FROM suppliers WHERE id = $1 AND status = 'ACTIVE'`, id).
		Scan(&protocol, &endpoint.BaseURL, &method, &endpoint.Template, &endpoint.RPCMethod,
			&endpoint.SignatureRecipe, &timeoutSeconds, &usernameEnvVar, &credentialEnv)
	if errors.Is(err, pgx.ErrNoRows) {
		return supplier.Endpoint{}, apperror.ErrNotFound
	}
	if err != nil {
		return supplier.Endpoint{}, err
	}
	endpoint.Protocol = supplier.Protocol(protocol)
	endpoint.Method = method
	endpoint.Timeout = time.Duration(timeoutSeconds) * time.Second
	if usernameEnvVar != "" {
		endpoint.Username = os.Getenv(usernameEnvVar)
	}
	if credentialEnv != "" {
		endpoint.Credential = os.Getenv(credentialEnv)
	}
	return endpoint, nil
}

// PendingDispatch is a fulfilment waiting to be sent, with what the supplier
// needs to identify the purchase.
type PendingDispatch struct {
	OrderID     string
	OperatorID  string
	SupplierID  string
	SupplierSKU string
	AmountIDR   int64
	Destination string
}

// ListPendingDispatch returns fulfilments that have never been sent, oldest
// first — somebody has already been waiting longest for those.
func (r *SupplierRepository) ListPendingDispatch(ctx context.Context, limit int32) ([]PendingDispatch, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := r.pool.Query(ctx, `
		SELECT f.order_id::text, f.operator_id::text, f.supplier_id::text,
		       pr.supplier_sku, o.total_price_idr, o.destination
		FROM order_fulfilments f
		JOIN orders o ON o.id = f.order_id
		JOIN product_routes pr ON pr.product_id = o.product_id AND pr.is_active
		JOIN suppliers s ON s.id = f.supplier_id AND s.status = 'ACTIVE'
		WHERE f.status = 'PENDING'
		ORDER BY f.created_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pending := make([]PendingDispatch, 0)
	for rows.Next() {
		var item PendingDispatch
		if err := rows.Scan(&item.OrderID, &item.OperatorID, &item.SupplierID,
			&item.SupplierSKU, &item.AmountIDR, &item.Destination); err != nil {
			return nil, err
		}
		pending = append(pending, item)
	}
	return pending, rows.Err()
}

// PendingDispatchFor resolves one order for immediate sending, for the fast
// path that runs the moment a payment settles rather than at the next sweep.
func (r *SupplierRepository) PendingDispatchFor(ctx context.Context, orderID string) (PendingDispatch, error) {
	id, err := pgUUID(orderID)
	if err != nil {
		return PendingDispatch{}, apperror.ErrValidation
	}
	var item PendingDispatch
	err = r.pool.QueryRow(ctx, `
		SELECT f.order_id::text, f.operator_id::text, f.supplier_id::text,
		       pr.supplier_sku, o.total_price_idr, o.destination
		FROM order_fulfilments f
		JOIN orders o ON o.id = f.order_id
		JOIN product_routes pr ON pr.product_id = o.product_id AND pr.is_active
		JOIN suppliers s ON s.id = f.supplier_id AND s.status = 'ACTIVE'
		WHERE f.order_id = $1 AND f.status = 'PENDING'`, id).
		Scan(&item.OrderID, &item.OperatorID, &item.SupplierID,
			&item.SupplierSKU, &item.AmountIDR, &item.Destination)
	if errors.Is(err, pgx.ErrNoRows) {
		// Already sent, already settled, or no longer routable. Not an error:
		// the fast path and the sweep can both reach the same order, and
		// whichever arrives second should simply find nothing to do.
		return PendingDispatch{}, apperror.ErrNotFound
	}
	return item, err
}
