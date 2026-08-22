package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
)

type FamilyTrackerRepository struct {
	queries *db.Queries
}

func NewFamilyTrackerRepository(queries *db.Queries) *FamilyTrackerRepository {
	return &FamilyTrackerRepository{queries: queries}
}

// Get returns the public-shape FamilyStatus plus the underlying pilgrim's
// id/operator_id — those two are internal plumbing only (journey status +
// ritual progress lookups in the service layer), never part of the public
// struct itself; see domain.FamilyStatus's privacy note.
func (r *FamilyTrackerRepository) Get(ctx context.Context, appAccessCode string) (status *domain.FamilyStatus, pilgrimID, operatorID string, err error) {
	row, err := r.queries.GetFamilyTrackerInfo(ctx, appAccessCode)
	if err != nil {
		return nil, "", "", databaseError(err)
	}
	status = &domain.FamilyStatus{
		FirstName: row.FirstName, PaymentStatus: row.PaymentStatus, HotelCheckedIn: row.HotelCheckedIn,
		PilgrimStatus: row.PilgrimStatus, SeasonName: row.SeasonName, DepartureDate: row.DepartureDate.Time,
		GroupName: row.GroupName, LeaderName: row.LeaderName, HasActiveSOS: row.HasActiveSos,
	}
	if row.LastLocationAt.Valid {
		t := row.LastLocationAt.Time
		status.LastLocationAt = &t
	}
	return status, uuidString(row.ID), uuidString(row.OperatorID), nil
}
