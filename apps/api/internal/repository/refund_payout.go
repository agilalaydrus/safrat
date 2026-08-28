package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefundPayoutRepository struct {
	pool *pgxpool.Pool
}

func NewRefundPayoutRepository(pool *pgxpool.Pool) *RefundPayoutRepository {
	return &RefundPayoutRepository{pool: pool}
}

func (r *RefundPayoutRepository) UserHasTwoFactor(ctx context.Context, userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, apperror.ErrValidation
	}
	var enabled bool
	err := r.pool.QueryRow(ctx, `SELECT COALESCE("twoFactorEnabled", false) FROM "user" WHERE id = $1`, userID).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, apperror.ErrNotFound
	}
	return enabled, err
}

func (r *RefundPayoutRepository) ReservedForPilgrim(ctx context.Context, pilgrimID string) (int64, error) {
	return reservedForPilgrim(ctx, r.pool, pilgrimID)
}

func (r *RefundPayoutRepository) ReservedForPilgrimTx(ctx context.Context, tx pgx.Tx, pilgrimID string) (int64, error) {
	return reservedForPilgrim(ctx, tx, pilgrimID)
}

type payoutQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func reservedForPilgrim(ctx context.Context, query payoutQuerier, pilgrimID string) (int64, error) {
	id, err := pgUUID(pilgrimID)
	if err != nil {
		return 0, apperror.ErrValidation
	}
	var total int64
	err = query.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_idr), 0)::bigint
		FROM pilgrim_refund_payout_requests
		WHERE pilgrim_id = $1 AND status IN ('REQUESTED', 'PROCESSING')`, id).Scan(&total)
	return total, err
}

func (r *RefundPayoutRepository) FindByKeyTx(ctx context.Context, tx pgx.Tx, pilgrimID, key string) (*domain.RefundPayoutRequest, error) {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil || strings.TrimSpace(key) == "" {
		return nil, apperror.ErrValidation
	}
	request, err := scanRefundPayout(tx.QueryRow(ctx, refundPayoutSelect+`
		WHERE pr.pilgrim_id = $1 AND pr.idempotency_key = $2`, pilgrimUUID, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return request, err
}

type CreateRefundPayoutParams struct {
	OperatorID, PilgrimID, Method, Note, IdempotencyKey, RequestedByUserID string
	AmountIDR                                                              int64
}

func (r *RefundPayoutRepository) CreateTx(ctx context.Context, tx pgx.Tx, p CreateRefundPayoutParams) (*domain.RefundPayoutRequest, error) {
	opID, err := pgUUID(p.OperatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	pilgrimID, err := pgUUID(p.PilgrimID)
	if err != nil || p.AmountIDR <= 0 || strings.TrimSpace(p.IdempotencyKey) == "" {
		return nil, apperror.ErrValidation
	}
	return scanRefundPayout(tx.QueryRow(ctx, refundPayoutInsertReturning,
		opID, pilgrimID, p.AmountIDR, p.Method, p.Note, p.IdempotencyKey, p.RequestedByUserID))
}

func (r *RefundPayoutRepository) ListForPilgrim(ctx context.Context, pilgrimID string) ([]*domain.RefundPayoutRequest, error) {
	id, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, refundPayoutSelect+`
		WHERE pr.pilgrim_id = $1 ORDER BY pr.created_at DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRefundPayouts(rows)
}

func (r *RefundPayoutRepository) ListByOperator(ctx context.Context, operatorID, status string) ([]*domain.RefundPayoutRequest, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, refundPayoutSelect+`
		WHERE pr.operator_id = $1 AND ($2 = '' OR pr.status = $2)
		ORDER BY CASE pr.status WHEN 'REQUESTED' THEN 0 WHEN 'PROCESSING' THEN 1 ELSE 2 END,
		         pr.created_at DESC`, id, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRefundPayouts(rows)
}

