package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	seasonSlugStripPattern = regexp.MustCompile(`[^a-z0-9]+`)
	seasonSlugTrimHyphens  = regexp.MustCompile(`(^-|-$)`)
)

// seasonSlugBase slugifies the full season name. The full name is what
// distinguishes it, e.g. "Musim Haji 2025" vs "Musim Haji 2026".
func seasonSlugBase(name string) string {
	lowered := seasonSlugStripPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	return seasonSlugTrimHyphens.ReplaceAllString(lowered, "")
}

type SeasonRepository struct {
	queries *db.Queries
}

func NewSeasonRepository(queries *db.Queries) *SeasonRepository {
	return &SeasonRepository{queries: queries}
}

func (r *SeasonRepository) Create(ctx context.Context, operatorID, name string, seasonType domain.SeasonType, startDate, endDate time.Time, capacity int32) (*domain.Season, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	slug, err := r.uniqueSlug(ctx, operatorUUID, name)
	if err != nil {
		return nil, err
	}
	season, err := r.queries.CreateSeason(ctx, db.CreateSeasonParams{
		OperatorID: operatorUUID,
		Name:       name,
		Type:       databaseSeasonType(seasonType),
		StartDate:  pgTimestamp(startDate),
		EndDate:    pgTimestamp(endDate),
		Capacity:   capacity,
		Slug:       pgtype.Text{String: slug, Valid: slug != ""},
	})
	// CreateSeason's ON CONFLICT returns the existing row only when every
	// business value matches. The same name with different details returns no
	// row so callers can direct the user to edit the existing season instead.
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrAlreadyExists
	}
	if err != nil {
		return nil, databaseError(err)
	}
	return toSeason(season), nil
}

// uniqueSlug mirrors OperatorRepository.uniqueSlug — same bounded-retry
// approach, scoped per operator instead of globally.
func (r *SeasonRepository) uniqueSlug(ctx context.Context, operatorUUID pgtype.UUID, name string) (string, error) {
	base := seasonSlugBase(name)
	if base == "" {
		return "", nil
	}
	for attempt := 1; attempt <= 50; attempt++ {
		candidate := base
		if attempt > 1 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		exists, err := r.queries.SeasonSlugExists(ctx, db.SeasonSlugExistsParams{OperatorID: operatorUUID, Slug: pgtype.Text{String: candidate, Valid: true}})
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", nil
}

func (r *SeasonRepository) GetActiveSeasonID(ctx context.Context, operatorID string) (string, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return "", err
	}
	id, err := r.queries.GetActiveSeasonIDForOperator(ctx, operatorUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return uuid.UUID(id.Bytes).String(), nil
}

func (r *SeasonRepository) GetBySlug(ctx context.Context, operatorID, slug string) (*domain.Season, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	season, err := r.queries.GetSeasonBySlug(ctx, db.GetSeasonBySlugParams{OperatorID: operatorUUID, Slug: pgtype.Text{String: slug, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toSeason(season), nil
}

func (r *SeasonRepository) GetByID(ctx context.Context, operatorID, seasonID string) (*domain.Season, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	season, err := r.queries.GetSeasonByID(ctx, db.GetSeasonByIDParams{ID: seasonUUID, OperatorID: operatorUUID})
	if err != nil {
		return nil, databaseError(err)
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

// ListPublicSeasons returns the not-yet-ended seasons for an operator's
// public profile page, with each season's registered-pilgrim count.
func (r *SeasonRepository) ListPublicSeasons(ctx context.Context, operatorID string) ([]*domain.PublicSeasonSummary, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListPublicSeasonsByOperator(ctx, operatorUUID)
	if err != nil {
		return nil, err
	}
	results := make([]*domain.PublicSeasonSummary, 0, len(rows))
	for _, row := range rows {
		results = append(results, &domain.PublicSeasonSummary{
			ID:           uuid.UUID(row.ID.Bytes).String(),
			Name:         row.Name,
			Slug:         row.Slug.String,
			Type:         domain.SeasonType(row.Type),
			StartDate:    row.StartDate.Time,
			EndDate:      row.EndDate.Time,
			PilgrimCount: int32(row.PilgrimCount),
		})
	}
	return results, nil
}

func (r *SeasonRepository) Update(ctx context.Context, operatorID, seasonID, name string, seasonType domain.SeasonType, startDate, endDate time.Time, capacity int32) (*domain.Season, error) {
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
		Type: databaseSeasonType(seasonType), StartDate: pgTimestamp(startDate), EndDate: pgTimestamp(endDate), Capacity: capacity,
	})
	if err != nil {
		return nil, databaseError(err)
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
	branchID, err := branchScope(ctx, r.queries, operatorUUID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetSeasonAnalytics(ctx, db.GetSeasonAnalyticsParams{OperatorID: operatorUUID, SeasonID: seasonUUID, BranchScope: branchID})
	if err != nil {
		return nil, err
	}
	orderStats, err := r.queries.GetSeasonOrderStats(ctx, db.GetSeasonOrderStatsParams{OperatorID: operatorUUID, SeasonID: seasonUUID, BranchScope: branchID})
	if err != nil {
		return nil, err
	}
	return &domain.SeasonAnalytics{
		TotalPilgrims: row.TotalPilgrims, PaidCount: row.PaidCount, DPCount: row.DpCount, UnpaidCount: row.UnpaidCount,
		DocsComplete: row.DocsComplete, CheckedInCount: row.CheckedInCount, RoomsAllocated: row.RoomsAllocated, SeatsAssigned: row.SeatsAssigned,
		WheelchairCount: row.WheelchairCount, UnassignedGroupCount: row.UnassignedGroupCount, UnassignedKloterCount: row.UnassignedKloterCount,
		OrderCount: orderStats.OrderCount, TotalRevenueIDR: orderStats.TotalRevenueIdr,
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
		Capacity:   value.Capacity,
		Slug:       value.Slug.String,
	}
}
