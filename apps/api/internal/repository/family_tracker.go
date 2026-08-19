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

func (r *FamilyTrackerRepository) Get(ctx context.Context, appAccessCode string) (*domain.FamilyStatus, error) {
	row, err := r.queries.GetFamilyTrackerInfo(ctx, appAccessCode)
	if err != nil {
		return nil, databaseError(err)
	}
	status := &domain.FamilyStatus{
		FirstName: row.FirstName, PaymentStatus: row.PaymentStatus, HotelCheckedIn: row.HotelCheckedIn,
		PilgrimStatus: row.PilgrimStatus, SeasonName: row.SeasonName, DepartureDate: row.DepartureDate.Time,
		GroupName: row.GroupName, LeaderName: row.LeaderName, HasActiveSOS: row.HasActiveSos,
	}
	if row.LastLocationAt.Valid {
		t := row.LastLocationAt.Time
		status.LastLocationAt = &t
	}
	return status, nil
}
