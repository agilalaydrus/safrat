package repository

import (
	"context"
	"strings"

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
