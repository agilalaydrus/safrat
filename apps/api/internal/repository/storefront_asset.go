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

var ErrStorefrontStorageQuota = errors.New("storefront storage quota exceeded")

type StorefrontAssetRepository struct {
	pool *pgxpool.Pool
}

func NewStorefrontAssetRepository(pool *pgxpool.Pool) *StorefrontAssetRepository {
	return &StorefrontAssetRepository{pool: pool}
}

func (r *StorefrontAssetRepository) Reserve(ctx context.Context, operatorID, reservationKey, kind string, sizeBytes int64, expiresAt time.Time, quotaBytes int64) error {
	id, err := pgUUID(operatorID)
	if err != nil || reservationKey == "" || kind == "" || sizeBytes <= 0 || quotaBytes <= 0 {
		return apperror.ErrValidation
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, id); err != nil {
		return err
	}
	var usedBytes int64
	if err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0)
		FROM operator_storefront_assets
		WHERE operator_id = $1 AND (state = 'LIVE' OR expires_at > NOW())`, id).Scan(&usedBytes); err != nil {
		return err
	}
	if usedBytes > quotaBytes-sizeBytes {
		return ErrStorefrontStorageQuota
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO operator_storefront_assets (reservation_key, operator_id, kind, size_bytes, expires_at)
		VALUES ($1, $2, $3, $4, $5)`, reservationKey, id, kind, sizeBytes, expiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *StorefrontAssetRepository) Confirm(ctx context.Context, operatorID, reservationKey, objectKey, publicURL string, sizeBytes int64) error {
	id, err := pgUUID(operatorID)
	if err != nil || reservationKey == "" || objectKey == "" || publicURL == "" || sizeBytes <= 0 {
		return apperror.ErrValidation
	}
	command, err := r.pool.Exec(ctx, `
		UPDATE operator_storefront_assets
		SET state = 'LIVE', object_key = $3, public_url = $4, size_bytes = $5,
		    confirmed_at = NOW(), orphaned_at = NULL
		WHERE operator_id = $1 AND reservation_key = $2
		  AND (state = 'PENDING' OR (state = 'LIVE' AND object_key = $3 AND public_url = $4))`, id, reservationKey, objectKey, publicURL, sizeBytes)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *StorefrontAssetRepository) Usage(ctx context.Context, operatorID string) (domain.StorefrontStorageUsage, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return domain.StorefrontStorageUsage{}, apperror.ErrValidation
	}
	var usage domain.StorefrontStorageUsage
	var assetCount, pendingCount int64
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(size_bytes) FILTER (WHERE state = 'LIVE' OR expires_at > NOW()), 0),
		       COUNT(*) FILTER (WHERE state = 'LIVE'),
		       COUNT(*) FILTER (WHERE state = 'PENDING' AND expires_at > NOW())
		FROM operator_storefront_assets WHERE operator_id = $1`, id).
		Scan(&usage.UsedBytes, &assetCount, &pendingCount)
	usage.AssetCount = int32(assetCount)
	usage.PendingCount = int32(pendingCount)
	return usage, err
}

func (r *StorefrontAssetRepository) RefreshOrphans(ctx context.Context) (int64, error) {
	command, err := r.pool.Exec(ctx, `
		UPDATE operator_storefront_assets AS asset
		SET orphaned_at = CASE
		  WHEN EXISTS (
		    SELECT 1 FROM operator_storefronts AS storefront
		    WHERE storefront.operator_id = asset.operator_id
		      AND (POSITION(asset.public_url IN storefront.draft::text) > 0 OR
		           POSITION(asset.public_url IN COALESCE(storefront.published::text, '')) > 0)
		  ) THEN NULL
		  ELSE COALESCE(asset.orphaned_at, NOW())
		END
		WHERE asset.state = 'LIVE'`)
	if err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

func (r *StorefrontAssetRepository) ListOrphans(ctx context.Context, before time.Time, limit int32) ([]domain.StorefrontAsset, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT asset.object_key, asset.operator_id::text
		FROM operator_storefront_assets AS asset
		WHERE asset.state = 'LIVE' AND asset.orphaned_at < $1
		  AND NOT EXISTS (
		    SELECT 1 FROM operator_storefronts AS storefront
		    WHERE storefront.operator_id = asset.operator_id
		      AND (POSITION(asset.public_url IN storefront.draft::text) > 0 OR
		           POSITION(asset.public_url IN COALESCE(storefront.published::text, '')) > 0)
		  )
		ORDER BY asset.orphaned_at, asset.object_key
		LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assets := make([]domain.StorefrontAsset, 0, limit)
	for rows.Next() {
		var asset domain.StorefrontAsset
		if err := rows.Scan(&asset.ObjectKey, &asset.OperatorID); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (r *StorefrontAssetRepository) DeleteRecord(ctx context.Context, operatorID, objectKey string) error {
	id, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	command, err := r.pool.Exec(ctx, `DELETE FROM operator_storefront_assets WHERE operator_id = $1 AND object_key = $2 AND state = 'LIVE'`, id, objectKey)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *StorefrontAssetRepository) ListExpiredReservations(ctx context.Context, before time.Time, limit int32) ([]domain.StorefrontAssetReservation, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT reservation_key, operator_id::text
		FROM operator_storefront_assets
		WHERE state = 'PENDING' AND expires_at <= $1
		ORDER BY expires_at, reservation_key
		LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reservations := make([]domain.StorefrontAssetReservation, 0, limit)
	for rows.Next() {
		var reservation domain.StorefrontAssetReservation
		if err := rows.Scan(&reservation.ReservationKey, &reservation.OperatorID); err != nil {
			return nil, err
		}
		reservations = append(reservations, reservation)
	}
	return reservations, rows.Err()
}

func (r *StorefrontAssetRepository) DeleteReservation(ctx context.Context, operatorID, reservationKey string) error {
	id, err := pgUUID(operatorID)
	if err != nil || reservationKey == "" {
		return apperror.ErrValidation
	}
	command, err := r.pool.Exec(ctx, `DELETE FROM operator_storefront_assets WHERE operator_id = $1 AND reservation_key = $2 AND state = 'PENDING'`, id, reservationKey)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
