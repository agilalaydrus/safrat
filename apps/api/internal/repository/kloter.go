package repository

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type KloterRepository struct{ queries *db.Queries }

func NewKloterRepository(queries *db.Queries) *KloterRepository {
	return &KloterRepository{queries: queries}
}

func (r *KloterRepository) ListForOperator(ctx context.Context, operatorID, seasonID string) ([]*domain.Kloter, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListKloters(ctx, db.ListKlotersParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Kloter, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.Kloter{ID: uuidString(row.ID), SeasonID: uuidString(row.SeasonID), OperatorID: uuidString(row.OperatorID), Code: row.Code, Embarkation: row.Embarkation, FlightNumber: row.FlightNumber, DepartureDate: timestamptzPtr(row.DepartureDate), Capacity: row.Capacity, PilgrimCount: row.PilgrimCount, Status: row.Status, Notes: row.Notes})
	}
	return result, nil
}

func (r *KloterRepository) Create(ctx context.Context, operatorID, seasonID, code, embarkation, flightNumber string, departureDate *time.Time, capacity int32) (*domain.Kloter, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	v, err := r.queries.CreateKloter(ctx, db.CreateKloterParams{OperatorID: opUUID, SeasonID: seasonUUID, Code: code, Embarkation: embarkation, FlightNumber: flightNumber, DepartureDate: pgTimestamptzOptional(departureDate), Capacity: capacity})
	if err != nil {
		return nil, err
	}
	return toKloter(v), nil
}

func (r *KloterRepository) Update(ctx context.Context, operatorID, kloterID, code, embarkation, flightNumber string, departureDate *time.Time, capacity int32) (*domain.Kloter, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	v, err := r.queries.UpdateKloter(ctx, db.UpdateKloterParams{ID: kloterUUID, OperatorID: opUUID, Code: code, Embarkation: embarkation, FlightNumber: flightNumber, DepartureDate: pgTimestamptzOptional(departureDate), Capacity: capacity})
	if err != nil {
		return nil, err
	}
	return toKloter(v), nil
}

func (r *KloterRepository) UpdateStatus(ctx context.Context, operatorID, kloterID, status string) (*domain.Kloter, error) {
	return r.updateStatus(ctx, r.queries, operatorID, kloterID, status)
}

func (r *KloterRepository) UpdateStatusTx(ctx context.Context, tx pgx.Tx, operatorID, kloterID, status string) (*domain.Kloter, error) {
	return r.updateStatus(ctx, r.queries.WithTx(tx), operatorID, kloterID, status)
}

func (r *KloterRepository) updateStatus(ctx context.Context, queries *db.Queries, operatorID, kloterID, status string) (*domain.Kloter, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	v, err := queries.UpdateKloterStatus(ctx, db.UpdateKloterStatusParams{ID: kloterUUID, OperatorID: opUUID, Status: status})
	if err != nil {
		return nil, err
	}
	return toKloter(v), nil
}

func (r *KloterRepository) Delete(ctx context.Context, operatorID, kloterID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return err
	}
	if err := r.queries.UnassignKloterPilgrims(ctx, db.UnassignKloterPilgrimsParams{KloterID: kloterUUID, OperatorID: opUUID}); err != nil {
		return err
	}
	return r.queries.DeleteKloter(ctx, db.DeleteKloterParams{ID: kloterUUID, OperatorID: opUUID})
}

// GetForOperator is the multi-tenant boundary check used by PilgrimService
// before trusting a kloter_id from a request body, same pattern as
// GroupRepository.EnsureGroupBelongsToOperator.
func (r *KloterRepository) GetForOperator(ctx context.Context, operatorID, kloterID string) (*domain.Kloter, error) {
	return r.getForOperator(ctx, r.queries, operatorID, kloterID, false)
}

func (r *KloterRepository) GetForOperatorForUpdateTx(ctx context.Context, tx pgx.Tx, operatorID, kloterID string) (*domain.Kloter, error) {
	return r.getForOperator(ctx, r.queries.WithTx(tx), operatorID, kloterID, true)
}

func (r *KloterRepository) getForOperator(ctx context.Context, queries *db.Queries, operatorID, kloterID string, forUpdate bool) (*domain.Kloter, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	var v db.Kloter
	if forUpdate {
		v, err = queries.GetKloterForOperatorForUpdate(ctx, db.GetKloterForOperatorForUpdateParams{ID: kloterUUID, OperatorID: opUUID})
	} else {
		v, err = queries.GetKloterForOperator(ctx, db.GetKloterForOperatorParams{ID: kloterUUID, OperatorID: opUUID})
	}
	if err != nil {
		return nil, err
	}
	return &domain.Kloter{ID: uuidString(v.ID), SeasonID: uuidString(v.SeasonID), OperatorID: uuidString(v.OperatorID), Code: v.Code, Embarkation: v.Embarkation, FlightNumber: v.FlightNumber, DepartureDate: timestamptzPtr(v.DepartureDate), Capacity: v.Capacity, Status: v.Status, Notes: v.Notes}, nil
}

func toKloter(v db.Kloter) *domain.Kloter {
	return &domain.Kloter{ID: uuidString(v.ID), SeasonID: uuidString(v.SeasonID), OperatorID: uuidString(v.OperatorID), Code: v.Code, Embarkation: v.Embarkation, FlightNumber: v.FlightNumber, DepartureDate: timestamptzPtr(v.DepartureDate), Capacity: v.Capacity, Status: v.Status, Notes: v.Notes}
}

func pgTimestamptzOptional(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}
