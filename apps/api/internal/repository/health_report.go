package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type HealthReportRepository struct{ queries *db.Queries }

func NewHealthReportRepository(queries *db.Queries) *HealthReportRepository {
	return &HealthReportRepository{queries: queries}
}

func (r *HealthReportRepository) Create(ctx context.Context, operatorID, pilgrimID, groupID, reportedByUserID, severity, symptoms, actionTaken string) (*domain.HealthReport, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, databaseError(err)
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, databaseError(err)
	}
	return createHealthReport(ctx, r.queries, opUUID, scope, pilgrimID, groupID, reportedByUserID, severity, symptoms, actionTaken)
}

// CreateTx is the transactional variant — used so the report insert and the
// outbox event that fans out its BERAT push commit atomically.
func (r *HealthReportRepository) CreateTx(ctx context.Context, tx pgx.Tx, operatorID, pilgrimID, groupID, reportedByUserID, severity, symptoms, actionTaken string) (*domain.HealthReport, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, databaseError(err)
	}
	// Resolve the actor's branch through the same transaction so a concurrent
	// membership change cannot widen this write after the scope was checked.
	scope, err := branchScope(ctx, r.queries.WithTx(tx), opUUID)
	if err != nil {
		return nil, err
	}
	return createHealthReport(ctx, r.queries.WithTx(tx), opUUID, scope, pilgrimID, groupID, reportedByUserID, severity, symptoms, actionTaken)
}

func createHealthReport(ctx context.Context, q *db.Queries, opUUID, scope pgtype.UUID, pilgrimID, groupID, reportedByUserID, severity, symptoms, actionTaken string) (*domain.HealthReport, error) {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, err
	}
	v, err := q.CreateHealthReport(ctx, db.CreateHealthReportParams{
		OperatorID: opUUID, PilgrimID: pilgrimUUID, GroupID: groupUUID, BranchScope: scope,
		ReportedBy: pgtype.Text{String: reportedByUserID, Valid: reportedByUserID != ""},
		Severity:   db.HealthSeverity(severity), Symptoms: symptoms, ActionTaken: actionTaken,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.HealthReport{ID: uuidString(v.ID), PilgrimID: uuidString(v.PilgrimID), GroupID: uuidString(v.GroupID), Severity: string(v.Severity), Symptoms: v.Symptoms, ActionTaken: v.ActionTaken, Resolved: v.Resolved, ResolvedAt: timestamptzPtr(v.ResolvedAt), CreatedAt: v.CreatedAt.Time}, nil
}

func (r *HealthReportRepository) List(ctx context.Context, operatorID string, resolved *bool) ([]domain.HealthReport, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	param := pgtype.Bool{}
	if resolved != nil {
		param = pgtype.Bool{Bool: *resolved, Valid: true}
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListHealthReports(ctx, db.ListHealthReportsParams{OperatorID: opUUID, Resolved: param, BranchScope: scope})
	if err != nil {
		return nil, err
	}
	result := make([]domain.HealthReport, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.HealthReport{
			ID: uuidString(row.ID), PilgrimID: uuidString(row.PilgrimID), PilgrimName: row.PilgrimName, GroupID: uuidString(row.GroupID), GroupName: row.GroupName,
			Severity: string(row.Severity), Symptoms: row.Symptoms, ActionTaken: row.ActionTaken, Resolved: row.Resolved, ResolvedAt: timestamptzPtr(row.ResolvedAt), CreatedAt: row.CreatedAt.Time,
		})
	}
	return result, nil
}

func (r *HealthReportRepository) Resolve(ctx context.Context, operatorID, reportID string) (*domain.HealthReport, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	reportUUID, err := pgUUID(reportID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	v, err := r.queries.ResolveHealthReport(ctx, db.ResolveHealthReportParams{ID: reportUUID, OperatorID: opUUID, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.HealthReport{ID: uuidString(v.ID), PilgrimID: uuidString(v.PilgrimID), GroupID: uuidString(v.GroupID), Severity: string(v.Severity), Symptoms: v.Symptoms, ActionTaken: v.ActionTaken, Resolved: v.Resolved, ResolvedAt: timestamptzPtr(v.ResolvedAt), CreatedAt: v.CreatedAt.Time}, nil
}

func (r *HealthReportRepository) HasOpenSevere(ctx context.Context, operatorID, pilgrimID string) (bool, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return false, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return false, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return false, err
	}
	return r.queries.HasOpenSevereHealthReport(ctx, db.HasOpenSevereHealthReportParams{PilgrimID: pilgrimUUID, OperatorID: opUUID, BranchScope: scope})
}
