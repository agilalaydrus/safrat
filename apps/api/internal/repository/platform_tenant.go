package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
)

// TenantDetail is one travel agency's whole picture, assembled here rather than
// in the browser: a screen about one tenant should not have to download every
// other tenant's subscriptions and usage to find the row it wants.
type TenantDetail struct {
	Operator PlatformOperator
	Usage    []UsageRow
	Override *PlatformPlanOverride
	Invoices []SubscriptionInvoiceRow
	Counts   TenantCounts
	Team     []TenantTeamMember
	Domains  []TenantDomain
	Audit    []TenantAuditEntry
	Funnel   StorefrontFunnelRow
}

type TenantCounts struct {
	Pilgrims      int32
	Branches      int32
	Seasons       int32
	Products      int32
	Registrations int32
	HeldOrders    int32
	KycPending    int32
	KycVerified   int32
	Staff         int32
}

type TenantTeamMember struct {
	UserID           string
	Name             string
	Email            string
	Role             string
	TwoFactorEnabled bool
	JoinedAt         *time.Time
}

type TenantDomain struct {
	Hostname   string
	IsPrimary  bool
	VerifiedAt *time.Time
}

type TenantAuditEntry struct {
	At         time.Time
	Actor      string
	Action     string
	EntityType string
	EntityID   string
}

// ErrTenantNotFound separates "no such tenant" from a database fault, so the
// handler can answer not_found instead of an internal error.
var ErrTenantNotFound = errors.New("tenant not found")

