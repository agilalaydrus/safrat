package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	appcrypto "github.com/hajj-saas/api/internal/crypto"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefundPayoutRepository struct {
	pool   *pgxpool.Pool
	sealer *appcrypto.Sealer
}

func NewRefundPayoutRepository(pool *pgxpool.Pool, sealers ...*appcrypto.Sealer) *RefundPayoutRepository {
	var sealer *appcrypto.Sealer
	if len(sealers) > 0 {
		sealer = sealers[0]
	}
	return &RefundPayoutRepository{pool: pool, sealer: sealer}
}

func (r *RefundPayoutRepository) UserHasTwoFactor(ctx context.Context, userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, apperror.ErrValidation
	}
	var enabled bool
	err := r.pool.QueryRow(ctx, `SELECT COALESCE("twoFactorEnabled", false) FROM "user" WHERE id=$1`, userID).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, apperror.ErrNotFound
	}
	return enabled, err
}

type payoutQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *RefundPayoutRepository) ReservedForPilgrim(ctx context.Context, id string) (int64, error) {
	return r.ReservedForBeneficiary(ctx, "PILGRIM", id)
}
func (r *RefundPayoutRepository) ReservedForPilgrimTx(ctx context.Context, tx pgx.Tx, id string) (int64, error) {
	return reservedForBeneficiary(ctx, tx, "PILGRIM", id)
}
func (r *RefundPayoutRepository) ReservedForBeneficiary(ctx context.Context, kind, id string) (int64, error) {
	return reservedForBeneficiary(ctx, r.pool, kind, id)
}
func (r *RefundPayoutRepository) ReservedForBeneficiaryTx(ctx context.Context, tx pgx.Tx, kind, id string) (int64, error) {
	return reservedForBeneficiary(ctx, tx, kind, id)
}
func reservedForBeneficiary(ctx context.Context, q payoutQuerier, kind, id string) (int64, error) {
	uuid, err := pgUUID(id)
	if err != nil {
		return 0, apperror.ErrValidation
	}
	column := "pilgrim_id"
	if kind == "AGENT" {
		column = "agent_id"
	} else if kind != "PILGRIM" {
		return 0, apperror.ErrValidation
	}
	var total int64
	err = q.QueryRow(ctx, `SELECT COALESCE(SUM(amount_idr),0)::bigint FROM pilgrim_refund_payout_requests WHERE `+column+`=$1 AND status IN ('REQUESTED','PROCESSING')`, uuid).Scan(&total)
	return total, err
}

