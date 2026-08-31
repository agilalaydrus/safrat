package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type LostReportRepository struct {
	queries *db.Queries
}

func NewLostReportRepository(queries *db.Queries) *LostReportRepository {
	return &LostReportRepository{queries: queries}
}

// Create takes pilgrimID/operatorID/groupID already resolved server-side
// by the caller (from app_access_code) — never from client input.
func (r *LostReportRepository) Create(ctx context.Context, pilgrimID, operatorID, groupID string, latitude, longitude float64, lastKnownLocation string) (*domain.LostReport, error) {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	var groupUUID pgtype.UUID
	if groupID != "" {
		groupUUID, err = pgUUID(groupID)
		if err != nil {
			return nil, apperror.ErrValidation
		}
	}
	row, err := r.queries.CreateLostReport(ctx, db.CreateLostReportParams{
		PilgrimID: pilgrimUUID, OperatorID: opUUID, GroupID: groupUUID, Latitude: latitude, Longitude: longitude, LastKnownLocation: lastKnownLocation,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toLostReport(row), nil
}

func (r *LostReportRepository) Resolve(ctx context.Context, operatorID, id string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return err
	}
	rows, err := r.queries.ResolveLostReport(ctx, db.ResolveLostReportParams{ID: idUUID, OperatorID: opUUID, BranchScope: scope})
	if err != nil {
		return databaseError(err)
	}
	if rows == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// ResolveForGroup scopes the resolve by group_id instead of operator_id —
// the caller (service layer) has already confirmed leader ownership of the
// group via EnsureLeaderOwnsGroup, but this scoping is what actually
// prevents resolving a report outside that group at the DB level. Zero rows
// affected means the report doesn't exist or isn't LOST in this group.
func (r *LostReportRepository) ResolveForGroup(ctx context.Context, operatorID, groupID, id string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return err
	}
	rows, err := r.queries.ResolveGroupLostReport(ctx, db.ResolveGroupLostReportParams{ID: idUUID, GroupID: groupUUID, BranchScope: scope})
	if err != nil {
		return databaseError(err)
	}
	if rows == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *LostReportRepository) ListActive(ctx context.Context, operatorID string) ([]*domain.LostReport, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListActiveLostReports(ctx, db.ListActiveLostReportsParams{OperatorID: opUUID, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.LostReport, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.LostReport{
			ID: uuidString(row.ID), PilgrimID: uuidString(row.PilgrimID), PilgrimName: row.FullName, PilgrimPhone: row.Phone.String,
			OperatorID: uuidString(row.OperatorID), GroupID: nullableUUIDString(row.GroupID), GroupName: row.GroupName.String,
			Latitude: row.Latitude, Longitude: row.Longitude, LastKnownLocation: row.LastKnownLocation, Status: row.Status,
			CreatedAt: row.CreatedAt.Time, ResolvedAt: timestamptzPtr(row.ResolvedAt),
		})
	}
	return result, nil
}

func (r *LostReportRepository) ListForGroup(ctx context.Context, operatorID, groupID string) ([]*domain.LostReport, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListGroupLostReports(ctx, db.ListGroupLostReportsParams{OperatorID: opUUID, GroupID: groupUUID, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.LostReport, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.LostReport{
			ID: uuidString(row.ID), PilgrimID: uuidString(row.PilgrimID), PilgrimName: row.FullName, PilgrimPhone: row.Phone.String,
			OperatorID: uuidString(row.OperatorID), GroupID: nullableUUIDString(row.GroupID),
			Latitude: row.Latitude, Longitude: row.Longitude, LastKnownLocation: row.LastKnownLocation, Status: row.Status,
			CreatedAt: row.CreatedAt.Time, ResolvedAt: timestamptzPtr(row.ResolvedAt),
		})
	}
	return result, nil
}

func toLostReport(value db.LostReport) *domain.LostReport {
	return &domain.LostReport{
		ID: uuidString(value.ID), PilgrimID: uuidString(value.PilgrimID), OperatorID: uuidString(value.OperatorID),
		GroupID: nullableUUIDString(value.GroupID), Latitude: value.Latitude, Longitude: value.Longitude,
		LastKnownLocation: value.LastKnownLocation, Status: value.Status, CreatedAt: value.CreatedAt.Time, ResolvedAt: timestamptzPtr(value.ResolvedAt),
	}
}
