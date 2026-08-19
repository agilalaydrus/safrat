package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
)

type StaffScheduleRepository struct {
	queries *db.Queries
}

func NewStaffScheduleRepository(queries *db.Queries) *StaffScheduleRepository {
	return &StaffScheduleRepository{queries: queries}
}

func (r *StaffScheduleRepository) Assign(ctx context.Context, operatorID, kloterID, staffID, staffName, staffEmail, role, duties string) (*domain.KloterStaff, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if role == "" {
		role = "COORDINATOR"
	}
	row, err := r.queries.AssignStaffToKloter(ctx, db.AssignStaffToKloterParams{
		OperatorID: opUUID, KloterID: kloterUUID, StaffID: staffID, StaffName: staffName, StaffEmail: staffEmail, Role: role, Duties: duties,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.KloterStaff{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), KloterID: uuidString(row.KloterID),
		StaffID: row.StaffID, StaffName: row.StaffName, StaffEmail: row.StaffEmail, Role: row.Role, Duties: row.Duties, CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *StaffScheduleRepository) ListForKloter(ctx context.Context, operatorID, kloterID string) ([]*domain.KloterStaff, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListKloterStaff(ctx, db.ListKloterStaffParams{OperatorID: opUUID, KloterID: kloterUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.KloterStaff, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.KloterStaff{
			ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), KloterID: uuidString(row.KloterID), KloterName: row.KloterName,
			StaffID: row.StaffID, StaffName: row.StaffName, StaffEmail: row.StaffEmail, Role: row.Role, Duties: row.Duties,
			DepartureDate: timestamptzPtr(row.DepartureDate), CreatedAt: row.CreatedAt.Time,
		})
	}
	return result, nil
}

func (r *StaffScheduleRepository) ListMine(ctx context.Context, operatorID, staffID string) ([]*domain.KloterStaff, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListMyAssignments(ctx, db.ListMyAssignmentsParams{StaffID: staffID, OperatorID: opUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.KloterStaff, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.KloterStaff{
			ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), KloterID: uuidString(row.KloterID), KloterName: row.KloterName,
			StaffID: row.StaffID, StaffName: row.StaffName, StaffEmail: row.StaffEmail, Role: row.Role, Duties: row.Duties,
			DepartureDate: timestamptzPtr(row.DepartureDate), SeasonName: row.SeasonName, CreatedAt: row.CreatedAt.Time,
		})
	}
	return result, nil
}

func (r *StaffScheduleRepository) ListAll(ctx context.Context, operatorID, seasonID string) ([]*domain.KloterScheduleSummary, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListAllStaffSchedule(ctx, db.ListAllStaffScheduleParams{OperatorID: opUUID, ID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.KloterScheduleSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.KloterScheduleSummary{
			KloterID: uuidString(row.KloterID), KloterName: row.KloterName, SeasonName: row.SeasonName,
			StaffCount: row.StaffCount, StaffNames: stringOrEmpty(row.StaffNames), DepartureDate: timestamptzPtr(row.DepartureDate),
		})
	}
	return result, nil
}

func (r *StaffScheduleRepository) Remove(ctx context.Context, kloterID, staffID, operatorID string) error {
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return apperror.ErrValidation
	}
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	return databaseError(r.queries.RemoveStaffFromKloter(ctx, db.RemoveStaffFromKloterParams{KloterID: kloterUUID, StaffID: staffID, OperatorID: opUUID}))
}
