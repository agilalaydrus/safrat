package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
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

func (r *SeasonRepository) Update(ctx context.Context, operatorID, seasonID, name string, seasonType domain.SeasonType, startDate, endDate time.Time) (*domain.Season, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	season, err := r.queries.UpdateSeason(ctx, db.UpdateSeasonParams{
		ID: seasonUUID, OperatorID: operatorUUID, Name: name,
		Type: databaseSeasonType(seasonType), StartDate: pgTimestamp(startDate), EndDate: pgTimestamp(endDate),
	})
	if err != nil {
		return nil, err
	}
	return toSeason(season), nil
}

// HasData reports whether any pilgrim/group/kloter/hotel/movement/product/
// order row still references this season — every one of those cascades on
// the season FK, so Delete refuses when this is true (see season.sql).
func (r *SeasonRepository) HasData(ctx context.Context, seasonID string) (bool, error) {
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return false, err
	}
	return r.queries.SeasonHasData(ctx, seasonUUID)
}

func (r *SeasonRepository) Delete(ctx context.Context, operatorID, seasonID string) error {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return err
	}
	return r.queries.DeleteSeason(ctx, db.DeleteSeasonParams{ID: seasonUUID, OperatorID: operatorUUID})
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

func (r *SeasonRepository) GetAnalytics(ctx context.Context, operatorID, seasonID string) (*domain.SeasonAnalytics, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetSeasonAnalytics(ctx, db.GetSeasonAnalyticsParams{OperatorID: operatorUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, err
	}
	return &domain.SeasonAnalytics{
		TotalPilgrims:  row.TotalPilgrims,
		PaidCount:      row.PaidCount,
		DPCount:        row.DpCount,
		UnpaidCount:    row.UnpaidCount,
		DocsComplete:   row.DocsComplete,
		CheckedInCount: row.CheckedInCount,
		RoomsAllocated: row.RoomsAllocated,
		SeatsAssigned:  row.SeatsAssigned,
	}, nil
}

func pgUUID(value string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse UUID: %w", err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

// pgText treats an empty string as NULL — used for optional text columns
// (e.g. a Better Auth user id that may be absent).
func pgText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

// nonNilStrings guards a NOT NULL text[] column against pgx sending a nil Go
// slice as SQL NULL — an absent repeated proto field decodes to nil, not an
// empty slice, and that difference matters here even though the two are
// interchangeable everywhere else in Go.
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func pgTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func databaseSeasonType(value domain.SeasonType) db.SeasonType {
	switch value {
	case domain.SeasonTypeUmrahReguler:
		return db.SeasonTypeUMRAHREGULER
	case domain.SeasonTypeUmrahRajab:
		return db.SeasonTypeUMRAHRAJAB
	case domain.SeasonTypeUmrahRamadhan:
		return db.SeasonTypeUMRAHRAMADHAN
	case domain.SeasonTypeUmrahSyawal:
		return db.SeasonTypeUMRAHSYAWAL
	case domain.SeasonTypeUmrahDzulqaidah:
		return db.SeasonTypeUMRAHDZULQAIDAH
	default:
		return db.SeasonTypeHAJJ
	}
}

func domainSeasonType(value db.SeasonType) domain.SeasonType {
	switch value {
	case db.SeasonTypeUMRAHREGULER:
		return domain.SeasonTypeUmrahReguler
	case db.SeasonTypeUMRAHRAJAB:
		return domain.SeasonTypeUmrahRajab
	case db.SeasonTypeUMRAHRAMADHAN:
		return domain.SeasonTypeUmrahRamadhan
	case db.SeasonTypeUMRAHSYAWAL:
		return domain.SeasonTypeUmrahSyawal
	case db.SeasonTypeUMRAHDZULQAIDAH:
		return domain.SeasonTypeUmrahDzulqaidah
	default:
		return domain.SeasonTypeHajj
	}
}

func toSeason(value db.Season) *domain.Season {
	return &domain.Season{
		ID:         uuid.UUID(value.ID.Bytes).String(),
		OperatorID: uuid.UUID(value.OperatorID.Bytes).String(),
		Name:       value.Name,
		Type:       domainSeasonType(value.Type),
		StartDate:  value.StartDate.Time,
		EndDate:    value.EndDate.Time,
		IsActive:   value.IsActive,
		CreatedAt:  value.CreatedAt.Time,
	}
}
