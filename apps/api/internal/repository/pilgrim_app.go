package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

// GetAppInfo is the public Pilgrim App's entry point — looked up by
// app_access_code, not operator-scoped, since the caller has no session.
func (r *PilgrimRepository) GetAppInfo(ctx context.Context, appAccessCode string) (*domain.PilgrimAppInfo, error) {
	row, err := r.queries.GetPilgrimAppInfo(ctx, appAccessCode)
	if err != nil {
		return nil, err
	}
	return &domain.PilgrimAppInfo{
		ID:                 uuidString(row.ID),
		FullName:           row.FullName,
		PassportNumber:     row.PassportNumber,
		GroupName:          row.GroupName.String,
		HotelName:          row.HotelName.String,
		RoomNumber:         row.RoomNumber.String,
		RequiresWheelchair: row.RequiresWheelchair,
		SeasonID:           uuidString(row.SeasonID),
		OperatorID:         uuidString(row.OperatorID),
	}, nil
}

func (r *PilgrimRepository) GetByAppAccessCode(ctx context.Context, appAccessCode string) (*domain.Pilgrim, error) {
	pilgrim, err := r.queries.GetPilgrimByAppAccessCode(ctx, appAccessCode)
	if err != nil {
		return nil, err
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) ListUpcomingMovements(ctx context.Context, operatorID, seasonID string) ([]*Movement, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListUpcomingMovementsForSeason(ctx, db.ListUpcomingMovementsForSeasonParams{SeasonID: seasonUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*Movement, 0, len(rows))
	for _, row := range rows {
		result = append(result, &Movement{ID: uuidString(row.ID), SeasonID: uuidString(row.SeasonID), OperatorID: uuidString(row.OperatorID), Name: row.Name, Origin: row.Origin, Destination: row.Destination, ScheduledAt: row.ScheduledAt.Time, Status: row.Status, CreatedAt: row.CreatedAt.Time})
	}
	return result, nil
}
