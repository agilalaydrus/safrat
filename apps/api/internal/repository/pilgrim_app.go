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
		AgentID:             nullableUUIDString(row.AgentID),
		OperatorID:          uuidString(row.OperatorID),
		KloterID:            nullableUUIDString(row.KloterID),
		KloterCode:          row.KloterCode.String,
		KloterEmbarkation:   row.KloterEmbarkation.String,
		KloterFlightNumber:  row.KloterFlightNumber.String,
		KloterDepartureDate: timestamptzPtr(row.KloterDepartureDate),
		LinkedGoogleEmail:   row.LinkedGoogleEmail.String,
		Phone:               row.Phone.String,
		NIK:                 row.Nik,
		Address:             row.Address,
		KYCStatus:           row.KycStatus,
		KYCRejectionReason:  row.KycRejectionReason,
		PlaceOfBirth:        row.PlaceOfBirth,
		MaritalStatus:       row.MaritalStatus,
		Occupation:          row.Occupation,
		FatherName:          row.FatherName,
		Status:              row.Status,
		PaymentStatus:       row.PaymentStatus,
	}, nil
}

// ListHotelStays returns every hotel this pilgrim has a room in — GetAppInfo's
// HotelName/RoomNumber is just a "most recent" display summary.
func (r *PilgrimRepository) ListHotelStays(ctx context.Context, appAccessCode string) ([]domain.HotelStay, error) {
	rows, err := r.queries.ListPilgrimHotelStays(ctx, appAccessCode)
	if err != nil {
		return nil, err
	}
	stays := make([]domain.HotelStay, 0, len(rows))
	for _, row := range rows {
		stays = append(stays, domain.HotelStay{
			HotelName:  row.HotelName,
			RoomNumber: row.RoomNumber,
			RoomType:   row.RoomType,
		})
	}
	return stays, nil
}

// LinkGoogleAccount ties the pilgrim behind app_access_code to a real
// Better Auth user id. userID must already be server-validated by the
// caller (PilgrimAppService.LinkGoogleAccount, via the session-only —
// not org-scoped — middleware lane) — never accept it from a request body.
func (r *PilgrimRepository) LinkGoogleAccount(ctx context.Context, appAccessCode, userID string) error {
	_, err := r.queries.LinkPilgrimGoogleAccount(ctx, db.LinkPilgrimGoogleAccountParams{
		AppAccessCode: appAccessCode,
		LinkedUserID:  pgtype.Text{String: userID, Valid: true},
	})
	if err != nil {
		return databaseError(err)
	}
	return nil
}

// UserIsOrganizationMember guards LinkGoogleAccount against linking a
// signed-in staff/leader account (an org member) to a pilgrim record.
func (r *PilgrimRepository) UserIsOrganizationMember(ctx context.Context, userID string) (bool, error) {
	return r.queries.UserIsOrganizationMember(ctx, userID)
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
		result = append(result, &Movement{
			ID: uuidString(row.ID), SeasonID: uuidString(row.SeasonID), OperatorID: uuidString(row.OperatorID), Name: row.Name, Origin: row.Origin, Destination: row.Destination,
			ScheduledAt: row.ScheduledAt.Time, Status: row.Status, Mode: row.Mode, KloterID: nullableUUIDString(row.KloterID), CreatedAt: row.CreatedAt.Time,
			Airline: row.Airline, FlightNumber: row.FlightNumber, TripLeg: row.TripLeg,
		})
	}
	return result, nil
}
