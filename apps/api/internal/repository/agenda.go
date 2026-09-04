package repository

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type AgendaRepository struct {
	queries *db.Queries
}

func NewAgendaRepository(queries *db.Queries) *AgendaRepository {
	return &AgendaRepository{queries: queries}
}

func optionalTimestamptz(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func toAgendaEvent(row db.AgendaEvent) *domain.AgendaEvent {
	return &domain.AgendaEvent{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID),
		BranchID: nullableUUIDString(row.BranchID), SeasonID: nullableUUIDString(row.SeasonID),
		Title: row.Title, Location: row.Location, StartsAt: row.StartsAt.Time, EndsAt: row.EndsAt.Time,
		Notes: row.Notes, CreatedAt: row.CreatedAt.Time,
	}
}

func (r *AgendaRepository) CreateEvent(ctx context.Context, operatorID, branchID, seasonID, title, location string, startsAt, endsAt time.Time, notes string) (*domain.AgendaEvent, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateAgendaEvent(ctx, db.CreateAgendaEventParams{
		OperatorID: op, BranchID: pgUUIDOrNull(branchID), SeasonID: pgUUIDOrNull(seasonID),
		Title: title, Location: location, StartsAt: optionalTimestamptz(startsAt), EndsAt: optionalTimestamptz(endsAt), Notes: notes,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toAgendaEvent(row), nil
}

func (r *AgendaRepository) UpdateEvent(ctx context.Context, operatorID, id, branchID, seasonID, title, location string, startsAt, endsAt time.Time, notes string) (*domain.AgendaEvent, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	eventID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpdateAgendaEvent(ctx, db.UpdateAgendaEventParams{
		ID: eventID, OperatorID: op, BranchID: pgUUIDOrNull(branchID), SeasonID: pgUUIDOrNull(seasonID),
		Title: title, Location: location, StartsAt: optionalTimestamptz(startsAt), EndsAt: optionalTimestamptz(endsAt), Notes: notes,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toAgendaEvent(row), nil
}

func (r *AgendaRepository) DeleteEvent(ctx context.Context, operatorID, id string) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	eventID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	if err := r.queries.DeleteAgendaEvent(ctx, db.DeleteAgendaEventParams{ID: eventID, OperatorID: op}); err != nil {
		return databaseError(err)
	}
	return nil
}

// ListAgenda merges the three sources of the combined calendar: internal
// events, manasik sessions, and kloter departures/returns. branchID empty
// means no branch filter — every internal event shows, head office and every
// branch alike; manasik and kloter items always show regardless, since
// neither is ever branch-owned.
func (r *AgendaRepository) ListAgenda(ctx context.Context, operatorID, seasonID, branchID string) ([]*domain.AgendaItem, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}

	var branchFilter pgtype.UUID
	if branchID != "" {
		branchFilter, err = pgUUID(branchID)
		if err != nil {
			return nil, apperror.ErrValidation
		}
	}

	events, err := r.queries.ListAgendaEvents(ctx, db.ListAgendaEventsParams{OperatorID: op, SeasonID: season, BranchID: branchFilter})
	if err != nil {
		return nil, databaseError(err)
	}
	sessions, err := r.queries.ListAgendaManasikSessions(ctx, db.ListAgendaManasikSessionsParams{OperatorID: op, SeasonID: season})
	if err != nil {
		return nil, databaseError(err)
	}
	movements, err := r.queries.ListAgendaKloterMovements(ctx, db.ListAgendaKloterMovementsParams{OperatorID: op, SeasonID: season})
	if err != nil {
		return nil, databaseError(err)
	}

	items := make([]*domain.AgendaItem, 0, len(events)+len(sessions)+len(movements))
	for _, e := range events {
		items = append(items, &domain.AgendaItem{
			ID: uuidString(e.ID), Kind: "INTERNAL", Title: e.Title, Location: e.Location,
			StartsAt: e.StartsAt.Time, EndsAt: e.EndsAt.Time, Notes: e.Notes,
			BranchID: nullableUUIDString(e.BranchID), BranchName: e.BranchName,
		})
	}
	for _, s := range sessions {
		item := &domain.AgendaItem{
			ID: uuidString(s.ID), Kind: "MANASIK", Title: s.Title, Location: s.Location,
			StartsAt: s.ScheduledAt.Time, KloterCode: s.KloterCode,
		}
		if s.DurationMinutes > 0 {
			item.EndsAt = s.ScheduledAt.Time.Add(time.Duration(s.DurationMinutes) * time.Minute)
		}
		items = append(items, item)
	}
	for _, m := range movements {
		title := m.MovementName
		if m.Origin != "" && m.Destination != "" {
			title = m.Origin + " → " + m.Destination
		}
		items = append(items, &domain.AgendaItem{
			ID: uuidString(m.KloterID) + ":" + m.Leg, Kind: m.Leg, Title: title,
			StartsAt: m.ScheduledAt.Time, KloterID: uuidString(m.KloterID), KloterCode: m.KloterCode,
		})
	}
	return items, nil
}
