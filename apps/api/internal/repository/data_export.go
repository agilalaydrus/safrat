package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DataExportRepository struct {
	pool *pgxpool.Pool
}

func NewDataExportRepository(pool *pgxpool.Pool) *DataExportRepository {
	return &DataExportRepository{pool: pool}
}

type DataExportRow struct {
	ID          string
	OperatorID  string
	RequestedBy string
	Status      string
	ObjectKey   string
	SizeBytes   int64
	Error       string
	RequestedAt time.Time
	CompletedAt *time.Time
	ExpiresAt   *time.Time
}

// Request opens a new export. Idempotent on the caller's key, so a
// double-clicked button opens one job, not two.
func (r *DataExportRepository) Request(ctx context.Context, operatorID, requestedBy, idempotencyKey string) (DataExportRow, error) {
	row := DataExportRow{}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return row, apperror.ErrValidation
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO operator_data_exports (operator_id, requested_by, idempotency_key)
		VALUES ($1,$2,$3)
		ON CONFLICT (idempotency_key) DO UPDATE SET idempotency_key = EXCLUDED.idempotency_key
		RETURNING id::text, operator_id::text, requested_by, status, object_key, size_bytes, error,
		          requested_at, completed_at, expires_at`,
		operator, requestedBy, idempotencyKey).
		Scan(&row.ID, &row.OperatorID, &row.RequestedBy, &row.Status, &row.ObjectKey, &row.SizeBytes,
			&row.Error, &row.RequestedAt, &row.CompletedAt, &row.ExpiresAt)
	return row, databaseError(err)
}

func (r *DataExportRepository) ListForOperator(ctx context.Context, operatorID string, limit int32) ([]DataExportRow, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, operator_id::text, requested_by, status, object_key, size_bytes, error,
		       requested_at, completed_at, expires_at
		FROM operator_data_exports
		WHERE operator_id = $1
		ORDER BY requested_at DESC LIMIT $2`, operator, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	exports := make([]DataExportRow, 0)
	for rows.Next() {
		var row DataExportRow
		if err := rows.Scan(&row.ID, &row.OperatorID, &row.RequestedBy, &row.Status, &row.ObjectKey,
			&row.SizeBytes, &row.Error, &row.RequestedAt, &row.CompletedAt, &row.ExpiresAt); err != nil {
			return nil, err
		}
		exports = append(exports, row)
	}
	return exports, rows.Err()
}

// Get is scoped to the operator so one tenant can never fetch the download key
// for another tenant's export by guessing its id.
func (r *DataExportRepository) Get(ctx context.Context, operatorID, exportID string) (DataExportRow, error) {
	row := DataExportRow{}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return row, apperror.ErrValidation
	}
	export, err := pgUUID(exportID)
	if err != nil {
		return row, apperror.ErrValidation
	}
	err = r.pool.QueryRow(ctx, `
		SELECT id::text, operator_id::text, requested_by, status, object_key, size_bytes, error,
		       requested_at, completed_at, expires_at
		FROM operator_data_exports WHERE id = $1 AND operator_id = $2`, export, operator).
		Scan(&row.ID, &row.OperatorID, &row.RequestedBy, &row.Status, &row.ObjectKey, &row.SizeBytes,
			&row.Error, &row.RequestedAt, &row.CompletedAt, &row.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, apperror.ErrNotFound
	}
	return row, databaseError(err)
}

// ClaimNext takes the oldest unstarted request, if there is one, and marks it
// PROCESSING in the same statement — the row itself is the lock, so two
// worker instances racing on the same request cannot both pick it up.
func (r *DataExportRepository) ClaimNext(ctx context.Context) (DataExportRow, bool, error) {
	row := DataExportRow{}
	err := r.pool.QueryRow(ctx, `
		UPDATE operator_data_exports SET status = 'PROCESSING'
		WHERE id = (
		  SELECT id FROM operator_data_exports WHERE status = 'PENDING'
		  ORDER BY requested_at LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		RETURNING id::text, operator_id::text, requested_by, status, object_key, size_bytes, error,
		          requested_at, completed_at, expires_at`).
		Scan(&row.ID, &row.OperatorID, &row.RequestedBy, &row.Status, &row.ObjectKey, &row.SizeBytes,
			&row.Error, &row.RequestedAt, &row.CompletedAt, &row.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, false, nil
	}
	if err != nil {
		return row, false, databaseError(err)
	}
	return row, true, nil
}

func (r *DataExportRepository) MarkReady(ctx context.Context, exportID, objectKey string, sizeBytes int64, expiresAt time.Time) error {
	export, err := pgUUID(exportID)
	if err != nil {
		return apperror.ErrValidation
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE operator_data_exports
		SET status = 'READY', object_key = $2, size_bytes = $3, completed_at = NOW(), expires_at = $4
		WHERE id = $1`, export, objectKey, sizeBytes, expiresAt)
	return databaseError(err)
}

func (r *DataExportRepository) MarkFailed(ctx context.Context, exportID, message string) error {
	export, err := pgUUID(exportID)
	if err != nil {
		return apperror.ErrValidation
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE operator_data_exports SET status = 'FAILED', error = $2, completed_at = NOW()
		WHERE id = $1`, export, message)
	return databaseError(err)
}

// ListExpired is what the cleanup sweep reads: READY exports whose link has
// lapsed, so their file can be deleted from storage and not just forgotten
// about in the database.
func (r *DataExportRepository) ListExpired(ctx context.Context, limit int32) ([]DataExportRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, operator_id::text, requested_by, status, object_key, size_bytes, error,
		       requested_at, completed_at, expires_at
		FROM operator_data_exports
		WHERE status = 'READY' AND expires_at IS NOT NULL AND expires_at < NOW()
		LIMIT $1`, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	exports := make([]DataExportRow, 0)
	for rows.Next() {
		var row DataExportRow
		if err := rows.Scan(&row.ID, &row.OperatorID, &row.RequestedBy, &row.Status, &row.ObjectKey,
			&row.SizeBytes, &row.Error, &row.RequestedAt, &row.CompletedAt, &row.ExpiresAt); err != nil {
			return nil, err
		}
		exports = append(exports, row)
	}
	return exports, rows.Err()
}

// MarkExpired flips a READY export whose file has been deleted so it stops
// pretending a download link still works. Never a status a caller can set
// directly — only the sweep that actually deleted the object gets to say so.
func (r *DataExportRepository) MarkExpired(ctx context.Context, exportID string) error {
	export, err := pgUUID(exportID)
	if err != nil {
		return apperror.ErrValidation
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE operator_data_exports SET status = 'FAILED', error = 'kedaluwarsa dan berkasnya sudah dihapus'
		WHERE id = $1 AND status = 'READY'`, export)
	return databaseError(err)
}

// HasReadyExport is D6's hard gate (TUGAS-PANEL-SAAS.md, §7.3 DESAIN):
// "ekspor data tenant wajib ditawarkan" before a tenant can be deleted. Not
// whether the tenant downloaded it — proving that is unnecessary and mostly
// unenforceable — only whether one was actually produced and is sitting
// there available. An expired link still counts: the file existed and the
// portability right was honoured at the time.
func (r *DataExportRepository) HasReadyExport(ctx context.Context, operatorID string) (bool, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	var exists bool
	err = r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operator_data_exports WHERE operator_id = $1 AND status = 'READY')`, operator).Scan(&exists)
	return exists, databaseError(err)
}
