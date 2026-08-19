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
)

type WaitlistRepository struct {
	queries *db.Queries
}

func NewWaitlistRepository(queries *db.Queries) *WaitlistRepository {
	return &WaitlistRepository{queries: queries}
}

// Join checks capacity server-side, rejects a season that still has room
// (caller should redirect to registration instead), rejects a duplicate
// email for this season, then inserts. Returns (entry, isFull, error) —
// isFull=false with a nil entry and nil error means "season has capacity,
// this call added nothing."
func (r *WaitlistRepository) Join(ctx context.Context, operatorID, seasonID, fullName, email, phone, productID string) (*domain.WaitlistEntry, bool, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	season, err := r.queries.GetSeasonCapacity(ctx, db.GetSeasonCapacityParams{ID: seasonUUID, OperatorID: opUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, apperror.ErrNotFound
	}
	if err != nil {
		return nil, false, err
	}
	count, err := r.queries.CountSeasonPilgrims(ctx, db.CountSeasonPilgrimsParams{SeasonID: seasonUUID, OperatorID: opUUID})
	if err != nil {
		return nil, false, err
	}
	isFull := season > 0 && count >= int64(season)
	if !isFull {
		return nil, false, nil
	}
	_, err = r.queries.GetWaitlistEntryByEmail(ctx, db.GetWaitlistEntryByEmailParams{SeasonID: seasonUUID, Email: email})
	if err == nil {
		return nil, true, apperror.ErrAlreadyExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, true, err
	}
	entry, err := r.queries.JoinWaitlist(ctx, db.JoinWaitlistParams{
		OperatorID: opUUID, SeasonID: seasonUUID, FullName: fullName, Email: email, Phone: phone, ProductID: pgUUIDOrNull(productID),
	})
	if err != nil {
		return nil, true, databaseError(err)
	}
	return toWaitlistEntry(entry), true, nil
}

func (r *WaitlistRepository) List(ctx context.Context, operatorID, seasonID string) ([]*domain.WaitlistEntry, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListWaitlist(ctx, db.ListWaitlistParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.WaitlistEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, toWaitlistEntry(row))
	}
	return result, nil
}

func (r *WaitlistRepository) Promote(ctx context.Context, operatorID, id string) (*domain.WaitlistEntry, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	entry, err := r.queries.PromoteWaitlistEntry(ctx, db.PromoteWaitlistEntryParams{ID: idUUID, OperatorID: opUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	return toWaitlistEntry(entry), nil
}

// PromoteNextWaiting is called after a cancellation frees a slot. Returns
// nil (not an error) when no one is waiting.
func (r *WaitlistRepository) PromoteNextWaiting(ctx context.Context, operatorID, seasonID string) (*domain.WaitlistEntry, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	next, err := r.queries.GetNextWaiting(ctx, seasonUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	promoted, err := r.queries.PromoteWaitlistEntry(ctx, db.PromoteWaitlistEntryParams{ID: next.ID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	return toWaitlistEntry(promoted), nil
}

func (r *WaitlistRepository) Remove(ctx context.Context, operatorID, id string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	return databaseError(r.queries.RemoveFromWaitlist(ctx, db.RemoveFromWaitlistParams{ID: idUUID, OperatorID: opUUID}))
}

// Leave is the public counterpart of Remove — authenticated by email
// match instead of an operator session.
func (r *WaitlistRepository) Leave(ctx context.Context, seasonID, email string) error {
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return apperror.ErrValidation
	}
	return databaseError(r.queries.LeaveWaitlist(ctx, db.LeaveWaitlistParams{SeasonID: seasonUUID, Email: email}))
}

func (r *WaitlistRepository) ConfirmSlot(ctx context.Context, id, seasonID, email string) (*domain.WaitlistEntry, error) {
	idUUID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	entry, err := r.queries.ConfirmWaitlistEntry(ctx, db.ConfirmWaitlistEntryParams{ID: idUUID, SeasonID: seasonUUID, Email: email})
	if err != nil {
		return nil, databaseError(err)
	}
	return toWaitlistEntry(entry), nil
}

// ExpireStale flips every stale PROMOTED entry (past its 48h window) to
// EXPIRED and returns the distinct (operatorID, seasonID) pairs affected
// so the worker can promote the next person in line for each. Best-effort
// sweep, not part of any other transaction.
func (r *WaitlistRepository) ExpireStale(ctx context.Context) ([]struct{ OperatorID, SeasonID string }, error) {
	rows, err := r.queries.ExpirePromotedEntries(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	result := make([]struct{ OperatorID, SeasonID string }, 0, len(rows))
	for _, row := range rows {
		operatorID := uuid.UUID(row.OperatorID.Bytes).String()
		seasonID := uuid.UUID(row.SeasonID.Bytes).String()
		key := operatorID + ":" + seasonID
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, struct{ OperatorID, SeasonID string }{OperatorID: operatorID, SeasonID: seasonID})
	}
	return result, nil
}

func pgUUIDOrNull(value string) pgtype.UUID {
	if value == "" {
		return pgtype.UUID{}
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func toWaitlistEntry(value db.SeasonWaitlist) *domain.WaitlistEntry {
	entry := &domain.WaitlistEntry{
		ID:         uuid.UUID(value.ID.Bytes).String(),
		OperatorID: uuid.UUID(value.OperatorID.Bytes).String(),
		SeasonID:   uuid.UUID(value.SeasonID.Bytes).String(),
		FullName:   value.FullName,
		Email:      value.Email,
		Phone:      value.Phone,
		Position:   value.Position,
		Status:     value.Status,
		CreatedAt:  value.CreatedAt.Time,
	}
	if value.ProductID.Valid {
		entry.ProductID = uuid.UUID(value.ProductID.Bytes).String()
	}
	if value.PromotedAt.Valid {
		t := value.PromotedAt.Time
		entry.PromotedAt = &t
	}
	if value.ExpiresAt.Valid {
		t := value.ExpiresAt.Time
		entry.ExpiresAt = &t
	}
	return entry
}