func (r *PlatformRepository) GetTenantDetail(ctx context.Context, operatorID string) (TenantDetail, error) {
	detail := TenantDetail{}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return detail, ErrTenantNotFound
	}

	// The same shape ListOperators produces, for one tenant. Kept in step with
	// it deliberately: two different definitions of "access until" between the
	// list and the detail would have support quoting one date and the tenant
	// seeing another.
	err = r.pool.QueryRow(ctx, `
		SELECT o.id::text, o.name, COALESCE(o.slug, ''),
		       COALESCE(s.plan::text, ''), COALESCE(s.status::text, ''), s.access_until,
		       CASE WHEN s.operator_id IS NULL THEN NULL ELSE subscription_effective_access_until(s.access_until, s.grace_period_days) END,
		       -- D7: only set once access has actually lapsed — see the same
		       -- guard and comment in PlatformRepository.ListOperators. 90 days
		       -- here must match TenantDeletionGraceDays in platform_deletion.go.
		       CASE WHEN s.operator_id IS NULL
		              OR subscription_effective_access_until(s.access_until, s.grace_period_days) > NOW()
		            THEN NULL
		            ELSE subscription_effective_access_until(s.access_until, s.grace_period_days) + INTERVAL '90 days'
		       END,
		       COALESCE(s.grace_period_days, platform_grace_period_days())::int,
		       s.grace_period_days,
		       COALESCE(s.credit_balance_idr,0),
		       COALESCE(p.count, 0)::int, COALESCE(pr.count, 0)::int, COALESCE(h.count, 0)::int,
		       o.created_at, s.suspended_at,
		       COALESCE((
		         SELECT d.stage FROM dunning_log d
		         WHERE d.operator_id = o.id
		           AND d.lapsed_at = subscription_effective_access_until(s.access_until, s.grace_period_days)
		         ORDER BY d.sent_at DESC LIMIT 1
		       ), ''),
		       COALESCE((
		         SELECT i.amount_idr FROM subscription_invoices i
		         WHERE i.operator_id = o.id AND i.status = 'PENDING'
		         ORDER BY i.created_at DESC LIMIT 1
		       ), 0)
		FROM operators o
		LEFT JOIN subscriptions s ON s.operator_id = o.id
		LEFT JOIN (SELECT operator_id, COUNT(*) AS count FROM pilgrims WHERE is_substituted = false GROUP BY operator_id) p ON p.operator_id = o.id
		LEFT JOIN (SELECT operator_id, COUNT(*) AS count FROM products GROUP BY operator_id) pr ON pr.operator_id = o.id
		LEFT JOIN (SELECT operator_id, COUNT(*) AS count FROM orders WHERE status = 'HELD' GROUP BY operator_id) h ON h.operator_id = o.id
		WHERE o.id = $1`, operator).Scan(
		&detail.Operator.ID, &detail.Operator.Name, &detail.Operator.Slug, &detail.Operator.Plan,
		&detail.Operator.SubscriptionStatus, &detail.Operator.AccessUntil, &detail.Operator.EffectiveAccessUntil,
		&detail.Operator.DeletionEligibleAt,
		&detail.Operator.GracePeriodDays, &detail.Operator.GraceOverrideDays, &detail.Operator.CreditBalanceIDR,
		&detail.Operator.PilgrimCount, &detail.Operator.ProductCount, &detail.Operator.HeldOrderCount,
		&detail.Operator.CreatedAt, &detail.Operator.SuspendedAt, &detail.Operator.DunningStage,
		&detail.Operator.OutstandingIDR)
	if errors.Is(err, pgx.ErrNoRows) {
		return detail, ErrTenantNotFound
	}
	if err != nil {
		return detail, err
	}

	// One pass for the counts. Each is a separate scan of a small table, and
	// running them as one row keeps the page a fixed number of round trips
	// however many blocks it grows.
	if err := r.pool.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM pilgrims WHERE operator_id = $1 AND is_substituted = false)::int,
		  (SELECT COUNT(*) FROM branches WHERE operator_id = $1 AND is_active = true)::int,
		  (SELECT COUNT(*) FROM seasons WHERE operator_id = $1)::int,
		  (SELECT COUNT(*) FROM products WHERE operator_id = $1)::int,
		  (SELECT COUNT(*) FROM pilgrim_registrations WHERE operator_id = $1)::int,
		  (SELECT COUNT(*) FROM orders WHERE operator_id = $1 AND status = 'HELD')::int,
		  (SELECT COUNT(*) FROM kyc_records WHERE operator_id = $1 AND status = 'PENDING')::int,
		  (SELECT COUNT(*) FROM kyc_records WHERE operator_id = $1 AND status = 'VERIFIED')::int,
		  (SELECT COUNT(*) FROM member m JOIN operators o ON o.better_auth_org_id = m."organizationId"
		     WHERE o.id = $1)::int`, operator).Scan(
		&detail.Counts.Pilgrims, &detail.Counts.Branches, &detail.Counts.Seasons,
		&detail.Counts.Products, &detail.Counts.Registrations, &detail.Counts.HeldOrders,
		&detail.Counts.KycPending, &detail.Counts.KycVerified, &detail.Counts.Staff); err != nil {
		return detail, err
	}

	// Deliberately the same shape as ListUsage, scoped to one tenant: the plan
	// comes from operators (not subscriptions) and the limit from plan_limits
	// with any live override on top. Two different definitions of "the limit"
	// between the list and the detail would have support quoting one number
	// while enforcement uses another.
	//
	// DISTINCT ON keeps the newest period only. Without it every past period's
	// snapshot renders as another row of today's usage.
	usage, err := r.pool.Query(ctx, `
		WITH latest AS (
		  SELECT DISTINCT ON (metric) metric, value, computed_at, period_start
		  FROM usage_counters
		  WHERE operator_id = $1
		  ORDER BY metric, period_start DESC
		)
		SELECT l.metric, l.value, l.computed_at, o.plan::text,
		       CASE l.metric
		         WHEN 'pilgrims' THEN COALESCE(po.max_pilgrims, pl.max_pilgrims)
		         WHEN 'branches' THEN COALESCE(po.max_branches, pl.max_branches)
		         ELSE NULL
		       END
		FROM latest l
		JOIN operators o ON o.id = $1
		JOIN plan_limits pl ON pl.plan = o.plan
		LEFT JOIN plan_overrides po ON po.operator_id = o.id
		  AND (po.expires_at IS NULL OR po.expires_at > NOW())
		ORDER BY l.metric`, operator)
	if err != nil {
		return detail, err
	}
	defer usage.Close()
	for usage.Next() {
		row := UsageRow{OperatorID: detail.Operator.ID, OperatorName: detail.Operator.Name}
		var limit *int32
		if err := usage.Scan(&row.Metric, &row.Value, &row.ComputedAt, &row.Plan, &limit); err != nil {
			return detail, err
		}
		if limit != nil {
			// Zero is a real limit — STARTER allows no branches at all — so it
			// must survive as zero rather than collapse into "unlimited".
			value := int64(*limit)
			row.Limit = &value
		}
		detail.Usage = append(detail.Usage, row)
	}
	if err := usage.Err(); err != nil {
		return detail, err
	}

	// Most tenants have no override, and that is not a fault. GetPlanOverride
	// maps a missing row through databaseError, so the sentinel to check is
	// apperror.ErrNotFound rather than pgx.ErrNoRows — matching on the wrong
	// one turns "this tenant has the plan's own limits" into a 404 for the
	// whole page.
	override, err := r.GetPlanOverride(ctx, detail.Operator.ID)
	switch {
	case err == nil:
		detail.Override = &override
	case errors.Is(err, apperror.ErrNotFound), errors.Is(err, pgx.ErrNoRows):
	default:
		return detail, err
	}

	invoices, err := r.pool.Query(ctx, `
		SELECT id::text, plan::text, status::text, channel::text, amount_idr,
		       due_at, paid_at, voided_at, COALESCE(voided_reason, ''), created_at
		FROM subscription_invoices
		WHERE operator_id = $1
		ORDER BY created_at DESC
		LIMIT 24`, operator)
	if err != nil {
		return detail, err
	}
	defer invoices.Close()
	for invoices.Next() {
		row := SubscriptionInvoiceRow{OperatorID: detail.Operator.ID, OperatorName: detail.Operator.Name}
		if err := invoices.Scan(&row.ID, &row.Plan, &row.Status, &row.Channel, &row.AmountIDR,
			&row.DueAt, &row.PaidAt, &row.VoidedAt, &row.VoidedReason, &row.CreatedAt); err != nil {
			return detail, err
		}
		detail.Invoices = append(detail.Invoices, row)
	}
	if err := invoices.Err(); err != nil {
		return detail, err
	}

	// Better Auth's own tables, joined the same raw-SQL way the auth
	// interceptor does. "user" is quoted because it is a reserved word.
	team, err := r.pool.Query(ctx, `
		SELECT u.id, COALESCE(u.name, ''), COALESCE(u.email, ''), m.role,
		       COALESCE(u."twoFactorEnabled", false), m."createdAt"
		FROM member m
		JOIN "user" u ON u.id = m."userId"
		JOIN operators o ON o.better_auth_org_id = m."organizationId"
		WHERE o.id = $1
		ORDER BY m."createdAt"`, operator)
	if err != nil {
		return detail, err
	}
	defer team.Close()
	for team.Next() {
		var row TenantTeamMember
		if err := team.Scan(&row.UserID, &row.Name, &row.Email, &row.Role, &row.TwoFactorEnabled, &row.JoinedAt); err != nil {
			return detail, err
		}
		detail.Team = append(detail.Team, row)
	}
	if err := team.Err(); err != nil {
		return detail, err
	}

	domains, err := r.pool.Query(ctx, `
		SELECT hostname, is_primary, verified_at
		FROM operator_domains WHERE operator_id = $1
		ORDER BY is_primary DESC, hostname`, operator)
	if err != nil {
		return detail, err
	}
	defer domains.Close()
	for domains.Next() {
		var row TenantDomain
		if err := domains.Scan(&row.Hostname, &row.IsPrimary, &row.VerifiedAt); err != nil {
			return detail, err
		}
		detail.Domains = append(detail.Domains, row)
	}
	if err := domains.Err(); err != nil {
		return detail, err
	}

	// The audit trail for this tenant only. An entry whose actor account has
	// since been deleted still has to name somebody, so the id stands in when
	// the join finds nothing.
	audit, err := r.pool.Query(ctx, `
		SELECT a.created_at, COALESCE(NULLIF(u.email, ''), a.user_id, 'sistem'),
		       a.action, COALESCE(a.entity_type, ''), COALESCE(a.entity_id, '')
		FROM audit_logs a
		LEFT JOIN "user" u ON u.id = a.user_id
		WHERE a.operator_id = $1
		ORDER BY a.created_at DESC
		LIMIT 40`, operator)
	if err != nil {
		return detail, err
	}
	defer audit.Close()
	for audit.Next() {
		var row TenantAuditEntry
		if err := audit.Scan(&row.At, &row.Actor, &row.Action, &row.EntityType, &row.EntityID); err != nil {
			return detail, err
		}
		detail.Audit = append(detail.Audit, row)
	}
	if err := audit.Err(); err != nil {
		return detail, err
	}

	detail.Funnel = StorefrontFunnelRow{
		OperatorID: detail.Operator.ID, OperatorName: detail.Operator.Name, Slug: detail.Operator.Slug,
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE((
		         SELECT SUM(visitors)::int FROM funnel_daily
		         WHERE operator_id = $1 AND step = 'LANDING' AND day >= (NOW() - INTERVAL '30 days')::date
		       ), 0),
		       COALESCE((
		         SELECT COUNT(*)::int FROM pilgrim_registrations
		         WHERE operator_id = $1 AND created_at >= NOW() - INTERVAL '30 days'
		       ), 0)`, operator).Scan(&detail.Funnel.Visitors, &detail.Funnel.Registrations); err != nil {
		return detail, err
	}
	if detail.Funnel.Visitors > 0 {
		detail.Funnel.Conversion = float64(detail.Funnel.Registrations) / float64(detail.Funnel.Visitors)
	}
	return detail, nil
}
