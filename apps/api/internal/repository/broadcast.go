package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

type BroadcastRepository struct{ queries *db.Queries }

func NewBroadcastRepository(queries *db.Queries) *BroadcastRepository {
	return &BroadcastRepository{queries: queries}
}

func (r *BroadcastRepository) Create(ctx context.Context, operatorID, seasonID, title, body string) (*domain.Broadcast, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateBroadcast(ctx, db.CreateBroadcastParams{OperatorID: opUUID, SeasonID: seasonUUID, Title: title, Body: body})
	if err != nil {
		return nil, databaseError(err)
	}
	return toBroadcast(row), nil
}

func (r *BroadcastRepository) List(ctx context.Context, operatorID, seasonID string) ([]*domain.Broadcast, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListBroadcasts(ctx, db.ListBroadcastsParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Broadcast, 0, len(rows))
	for _, row := range rows {
		result = append(result, toBroadcast(row))
	}
	return result, nil
}

func (r *BroadcastRepository) Delete(ctx context.Context, operatorID, broadcastID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	broadcastUUID, err := pgUUID(broadcastID)
	if err != nil {
		return err
	}
	return databaseError(r.queries.DeleteBroadcast(ctx, db.DeleteBroadcastParams{ID: broadcastUUID, OperatorID: opUUID}))
}

func toBroadcast(row db.Broadcast) *domain.Broadcast {
	return &domain.Broadcast{
		ID:         uuid.UUID(row.ID.Bytes).String(),
		OperatorID: uuid.UUID(row.OperatorID.Bytes).String(),
		SeasonID:   uuid.UUID(row.SeasonID.Bytes).String(),
		Title:      row.Title,
		Body:       row.Body,
		CreatedAt:  row.CreatedAt.Time,
	}
}