func (r *RefundPayoutRepository) FindByKeyTx(ctx context.Context, tx pgx.Tx, kind, ownerID, key string) (*domain.RefundPayoutRequest, error) {
	id, err := pgUUID(ownerID)
	if err != nil || strings.TrimSpace(key) == "" {
		return nil, apperror.ErrValidation
	}
	column := "pr.pilgrim_id"
	if kind == "AGENT" {
		column = "pr.agent_id"
	} else if kind != "PILGRIM" {
		return nil, apperror.ErrValidation
	}
	request, err := scanRefundPayout(tx.QueryRow(ctx, refundPayoutSelect+` WHERE `+column+`=$1 AND pr.idempotency_key=$2`, id, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return request, err
}

type CreateRefundPayoutParams struct {
	OperatorID, BeneficiaryKind, BeneficiaryID                             string
	Method, Note, IdempotencyKey, RequestedByUserID                        string
	DestinationChannel, DestinationAccountHolder, DestinationAccountNumber string
	AmountIDR                                                              int64
}

func (r *RefundPayoutRepository) CreateTx(ctx context.Context, tx pgx.Tx, p CreateRefundPayoutParams) (*domain.RefundPayoutRequest, error) {
	opID, err := pgUUID(p.OperatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	ownerID, err := pgUUID(p.BeneficiaryID)
	if err != nil || p.AmountIDR <= 0 || strings.TrimSpace(p.IdempotencyKey) == "" {
		return nil, apperror.ErrValidation
	}
	var pilgrimID, agentID any
	if p.BeneficiaryKind == "PILGRIM" {
		pilgrimID = ownerID
	} else if p.BeneficiaryKind == "AGENT" {
		agentID = ownerID
	} else {
		return nil, apperror.ErrValidation
	}
	account := strings.TrimSpace(p.DestinationAccountNumber)
	encrypted, last4 := "", ""
	if p.Method != "CASH" {
		if account == "" || strings.TrimSpace(p.DestinationChannel) == "" || strings.TrimSpace(p.DestinationAccountHolder) == "" {
			return nil, apperror.ErrValidation
		}
		encrypted, err = r.sealer.Seal(account)
		if err != nil {
			return nil, err
		}
		if len(account) <= 4 {
			last4 = account
		} else {
			last4 = account[len(account)-4:]
		}
	}
	return scanRefundPayout(tx.QueryRow(ctx, refundPayoutInsertReturning,
		opID, p.BeneficiaryKind, pilgrimID, agentID, p.AmountIDR, p.Method, p.Note,
		p.IdempotencyKey, p.RequestedByUserID, strings.TrimSpace(p.DestinationChannel),
		strings.TrimSpace(p.DestinationAccountHolder), encrypted, last4))
}

func (r *RefundPayoutRepository) DestinationAccount(request *domain.RefundPayoutRequest) (string, error) {
	if request == nil {
		return "", apperror.ErrValidation
	}
	return r.sealer.Open(request.DestinationAccountEncrypted)
}

func (r *RefundPayoutRepository) ListForPilgrim(ctx context.Context, id string) ([]*domain.RefundPayoutRequest, error) {
	return r.listFor(ctx, "PILGRIM", id)
}
func (r *RefundPayoutRepository) ListForAgent(ctx context.Context, id string) ([]*domain.RefundPayoutRequest, error) {
	return r.listFor(ctx, "AGENT", id)
}
func (r *RefundPayoutRepository) listFor(ctx context.Context, kind, id string) ([]*domain.RefundPayoutRequest, error) {
	uuid, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	column := "pr.pilgrim_id"
	if kind == "AGENT" {
		column = "pr.agent_id"
	}
	rows, err := r.pool.Query(ctx, refundPayoutSelect+` WHERE `+column+`=$1 ORDER BY pr.created_at DESC`, uuid)
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
	rows, err := r.pool.Query(ctx, refundPayoutSelect+` WHERE pr.operator_id=$1 AND ($2='' OR pr.status=$2) ORDER BY CASE pr.status WHEN 'REQUESTED' THEN 0 WHEN 'PROCESSING' THEN 1 ELSE 2 END, pr.created_at DESC`, id, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRefundPayouts(rows)
}

func (r *RefundPayoutRepository) ListGatewayWork(ctx context.Context, limit int) ([]*domain.RefundPayoutRequest, error) {
	if limit < 1 {
		limit = 25
	}
	rows, err := r.pool.Query(ctx, refundPayoutSelect+`
		WHERE pr.method <> 'CASH' AND (pr.status='REQUESTED' OR
		 (pr.status='PROCESSING' AND (pr.provider_last_attempt_at IS NULL OR pr.provider_last_attempt_at < NOW()-INTERVAL '1 minute')))
		ORDER BY pr.created_at LIMIT $1`, limit)
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
	request, err := scanRefundPayout(tx.QueryRow(ctx, refundPayoutSelect+` WHERE pr.operator_id=$1 AND pr.id=$2 FOR UPDATE OF pr`, opID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	return request, err
}
func (r *RefundPayoutRepository) LockByReferenceTx(ctx context.Context, tx pgx.Tx, requestID string) (*domain.RefundPayoutRequest, error) {
	id, err := pgUUID(requestID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	request, err := scanRefundPayout(tx.QueryRow(ctx, refundPayoutSelect+` WHERE pr.id=$1 FOR UPDATE OF pr`, id))
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
	return scanRefundPayout(tx.QueryRow(ctx, refundPayoutChangedSelect+`
		UPDATE pilgrim_refund_payout_requests SET status=$2::text, processed_by_user_id=NULLIF($3::text,''), resolution_note=$4::text, payment_reference=$5::text,
		processing_at=CASE WHEN $2::text='PROCESSING' THEN COALESCE(processing_at,NOW()) ELSE processing_at END,
		resolved_at=CASE WHEN $2::text IN ('PAID','FAILED','REVERSED') THEN COALESCE(resolved_at,NOW()) ELSE resolved_at END
		WHERE id=$1 RETURNING * ) `+refundPayoutChangedJoin, id, status, userID, note, paymentReference))
}

func (r *RefundPayoutRepository) RecordProviderAttemptTx(ctx context.Context, tx pgx.Tx, requestID, payoutID, providerStatus, failureCode, paymentReference string) (*domain.RefundPayoutRequest, error) {
	id, err := pgUUID(requestID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	return scanRefundPayout(tx.QueryRow(ctx, refundPayoutChangedSelect+`
		UPDATE pilgrim_refund_payout_requests SET provider='XENDIT', provider_payout_id=COALESCE(NULLIF($2,''),provider_payout_id),
		provider_status=$3, provider_failure_code=$4, payment_reference=COALESCE(NULLIF($5,''),payment_reference), provider_last_attempt_at=NOW()
		WHERE id=$1 RETURNING * ) `+refundPayoutChangedJoin, id, payoutID, providerStatus, failureCode, paymentReference))
}

// TransitionProviderTx records the authenticated provider result and changes
// the workflow status in one statement. This is required for PAID->REVERSED:
// the database guard intentionally rejects any intermediate mutation of a
// paid financial record.
func (r *RefundPayoutRepository) TransitionProviderTx(ctx context.Context, tx pgx.Tx, requestID, status, payoutID, providerStatus, failureCode, paymentReference string) (*domain.RefundPayoutRequest, error) {
	id, err := pgUUID(requestID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	return scanRefundPayout(tx.QueryRow(ctx, refundPayoutChangedSelect+`
		UPDATE pilgrim_refund_payout_requests SET status=$2::text, provider='XENDIT',
		provider_payout_id=COALESCE(NULLIF($3,''),provider_payout_id), provider_status=$4,
		provider_failure_code=$5, payment_reference=COALESCE(NULLIF($6,''),payment_reference),
		provider_last_attempt_at=NOW(), processed_by_user_id='xendit',
		processing_at=COALESCE(processing_at,NOW()),
		resolved_at=CASE WHEN $2::text IN ('PAID','FAILED','REVERSED') THEN COALESCE(resolved_at,NOW()) ELSE resolved_at END
		WHERE id=$1 RETURNING * ) `+refundPayoutChangedJoin,
		id, status, payoutID, providerStatus, failureCode, paymentReference))
}

func (r *RefundPayoutRepository) SetProofTx(ctx context.Context, tx pgx.Tx, requestID, proofURL string) (*domain.RefundPayoutRequest, error) {
	id, err := pgUUID(requestID)
	if err != nil || strings.TrimSpace(proofURL) == "" {
		return nil, apperror.ErrValidation
	}
	return scanRefundPayout(tx.QueryRow(ctx, refundPayoutChangedSelect+`
		UPDATE pilgrim_refund_payout_requests SET proof_url=$2 WHERE id=$1 RETURNING * ) `+refundPayoutChangedJoin, id, proofURL))
}
func (r *RefundPayoutRepository) TouchProviderAttempt(ctx context.Context, requestID string) error {
	id, err := pgUUID(requestID)
	if err != nil {
		return apperror.ErrValidation
	}
	_, err = r.pool.Exec(ctx, `UPDATE pilgrim_refund_payout_requests SET provider_last_attempt_at=$2 WHERE id=$1 AND status='PROCESSING'`, id, time.Now())
	return err
}

const refundPayoutSelect = `
	SELECT pr.id::text, pr.operator_id::text, COALESCE(pr.pilgrim_id::text,''),
	 COALESCE(p.full_name,a.name,''), COALESCE(p.phone,a.phone,''), pr.amount_idr,
	 pr.method,pr.note,pr.status,pr.idempotency_key,pr.requested_by_user_id,COALESCE(pr.processed_by_user_id,''),
	 pr.resolution_note,pr.payment_reference,pr.processing_at,pr.resolved_at,pr.created_at,pr.updated_at,
	 pr.beneficiary_kind,COALESCE(pr.agent_id::text,''),pr.destination_channel,pr.destination_account_holder,
	 pr.destination_account_encrypted,pr.destination_account_last4,pr.provider,COALESCE(pr.provider_payout_id,''),
	 pr.provider_status,pr.provider_failure_code,pr.proof_url,pr.provider_last_attempt_at
	FROM pilgrim_refund_payout_requests pr LEFT JOIN pilgrims p ON p.id=pr.pilgrim_id LEFT JOIN agents a ON a.id=pr.agent_id `

const refundPayoutInsertReturning = `
	WITH inserted AS (INSERT INTO pilgrim_refund_payout_requests
	 (operator_id,beneficiary_kind,pilgrim_id,agent_id,amount_idr,method,note,idempotency_key,requested_by_user_id,destination_channel,destination_account_holder,destination_account_encrypted,destination_account_last4)
	 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING *)
	SELECT inserted.id::text,inserted.operator_id::text,COALESCE(inserted.pilgrim_id::text,''),
	 COALESCE(p.full_name,a.name,''),COALESCE(p.phone,a.phone,''),inserted.amount_idr,inserted.method,inserted.note,inserted.status,
	 inserted.idempotency_key,inserted.requested_by_user_id,COALESCE(inserted.processed_by_user_id,''),inserted.resolution_note,
	 inserted.payment_reference,inserted.processing_at,inserted.resolved_at,inserted.created_at,inserted.updated_at,
	 inserted.beneficiary_kind,COALESCE(inserted.agent_id::text,''),inserted.destination_channel,inserted.destination_account_holder,
	 inserted.destination_account_encrypted,inserted.destination_account_last4,inserted.provider,COALESCE(inserted.provider_payout_id,''),
	 inserted.provider_status,inserted.provider_failure_code,inserted.proof_url,inserted.provider_last_attempt_at
	FROM inserted LEFT JOIN pilgrims p ON p.id=inserted.pilgrim_id LEFT JOIN agents a ON a.id=inserted.agent_id`

const refundPayoutChangedSelect = `WITH changed AS (`
const refundPayoutChangedJoin = `SELECT changed.id::text,changed.operator_id::text,COALESCE(changed.pilgrim_id::text,''),
	COALESCE(p.full_name,a.name,''),COALESCE(p.phone,a.phone,''),changed.amount_idr,changed.method,changed.note,changed.status,
	changed.idempotency_key,changed.requested_by_user_id,COALESCE(changed.processed_by_user_id,''),changed.resolution_note,
	changed.payment_reference,changed.processing_at,changed.resolved_at,changed.created_at,changed.updated_at,
	changed.beneficiary_kind,COALESCE(changed.agent_id::text,''),changed.destination_channel,changed.destination_account_holder,
	changed.destination_account_encrypted,changed.destination_account_last4,changed.provider,COALESCE(changed.provider_payout_id,''),
	changed.provider_status,changed.provider_failure_code,changed.proof_url,changed.provider_last_attempt_at
	FROM changed LEFT JOIN pilgrims p ON p.id=changed.pilgrim_id LEFT JOIN agents a ON a.id=changed.agent_id`

func scanRefundPayout(row rowScanner) (*domain.RefundPayoutRequest, error) {
	var r domain.RefundPayoutRequest
	err := row.Scan(&r.ID, &r.OperatorID, &r.PilgrimID, &r.PilgrimName, &r.PilgrimPhone, &r.AmountIDR, &r.Method, &r.Note,
		&r.Status, &r.IdempotencyKey, &r.RequestedByUserID, &r.ProcessedByUserID, &r.ResolutionNote, &r.PaymentReference,
		&r.ProcessingAt, &r.ResolvedAt, &r.CreatedAt, &r.UpdatedAt, &r.BeneficiaryKind, &r.AgentID, &r.DestinationChannel,
		&r.DestinationAccountHolder, &r.DestinationAccountEncrypted, &r.DestinationAccountLast4, &r.Provider, &r.ProviderPayoutID,
		&r.ProviderStatus, &r.ProviderFailureCode, &r.ProofURL, &r.ProviderLastAttemptAt)
	return &r, err
}
func scanRefundPayouts(rows pgx.Rows) ([]*domain.RefundPayoutRequest, error) {
	result := make([]*domain.RefundPayoutRequest, 0)
	for rows.Next() {
		item, err := scanRefundPayout(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
