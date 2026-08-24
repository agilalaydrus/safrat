package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StorefrontRepository struct {
	pool *pgxpool.Pool
}

func NewStorefrontRepository(pool *pgxpool.Pool) *StorefrontRepository {
	return &StorefrontRepository{pool: pool}
}

func (r *StorefrontRepository) Get(ctx context.Context, operatorID string) (*domain.StorefrontSnapshot, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	var result domain.StorefrontSnapshot
	var publishedAt *time.Time
	err = r.pool.QueryRow(ctx, `
		SELECT draft, COALESCE(published, '{}'::jsonb), draft_revision,
		       published_revision, published_at
		FROM operator_storefronts WHERE operator_id = $1`, id).
		Scan(&result.Draft, &result.Published, &result.DraftRevision, &result.PublishedRevision, &publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result.PublishedAt = publishedAt
	return &result, nil
}

func (r *StorefrontRepository) GetPublished(ctx context.Context, operatorID string) ([]byte, int64, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, 0, apperror.ErrValidation
	}
	var content []byte
	var revision int64
	err = r.pool.QueryRow(ctx, `
		SELECT published, published_revision
		FROM operator_storefronts
		WHERE operator_id = $1 AND published IS NOT NULL`, id).Scan(&content, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, apperror.ErrNotFound
	}
	return content, revision, err
}

func (r *StorefrontRepository) SaveDraft(ctx context.Context, operatorID string, content []byte, expectedRevision int64) (*domain.StorefrontSnapshot, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if expectedRevision == 0 {
		command, insertErr := r.pool.Exec(ctx, `
			INSERT INTO operator_storefronts (operator_id, draft, draft_revision)
			VALUES ($1, $2::jsonb, 1)
			ON CONFLICT (operator_id) DO NOTHING`, id, content)
		if insertErr != nil {
			return nil, insertErr
		}
		if command.RowsAffected() == 0 {
			return nil, apperror.ErrConflict
		}
	} else {
		command, updateErr := r.pool.Exec(ctx, `
			UPDATE operator_storefronts
			SET draft = $2::jsonb, draft_revision = draft_revision + 1, updated_at = NOW()
			WHERE operator_id = $1 AND draft_revision = $3`, id, content, expectedRevision)
		if updateErr != nil {
			return nil, updateErr
		}
		if command.RowsAffected() == 0 {
			return nil, apperror.ErrConflict
		}
	}
	return r.Get(ctx, operatorID)
}

// Publish performs the snapshot copy and legacy-column synchronization in one
// PostgreSQL statement. Readers therefore never observe a half-published CMS
// state, and expectedRevision prevents a stale browser tab from publishing a
// newer draft it has not reviewed.
func (r *StorefrontRepository) Publish(ctx context.Context, operatorID string, expectedRevision int64) (*domain.StorefrontSnapshot, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	command, err := r.pool.Exec(ctx, `
		WITH snapshot AS (
			UPDATE operator_storefronts
			SET published = draft,
			    published_revision = draft_revision,
			    published_at = NOW(),
			    updated_at = NOW()
			WHERE operator_id = $1 AND draft_revision = $2
			RETURNING operator_id, published
		)
		UPDATE operators AS operator SET
			logo_url = COALESCE(snapshot.published ->> 'logoUrl', ''),
			description = COALESCE(snapshot.published ->> 'description', ''),
			whatsapp_number = COALESCE(snapshot.published ->> 'whatsappNumber', ''),
			website = COALESCE(snapshot.published ->> 'website', ''),
			address = COALESCE(snapshot.published ->> 'address', ''),
			city = COALESCE(snapshot.published ->> 'city', ''),
			brand_color = COALESCE(NULLIF(snapshot.published ->> 'brandColor', ''), '#059669'),
			hero_eyebrow = COALESCE(snapshot.published ->> 'heroEyebrow', ''),
			hero_title = COALESCE(snapshot.published ->> 'heroTitle', ''),
			hero_subtitle = COALESCE(snapshot.published ->> 'heroSubtitle', ''),
			hero_image_url = COALESCE(snapshot.published ->> 'heroImageUrl', ''),
			is_profile_complete = TRUE
		FROM snapshot
		WHERE operator.id = snapshot.operator_id`, id, expectedRevision)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() == 0 {
		return nil, apperror.ErrConflict
	}
	return r.Get(ctx, operatorID)
}
