package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CancellationRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewCancellationRepository(pool *pgxpool.Pool, queries *db.Queries) *CancellationRepository {
	return &CancellationRepository{pool: pool, queries: queries}
}

func (r *CancellationRepository) CreatePolicy(ctx context.Context, operatorID, seasonID, name string, minDays int32, refundPct float64, sortOrder int32) (*domain.CancellationPolicy, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	policy, err := r.queries.CreateCancellationPolicy(ctx, db.CreateCancellationPolicyParams{
		OperatorID: opUUID, SeasonID: seasonUUID, Name: name, MinDays: minDays, RefundPct: refundPct, SortOrder: sortOrder,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toCancellationPolicy(policy), nil
}

func (r *CancellationRepository) ListPolicies(ctx context.Context, operatorID, seasonID string) ([]*domain.CancellationPolicy, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListCancellationPolicies(ctx, db.ListCancellationPoliciesParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.CancellationPolicy, 0, len(rows))
	for _, row := range rows {
		result = append(result, toCancellationPolicy(row))
	}
	return result, nil
}

func (r *CancellationRepository) DeletePolicy(ctx context.Context, operatorID, id string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	return databaseError(r.queries.DeleteCancellationPolicy(ctx, db.DeleteCancellationPolicyParams{ID: idUUID, OperatorID: opUUID}))
}

// MatchPolicy returns the matched tier (nil if none matches, i.e. 0%
// refund) and the pilgrim's actual paid total computed live from orders —
// never from a cached balance column.
func (r *CancellationRepository) MatchPolicy(ctx context.Context, seasonID string, daysBefore int32) (*domain.CancellationPolicy, error) {
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	policy, err := r.queries.GetMatchingPolicy(ctx, db.GetMatchingPolicyParams{SeasonID: seasonUUID, MinDays: daysBefore})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toCancellationPolicy(policy), nil
}

func (r *CancellationRepository) GetPaidTotal(ctx context.Context, pilgrimID string) (int64, error) {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return 0, apperror.ErrValidation
	}
	return r.queries.GetPilgrimPaidTotal(ctx, pilgrimUUID)
}

func (r *CancellationRepository) GetByPilgrimID(ctx context.Context, operatorID, pilgrimID string) (*domain.PilgrimCancellation, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.GetPilgrimCancellation(ctx, db.GetPilgrimCancellationParams{PilgrimID: pilgrimUUID, OperatorID: opUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.PilgrimCancellation{
		ID: uuid.UUID(row.ID.Bytes).String(), PilgrimID: uuid.UUID(row.PilgrimID.Bytes).String(),
		OperatorID: uuid.UUID(row.OperatorID.Bytes).String(), SeasonID: uuid.UUID(row.SeasonID.Bytes).String(),
		Reason: row.Reason, DaysBefore: row.DaysBefore, RefundPct: row.RefundPct,
		RefundAmountIDR: row.RefundAmountIdr, TotalPaidIDR: row.TotalPaidIdr,
		CancelledBy: row.CancelledBy, CancelledAt: row.CancelledAt.Time,
	}, nil
}

func (r *CancellationRepository) ListCancellations(ctx context.Context, operatorID, seasonID string) ([]*domain.PilgrimCancellation, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListCancellations(ctx, db.ListCancellationsParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.PilgrimCancellation, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.PilgrimCancellation{
			ID: uuid.UUID(row.ID.Bytes).String(), PilgrimID: uuid.UUID(row.PilgrimID.Bytes).String(), PilgrimName: row.PilgrimName,
			OperatorID: uuid.UUID(row.OperatorID.Bytes).String(), SeasonID: uuid.UUID(row.SeasonID.Bytes).String(),
			Reason: row.Reason, DaysBefore: row.DaysBefore, RefundPct: row.RefundPct,
			RefundAmountIDR: row.RefundAmountIdr, TotalPaidIDR: row.TotalPaidIdr,
			CancelledBy: row.CancelledBy, CancelledAt: row.CancelledAt.Time,
		})
	}
	return result, nil
}

// ConfirmCancellation runs atomically: re-fetches the pilgrim inside the
// transaction (closes the TOCTOU window a plain preview-then-confirm
// would leave open), refuses if already cancelled, inserts the immutable
// cancellation record, and marks the pilgrim CANCELLED. Every number
// written here is recomputed inside this function from operatorID/
// pilgrimID/reason/cancelledBy alone — nothing from a client-supplied
// preview is trusted.
func (r *CancellationRepository) ConfirmCancellation(ctx context.Context, operatorID, pilgrimID, seasonID, reason, cancelledBy string, daysBefore int32, refundPct float64, refundAmountIDR, totalPaidIDR int64, policyID string) (*domain.PilgrimCancellation, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.queries.WithTx(tx)

	pilgrim, err := qtx.GetPilgrim(ctx, db.GetPilgrimParams{ID: pilgrimUUID, OperatorID: opUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if pilgrim.Status == "CANCELLED" {
		return nil, apperror.ErrFailedPrecondition
	}

	var policyUUID pgtype.UUID
	if policyID != "" {
		policyUUID, err = pgUUID(policyID)
		if err != nil {
			return nil, apperror.ErrValidation
		}
	}

	cancellation, err := qtx.CreateCancellation(ctx, db.CreateCancellationParams{
		PilgrimID: pilgrimUUID, OperatorID: opUUID, SeasonID: seasonUUID, Reason: reason, DaysBefore: daysBefore,
		RefundPct: refundPct, RefundAmountIdr: refundAmountIDR, TotalPaidIdr: totalPaidIDR, CancelledBy: cancelledBy, PolicyID: policyUUID,
	})
	if err != nil {
		return nil, databaseError(err)
	}

	if err := qtx.MarkPilgrimCancelled(ctx, db.MarkPilgrimCancelledParams{ID: pilgrimUUID, OperatorID: opUUID}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &domain.PilgrimCancellation{
		ID: uuid.UUID(cancellation.ID.Bytes).String(), PilgrimID: pilgrimID, OperatorID: operatorID, SeasonID: seasonID,
		Reason: reason, DaysBefore: daysBefore, RefundPct: refundPct, RefundAmountIDR: refundAmountIDR,
		TotalPaidIDR: totalPaidIDR, CancelledBy: cancelledBy, CancelledAt: cancellation.CancelledAt.Time,
	}, nil
}

func toCancellationPolicy(value db.CancellationPolicy) *domain.CancellationPolicy {
	return &domain.CancellationPolicy{
		ID: uuid.UUID(value.ID.Bytes).String(), OperatorID: uuid.UUID(value.OperatorID.Bytes).String(),
		SeasonID: uuid.UUID(value.SeasonID.Bytes).String(), Name: value.Name,
		MinDays: value.MinDays, RefundPct: value.RefundPct, SortOrder: value.SortOrder,
	}
}
