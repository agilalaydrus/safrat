package repository

import (
	"context"
	"time"
)

// UsageRow is one tenant's standing against its limits.
type UsageRow struct {
	OperatorID   string
	OperatorName string
	Plan         string
	Metric       string
	Value        int64
	// nil means unlimited, which is different from zero. Zero is a real limit
	// — STARTER allows no branches at all — so the two must never be conflated.
	Limit       *int64
	ComputedAt  time.Time
	PeriodStart time.Time
}

// RecomputeUsage takes today's snapshot for every operator.
//
// One statement per metric rather than a loop over tenants: a hundred round
// trips to count a hundred rows is the shape that turns a cheap job into a slow
// one. ON CONFLICT makes a second run today overwrite, so the worker is safe to
// re-run and safe to run twice.
//
// All three run in one transaction over a share-locked list of operators. An
// INSERT..SELECT FROM operators reads its rows and then checks the foreign key
// at write time, so a tenant deleted in between fails the whole job — which is
// how the first version of this broke, and would have broken in production the
// first time somebody removed a tenant while the nightly job was running. FOR
// SHARE holds those rows for the few seconds the job takes; a deletion waits
// rather than tearing the snapshot in half.
func (r *SubscriptionRepository) RecomputeUsage(ctx context.Context) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var total int64
	statements := []string{
		// Substituted pilgrims are excluded, matching what the entitlement
		// trigger counts. A usage figure that disagrees with the limit it is
		// measured against would send people chasing a discrepancy that is not
		// there.
		`WITH ops AS (SELECT id FROM operators ORDER BY id FOR SHARE)
		 INSERT INTO usage_counters (operator_id, metric, period_start, value, computed_at)
		 SELECT o.id, 'pilgrims', CURRENT_DATE,
		        COALESCE((SELECT COUNT(*) FROM pilgrims p WHERE p.operator_id = o.id AND p.is_substituted = false), 0),
		        NOW()
		 FROM ops o
		 ON CONFLICT (operator_id, metric, period_start)
		 DO UPDATE SET value = EXCLUDED.value, computed_at = EXCLUDED.computed_at`,

		`WITH ops AS (SELECT id FROM operators ORDER BY id FOR SHARE)
		 INSERT INTO usage_counters (operator_id, metric, period_start, value, computed_at)
		 SELECT o.id, 'branches', CURRENT_DATE,
		        COALESCE((SELECT COUNT(*) FROM branches b WHERE b.operator_id = o.id AND b.is_active), 0),
		        NOW()
		 FROM ops o
		 ON CONFLICT (operator_id, metric, period_start)
		 DO UPDATE SET value = EXCLUDED.value, computed_at = EXCLUDED.computed_at`,

		// Only confirmed assets. A reservation that was never completed is not
		// storage anybody is using.
		`WITH ops AS (SELECT id FROM operators ORDER BY id FOR SHARE)
		 INSERT INTO usage_counters (operator_id, metric, period_start, value, computed_at)
		 SELECT o.id, 'storage_bytes', CURRENT_DATE,
		        COALESCE((SELECT SUM(a.size_bytes) FROM operator_storefront_assets a
		                  WHERE a.operator_id = o.id AND a.object_key IS NOT NULL), 0),
		        NOW()
		 FROM ops o
		 ON CONFLICT (operator_id, metric, period_start)
		 DO UPDATE SET value = EXCLUDED.value, computed_at = EXCLUDED.computed_at`,
	}
	for _, statement := range statements {
		command, err := tx.Exec(ctx, statement)
		if err != nil {
			return 0, databaseError(err)
		}
		total += command.RowsAffected()
	}
	return total, tx.Commit(ctx)
}

// ListUsage returns the latest snapshot for every tenant, alongside the limit
// each figure is measured against.
//
// Limits are read live rather than snapshotted: a limit raised this morning
// should show against last night's usage immediately, because the question the
// screen answers is "who is near a ceiling now".
func (r *SubscriptionRepository) ListUsage(ctx context.Context) ([]UsageRow, error) {
	rows, err := r.pool.Query(ctx, `
		WITH latest AS (
		  SELECT DISTINCT ON (operator_id, metric)
		         operator_id, metric, value, computed_at, period_start
		  FROM usage_counters
		  ORDER BY operator_id, metric, period_start DESC
		)
		SELECT o.id::text, o.name, o.plan::text, l.metric, l.value,
		       CASE l.metric
		         WHEN 'pilgrims' THEN COALESCE(po.max_pilgrims, pl.max_pilgrims)
		         WHEN 'branches' THEN COALESCE(po.max_branches, pl.max_branches)
		         ELSE NULL
		       END,
		       l.computed_at, l.period_start
		FROM latest l
		JOIN operators o ON o.id = l.operator_id
		JOIN plan_limits pl ON pl.plan = o.plan
		LEFT JOIN plan_overrides po ON po.operator_id = o.id
		  AND (po.expires_at IS NULL OR po.expires_at > NOW())
		ORDER BY o.name ASC, l.metric ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usage := make([]UsageRow, 0)
	for rows.Next() {
		var row UsageRow
		var limit *int32
		if err := rows.Scan(&row.OperatorID, &row.OperatorName, &row.Plan, &row.Metric,
			&row.Value, &limit, &row.ComputedAt, &row.PeriodStart); err != nil {
			return nil, err
		}
		if limit != nil {
			value := int64(*limit)
			row.Limit = &value
		}
		usage = append(usage, row)
	}
	return usage, rows.Err()
}
