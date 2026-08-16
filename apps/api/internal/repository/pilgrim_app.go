package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetAppInfo is the public Pilgrim App's entry point — looked up by
// app_access_code, not operator-scoped, since the caller has no session.
func (r *PilgrimRepository) GetAppInfo(ctx context.Context, appAccessCode string) (*domain.PilgrimAppInfo, error) {
	row, err := r.queries.GetPilgrimAppInfo(ctx, appAccessCode)
	if err != nil {
		return nil, err
	}
	return &domain.PilgrimAppInfo{
		ID:                  uuidString(row.ID),
		FullName:            row.FullName,
		PassportNumber:      row.PassportNumber,
		GroupName:           row.GroupName.String,
		HotelName:           row.HotelName.String,
		RoomNumber:          row.RoomNumber.String,
		RequiresWheelchair:  row.RequiresWheelchair,
		SeasonID:            uuidString(row.SeasonID),
		OperatorID:          uuidString(row.OperatorID),
		KloterID:            nullableUUIDString(row.KloterID),
		KloterCode:          row.KloterCode.String,
		KloterEmbarkation:   row.KloterEmbarkation.String,
		KloterFlightNumber:  row.KloterFlightNumber.String,
		KloterDepartureDate: timestamptzPtr(row.KloterDepartureDate),
	}, nil
}

func (r *PilgrimRepository) GetByAppAccessCode(ctx context.Context, appAccessCode string) (*domain.Pilgrim, error) {
	pilgrim, err := r.queries.GetPilgrimByAppAccessCode(ctx, appAccessCode)
	if err != nil {
		return nil, err
	}
	return toPilgrim(pilgrim), nil
}

// UpdateLocation is called every 5 minutes by the pilgrim PWA — a fire-and-
// forget GPS ping, not operator-scoped since the caller has no session, same
// as GetAppInfo.
func (r *PilgrimRepository) UpdateLocation(ctx context.Context, appAccessCode string, lat, lng float64) error {
	return r.queries.UpdatePilgrimLocation(ctx, db.UpdatePilgrimLocationParams{
		AppAccessCode: appAccessCode,
		LastLat:       pgtype.Float8{Float64: lat, Valid: true},
		LastLng:       pgtype.Float8{Float64: lng, Valid: true},
	})
}

// SetWheelchairRequest lets a pilgrim self-report a wheelchair need — public,
// same as UpdateLocation. Returns the pilgrim's id/operatorID/name so the
// caller can write an audit log entry without a second lookup.
func (r *PilgrimRepository) SetWheelchairRequest(ctx context.Context, appAccessCode string, requiresWheelchair bool) (pilgrimID, operatorID, fullName string, err error) {
	row, err := r.queries.SetPilgrimWheelchairRequest(ctx, db.SetPilgrimWheelchairRequestParams{AppAccessCode: appAccessCode, RequiresWheelchair: requiresWheelchair})
	if err != nil {
		return "", "", "", err
	}
	return uuidString(row.ID), uuidString(row.OperatorID), row.FullName, nil
}

// ListUpcomingMovements is called with the pilgrim's own kloterID (may be
// empty if unassigned) so a pilgrim only ever sees movements scoped to
// their own kloter, plus any unscoped/shared ones — never another kloter's.
func (r *PilgrimRepository) ListUpcomingMovements(ctx context.Context, operatorID, seasonID, kloterID string) ([]*Movement, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	var kloterUUID pgtype.UUID
	if kloterID != "" {
		kloterUUID, err = pgUUID(kloterID)
		if err != nil {
			return nil, err
		}
	}
	rows, err := r.queries.ListUpcomingMovementsForSeason(ctx, db.ListUpcomingMovementsForSeasonParams{SeasonID: seasonUUID, OperatorID: opUUID, KloterID: kloterUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*Movement, 0, len(rows))
	for _, row := range rows {
		result = append(result, &Movement{ID: uuidString(row.ID), SeasonID: uuidString(row.SeasonID), OperatorID: uuidString(row.OperatorID), Name: row.Name, Origin: row.Origin, Destination: row.Destination, ScheduledAt: row.ScheduledAt.Time, Status: row.Status, Mode: row.Mode, KloterID: nullableUUIDString(row.KloterID), CreatedAt: row.CreatedAt.Time})
	}
	return result, nil
}