func (r *RefundPayoutRepository) LockByIDTx(ctx context.Context, tx pgx.Tx, operatorID, requestID string) (*domain.RefundPayoutRequest, error) {
	opID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	id, err := pgUUID(requestID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	request, err := scanRefundPayout(tx.QueryRow(ctx, refundPayoutSelect+`
		WHERE pr.operator_id = $1 AND pr.id = $2 FOR UPDATE OF pr`, opID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	return request, err
}

func (r *RefundPayoutRepository) TransitionTx(ctx context.Context, tx pgx.Tx, requestID, status, userID, note, paymentReference string) (*domain.RefundPayoutRequest, error) {
	id, err := pgUUID(requestID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	return scanRefundPayout(tx.QueryRow(ctx, `
		WITH changed AS (
		  UPDATE pilgrim_refund_payout_requests
		  SET status = $2::text,
		      processed_by_user_id = NULLIF($3::text, ''),
		      resolution_note = $4::text,
		      payment_reference = $5::text,
		      processing_at = CASE WHEN $2::text = 'PROCESSING' THEN COALESCE(processing_at, NOW()) ELSE processing_at END,
		      resolved_at = CASE WHEN $2::text IN ('PAID', 'FAILED') THEN COALESCE(resolved_at, NOW()) ELSE resolved_at END
		  WHERE id = $1
		  RETURNING *
		)
		SELECT changed.id::text, changed.operator_id::text, changed.pilgrim_id::text,
		       p.full_name, COALESCE(p.phone, ''), changed.amount_idr, changed.method,
		       changed.note, changed.status, changed.idempotency_key,
		       changed.requested_by_user_id, COALESCE(changed.processed_by_user_id, ''),
		       changed.resolution_note, changed.payment_reference, changed.processing_at,
		       changed.resolved_at, changed.created_at, changed.updated_at
		FROM changed JOIN pilgrims p ON p.id = changed.pilgrim_id`, id, status, userID, note, paymentReference))
}

const refundPayoutSelect = `
	SELECT pr.id::text, pr.operator_id::text, pr.pilgrim_id::text,
	       p.full_name, COALESCE(p.phone, ''), pr.amount_idr, pr.method,
	       pr.note, pr.status, pr.idempotency_key, pr.requested_by_user_id,
	       COALESCE(pr.processed_by_user_id, ''), pr.resolution_note,
	       pr.payment_reference, pr.processing_at, pr.resolved_at,
	       pr.created_at, pr.updated_at
	FROM pilgrim_refund_payout_requests pr
	JOIN pilgrims p ON p.id = pr.pilgrim_id `

const refundPayoutInsertReturning = `
	WITH inserted AS (
	  INSERT INTO pilgrim_refund_payout_requests
	    (operator_id, pilgrim_id, amount_idr, method, note, idempotency_key, requested_by_user_id)
	  VALUES ($1, $2, $3, $4, $5, $6, $7)
	  RETURNING *
	)
	SELECT inserted.id::text, inserted.operator_id::text, inserted.pilgrim_id::text,
	       p.full_name, COALESCE(p.phone, ''), inserted.amount_idr, inserted.method,
	       inserted.note, inserted.status, inserted.idempotency_key,
	       inserted.requested_by_user_id, COALESCE(inserted.processed_by_user_id, ''),
	       inserted.resolution_note, inserted.payment_reference, inserted.processing_at,
	       inserted.resolved_at, inserted.created_at, inserted.updated_at
	FROM inserted JOIN pilgrims p ON p.id = inserted.pilgrim_id`

func scanRefundPayout(row rowScanner) (*domain.RefundPayoutRequest, error) {
	var request domain.RefundPayoutRequest
	err := row.Scan(&request.ID, &request.OperatorID, &request.PilgrimID,
		&request.PilgrimName, &request.PilgrimPhone, &request.AmountIDR,
		&request.Method, &request.Note, &request.Status, &request.IdempotencyKey,
		&request.RequestedByUserID, &request.ProcessedByUserID,
		&request.ResolutionNote, &request.PaymentReference,
		&request.ProcessingAt, &request.ResolvedAt, &request.CreatedAt, &request.UpdatedAt)
	return &request, err
}

func scanRefundPayouts(rows pgx.Rows) ([]*domain.RefundPayoutRequest, error) {
	requests := make([]*domain.RefundPayoutRequest, 0)
	for rows.Next() {
		request, err := scanRefundPayout(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}
