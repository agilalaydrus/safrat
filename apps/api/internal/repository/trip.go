package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

// TripRepository is the kloter-scoped counterpart to GroupLeaderRepository —
// every method here answers "what does THIS kloter_staff assignment let me
// see", never a full season/operator view. EnsureStaffAssignedToKloter must
// be called before any other method, mirroring EnsureLeaderOwnsGroup.
type TripRepository struct {
	queries *db.Queries
}

func NewTripRepository(queries *db.Queries) *TripRepository {
	return &TripRepository{queries: queries}
}

func (r *TripRepository) EnsureStaffAssignedToKloter(ctx context.Context, operatorID, kloterID, staffID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return err
	}
	_, err = r.queries.GetKloterStaffAssignment(ctx, db.GetKloterStaffAssignmentParams{KloterID: kloterUUID, OperatorID: opUUID, StaffID: staffID})
	return err
}

func (r *TripRepository) ListRoster(ctx context.Context, operatorID, kloterID string) ([]*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListPilgrimsForKloter(ctx, db.ListPilgrimsForKloterParams{OperatorID: opUUID, KloterID: kloterUUID, BranchScope: scope})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Pilgrim, 0, len(rows))
	for _, row := range rows {
		result = append(result, toPilgrim(row))
	}
	return result, nil
}

func (r *TripRepository) ListActiveSOSAlerts(ctx context.Context, operatorID, kloterID string) ([]*domain.SOSAlert, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListActiveSOSAlertsForKloter(ctx, db.ListActiveSOSAlertsForKloterParams{OperatorID: opUUID, KloterID: kloterUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.SOSAlert, 0, len(rows))
	for _, row := range rows {
		alert := db.SosAlert{ID: row.ID, OperatorID: row.OperatorID, PilgrimID: row.PilgrimID, Status: row.Status, AcknowledgedBy: row.AcknowledgedBy, AcknowledgedAt: row.AcknowledgedAt, ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt, Notes: row.Notes, CreatedAt: row.CreatedAt, Lat: row.Lat, Lng: row.Lng}
		result = append(result, toSOSAlert(alert, row.PilgrimName))
	}
	return result, nil
}

func (r *TripRepository) ListMovements(ctx context.Context, operatorID, kloterID string) ([]*Movement, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListMovementsForKloter(ctx, db.ListMovementsForKloterParams{OperatorID: opUUID, KloterID: kloterUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*Movement, 0, len(rows))
	for _, v := range rows {
		result = append(result, &Movement{
			ID: uuidString(v.ID), SeasonID: uuidString(v.SeasonID), OperatorID: uuidString(v.OperatorID), Name: v.Name, Origin: v.Origin, Destination: v.Destination,
			ScheduledAt: v.ScheduledAt.Time, Status: v.Status, Mode: v.Mode, KloterID: nullableUUIDString(v.KloterID), CreatedAt: v.CreatedAt.Time,
			VehicleCount: v.VehicleCount, TotalCapacity: v.TotalCapacity, AssignedCount: v.AssignedCount,
			Airline: v.Airline, FlightNumber: v.FlightNumber, TripLeg: v.TripLeg,
		})
	}
	return result, nil
}
