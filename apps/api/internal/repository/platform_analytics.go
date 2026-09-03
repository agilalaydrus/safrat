package repository

import (
	"context"
	"time"
)

// PlatformAnalytics is how the business is actually doing.
type PlatformAnalytics struct {
	MRRIDR            int64
	PayingTenants     int32
	TrialingTenants   int32
	SuspendedTenants  int32
	LapsedTenants     int32
	NewMRRIDR         int64
	ExpansionMRRIDR   int64
	ContractionMRRIDR int64
	ChurnedMRRIDR     int64
	// Subscriptions that began a trial in the window, and how many of those
	// went on to pay. Measured on the trial's own cohort rather than on
	// conversions in the window: dividing this month's conversions by this
	// month's signups compares two different groups of people.
	TrialsStarted    int32
	TrialsConverted  int32
	ChurnedTenants   int32
	ByPlan           []PlanRevenue
	Days             int32
	MRRAtWindowStart int64
}

type PlanRevenue struct {
	Plan          string
	Tenants       int32
	MonthlyIDR    int64
	MRRIDR        int64
	TrialTenants  int32
	LapsedTenants int32
}

// payingPredicate is the one definition of "this tenant is paying us".
//
// Deliberately identical to the access check in GetAccessByOrgID: a tenant that
// cannot log in is not revenue, and an MRR that counts people who are locked
// out is a number that flatters us at exactly the moment it should not.
const payingPredicate = `
	s.status::text = 'ACTIVE'
	AND s.cancelled_at IS NULL
	AND s.suspended_at IS NULL
	AND subscription_effective_access_until(s.access_until, s.grace_period_days) > NOW()`

// Analytics computes the figures for the platform's own dashboard.
//
// Everything here is derived from live rows rather than from a monthly
// snapshot table, which has one consequence worth stating on the screen: the
// movement figures describe subscriptions as they are now, so a tenant who
// upgraded and then cancelled inside the same window appears in churn and not
// in expansion. Snapshots would fix that and are not worth their weight yet.
func (r *PlatformRepository) Analytics(ctx context.Context, days int32) (PlatformAnalytics, error) {
	result := PlatformAnalytics{Days: days}
	if days <= 0 || days > 365 {
		days = 30
		result.Days = days
	}
	since := time.Now().AddDate(0, 0, -int(days))

	// Current standing. Trials contribute nothing to MRR — they are not paying
	// yet, and counting them is how a plan sounds healthier than it is.
	if err := r.pool.QueryRow(ctx, `
		SELECT
		  COALESCE(SUM(p.monthly_idr) FILTER (WHERE `+payingPredicate+`), 0)::bigint,
		  COUNT(*) FILTER (WHERE `+payingPredicate+`)::int,
		  COUNT(*) FILTER (WHERE s.status::text = 'TRIALING' AND s.cancelled_at IS NULL)::int,
		  COUNT(*) FILTER (WHERE s.suspended_at IS NOT NULL)::int,
		  COUNT(*) FILTER (WHERE s.cancelled_at IS NULL AND s.suspended_at IS NULL
		    AND s.status::text <> 'TRIALING'
		    AND subscription_effective_access_until(s.access_until, s.grace_period_days) <= NOW())::int
		FROM subscriptions s
		JOIN plan_prices p ON p.plan = s.plan`).
		Scan(&result.MRRIDR, &result.PayingTenants, &result.TrialingTenants,
			&result.SuspendedTenants, &result.LapsedTenants); err != nil {
		return result, databaseError(err)
	}

	// New: subscriptions that started inside the window and are paying now.
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(p.monthly_idr), 0)::bigint
		FROM subscriptions s
		JOIN plan_prices p ON p.plan = s.plan
		WHERE s.created_at >= $1 AND `+payingPredicate, since).Scan(&result.NewMRRIDR); err != nil {
		return result, databaseError(err)
	}

	// Expansion and contraction come from the plan-change ledger rather than
	// from comparing today's plan with a remembered one — the ledger is the
	// only place that knows a tenant moved twice.
	if err := r.pool.QueryRow(ctx, `
		SELECT
		  COALESCE(SUM(GREATEST(to_price.monthly_idr - from_price.monthly_idr, 0)), 0)::bigint,
		  COALESCE(SUM(GREATEST(from_price.monthly_idr - to_price.monthly_idr, 0)), 0)::bigint
		FROM subscription_adjustments a
		JOIN plan_prices from_price ON from_price.plan = a.from_plan
		JOIN plan_prices to_price ON to_price.plan = a.to_plan
		WHERE a.effective_at >= $1 AND a.from_plan IS DISTINCT FROM a.to_plan`, since).
		Scan(&result.ExpansionMRRIDR, &result.ContractionMRRIDR); err != nil {
		return result, databaseError(err)
	}

	// Churn: cancelled inside the window, valued at what they were paying.
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(p.monthly_idr), 0)::bigint, COUNT(*)::int
		FROM subscriptions s
		JOIN plan_prices p ON p.plan = s.plan
		WHERE s.cancelled_at >= $1`, since).
		Scan(&result.ChurnedMRRIDR, &result.ChurnedTenants); err != nil {
		return result, databaseError(err)
	}

	// Trial conversion, measured on the cohort that started in the window.
	// A trial still running is neither converted nor lost, so it is counted in
	// the denominator and the screen says the figure only settles later.
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int,
		       COUNT(*) FILTER (WHERE s.status::text = 'ACTIVE' AND s.cancelled_at IS NULL)::int
		FROM subscriptions s
		WHERE s.created_at >= $1`, since).
		Scan(&result.TrialsStarted, &result.TrialsConverted); err != nil {
		return result, databaseError(err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT p.plan::text, p.monthly_idr,
		       COUNT(*) FILTER (WHERE `+payingPredicate+`)::int,
		       COALESCE(SUM(p.monthly_idr) FILTER (WHERE `+payingPredicate+`), 0)::bigint,
		       COUNT(*) FILTER (WHERE s.status::text = 'TRIALING' AND s.cancelled_at IS NULL)::int,
		       COUNT(*) FILTER (WHERE s.cancelled_at IS NULL AND s.status::text <> 'TRIALING'
		         AND subscription_effective_access_until(s.access_until, s.grace_period_days) <= NOW())::int
		FROM plan_prices p
		LEFT JOIN subscriptions s ON s.plan = p.plan
		GROUP BY p.plan, p.monthly_idr
		ORDER BY p.monthly_idr`)
	if err != nil {
		return result, databaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var row PlanRevenue
		if err := rows.Scan(&row.Plan, &row.MonthlyIDR, &row.Tenants, &row.MRRIDR,
			&row.TrialTenants, &row.LapsedTenants); err != nil {
			return result, err
		}
		result.ByPlan = append(result.ByPlan, row)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	// The starting point NRR is measured against, reconstructed rather than
	// remembered: what is here now, minus what arrived, plus what left.
	result.MRRAtWindowStart = result.MRRIDR - result.NewMRRIDR -
		result.ExpansionMRRIDR + result.ContractionMRRIDR + result.ChurnedMRRIDR
	return result, nil
}
