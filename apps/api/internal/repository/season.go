package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/jackc/pgx/v5/pgtype"
)

type SeasonRepository struct {
	queries *db.Queries
}

func NewSeasonRepository(queries *db.Queries) *SeasonRepository {
	return &SeasonRepository{queries: queries}
}

func (r *SeasonRepository) Create(ctx context.Context, operatorID, name string, seasonType domain.SeasonType, startDate, endDate time.Time) (*domain.Season, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	season, err := r.queries.CreateSeason(ctx, db.CreateSeasonParams{
		OperatorID: operatorUUID,
		Name:       name,
		Type:       databaseSeasonType(seasonType),
		StartDate:  pgTimestamp(startDate),
		EndDate:    pgTimestamp(endDate),
	})
	if err != nil {
		return nil, err
	}
	return toSeason(season), nil
}

func (r *SeasonRepository) ListByOperatorID(ctx context.Context, operatorID string) ([]*domain.Season, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasons, err := r.queries.ListSeasonsByOperatorID(ctx, operatorUUID)
	if err != nil {
		return nil, err
	}
	results := make([]*domain.Season, 0, len(seasons))
	for _, season := range seasons {
		results = append(results, toSeason(season))
	}
	return results, nil
}

func (r *SeasonRepository) SetActive(ctx context.Context, operatorID, seasonID string) ([]*domain.Season, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	seasons, err := r.queries.SetActiveSeason(ctx, db.SetActiveSeasonParams{ID: seasonUUID, OperatorID: operatorUUID})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.Season, 0, len(seasons))
	for _, season := range seasons {
		results = append(results, toSeason(season))
	}
	return results, nil
}

func pgUUID(value string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse UUID: %w", err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func pgTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func databaseSeasonType(value domain.SeasonType) db.SeasonType {
	if value == domain.SeasonTypeUmrah {
		return db.SeasonTypeUMRAH
	}
	return db.SeasonTypeHAJJ
}

func toSeason(value db.Season) *domain.Season {
	seasonType := domain.SeasonTypeHajj
	if value.Type == db.SeasonTypeUMRAH {
		seasonType = domain.SeasonTypeUmrah
	}
	return &domain.Season{
		ID:         uuid.UUID(value.ID.Bytes).String(),
		OperatorID: uuid.UUID(value.OperatorID.Bytes).String(),
		Name:       value.Name,
		Type:       seasonType,
		StartDate:  value.StartDate.Time,
		EndDate:    value.EndDate.Time,
		IsActive:   value.IsActive,
		CreatedAt:  value.CreatedAt.Time,
	}
}
