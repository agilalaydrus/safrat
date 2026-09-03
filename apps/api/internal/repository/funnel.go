package repository

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FunnelRepository struct {
	pool *pgxpool.Pool
}

func NewFunnelRepository(pool *pgxpool.Pool) *FunnelRepository {
	return &FunnelRepository{pool: pool}
}

// FunnelEvent is one step a visitor reached.
type FunnelEvent struct {
	// Empty means TawafiqHub's own site.
	OperatorID   string
	VisitorHash  string
	Step         string
	Path         string
	ArticleSlug  string
	ReferrerHost string
	UTMSource    string
	UTMCampaign  string
	City         string
	Province     string
	EntityID     string
}

// Record writes one event.
//
// Deliberately without a transaction and without a return value beyond the
// error: a funnel row is not worth slowing a page for, and the caller is
// expected to ignore failures. Analytics that can take a storefront down is
// worse than no analytics.
func (r *FunnelRepository) Record(ctx context.Context, event FunnelEvent) error {
	operator, err := nullableUUID(strings.TrimSpace(event.OperatorID))
	if err != nil {
		return err
	}
	entity, err := nullableUUID(strings.TrimSpace(event.EntityID))
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO funnel_events
		  (operator_id, visitor_hash, step, path, article_slug, referrer_host,
		   utm_source, utm_campaign, city, province, entity_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		operator, event.VisitorHash, event.Step, trim(event.Path, 300), trim(event.ArticleSlug, 180),
		trim(event.ReferrerHost, 180), trim(event.UTMSource, 80), trim(event.UTMCampaign, 120),
		trim(event.City, 120), trim(event.Province, 120), entity)
	return databaseError(err)
}

// OperatorIDBySlug resolves a storefront slug, returning "" when it matches no
// operator. Unknown slugs are dropped rather than stored, so a caller inventing
// a tenant cannot put rows anywhere.
func (r *FunnelRepository) OperatorIDBySlug(ctx context.Context, slug string) (string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return "", nil
	}
	var id string
	err := r.pool.QueryRow(ctx, `SELECT id::text FROM operators WHERE slug = $1`, slug).Scan(&id)
	if err != nil {
		// Not found is not an error here: the caller records nothing and the
		// page carries on.
		return "", nil
	}
	return id, nil
}

func trim(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		return value[:max]
	}
	return value
}

// OperatorExists confirms an id belongs to a real operator, so a caller holding
// an id gets the same treatment as one holding a slug: an invented value is
// dropped rather than counted as the platform's own traffic.
func (r *FunnelRepository) OperatorExists(ctx context.Context, operatorID string) bool {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM operators WHERE id = $1::uuid)`, operatorID).Scan(&exists); err != nil {
		return false
	}
	return exists
}

// dailyBotEventCap bounds how many events one visitor token may contribute to a
// day's summary.
//
// A person browsing a storefront produces a handful: a landing, maybe an
// article, a form. Sixty is far above that and far below what a crawler that
// hides its user agent produces, so this catches the machines the agent filter
// misses without discarding anybody real. Set high on purpose — over-filtering
// is the worse mistake here, because it silently removes conversions that did
// happen.
const dailyBotEventCap = 60

// RollUpDay recomputes one day's summary from the raw events.
//
// Days are Asia/Jakarta, matching the rest of the system. A UTC day would put
// everything between 00:00 and 07:00 WIB — which for a storefront is the tail
// of the previous evening's browsing — onto the wrong date, and the hourly
// "when are people active" figure would be wrong by seven hours.
//
// Safe to run twice: the unique key makes the second pass overwrite. It has to
// be, because a daily job that duplicates instead of replacing eventually
// reports double and nobody notices which day it started.
func (r *FunnelRepository) RollUpDay(ctx context.Context, day time.Time) (int64, error) {
	command, err := r.pool.Exec(ctx, `
		WITH bounded AS (
		  SELECT operator_id, visitor_hash, step, utm_source
		  FROM funnel_events
		  WHERE (occurred_at AT TIME ZONE 'Asia/Jakarta')::date = $1::date
		    AND visitor_hash NOT IN (
		      -- Tokens that behaved like a machine on this day, dropped whole
		      -- rather than trimmed: half a crawler's session is not a person.
		      SELECT visitor_hash FROM funnel_events
		      WHERE (occurred_at AT TIME ZONE 'Asia/Jakarta')::date = $1::date
		      GROUP BY visitor_hash
		      HAVING COUNT(*) > $2
		    )
		)
		INSERT INTO funnel_daily (operator_id, day, step, utm_source, visitors, events, computed_at)
		SELECT operator_id, $1::date, step, utm_source,
		       COUNT(DISTINCT visitor_hash)::int, COUNT(*)::int, NOW()
		FROM bounded
		GROUP BY operator_id, step, utm_source
		ON CONFLICT (operator_id, day, step, utm_source)
		DO UPDATE SET visitors = EXCLUDED.visitors, events = EXCLUDED.events,
		              computed_at = EXCLUDED.computed_at`,
		day, dailyBotEventCap)
	if err != nil {
		return 0, databaseError(err)
	}
	return command.RowsAffected(), nil
}

// PurgeRawEvents deletes raw rows past their retention.
//
// The summaries stay forever — they are aggregate, cannot be turned back into
// individuals, and are the part still useful a year later. The raw rows are
// both a storage cost and a liability, and keeping them past their usefulness
// buys nothing.
func (r *FunnelRepository) PurgeRawEvents(ctx context.Context, keepDays int) (int64, error) {
	if keepDays < 30 {
		// A floor, in the same spirit as the audit retention: a mistaken zero
		// must not erase the data the summaries are rebuilt from.
		keepDays = 30
	}
	command, err := r.pool.Exec(ctx,
		`DELETE FROM funnel_events WHERE occurred_at < NOW() - make_interval(days => $1::int)`, keepDays)
	if err != nil {
		return 0, databaseError(err)
	}
	return command.RowsAffected(), nil
}
