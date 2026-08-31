package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

type MonitoringRepository struct{ queries *db.Queries }

func NewMonitoringRepository(queries *db.Queries) *MonitoringRepository {
	return &MonitoringRepository{queries: queries}
}

func (r *MonitoringRepository) ListActiveSOS(ctx context.Context, operatorID, seasonID string) ([]domain.SOSAlertMini, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListActiveSOSForSeason(ctx, db.ListActiveSOSForSeasonParams{OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: scope})
	if err != nil {
		return nil, err
	}
	result := make([]domain.SOSAlertMini, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.SOSAlertMini{ID: uuidString(row.ID), PilgrimName: row.PilgrimName, GroupID: nullableUUIDString(row.GroupID), GroupName: row.GroupName.String, Status: row.Status, CreatedAt: row.CreatedAt.Time})
	}
	return result, nil
}

func (r *MonitoringRepository) ListOpenHealthReports(ctx context.Context, operatorID, seasonID string) ([]domain.HealthReportMini, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListOpenHealthReportsForSeason(ctx, db.ListOpenHealthReportsForSeasonParams{OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: scope})
	if err != nil {
		return nil, err
	}
	result := make([]domain.HealthReportMini, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.HealthReportMini{ID: uuidString(row.ID), PilgrimName: row.PilgrimName, GroupName: row.GroupName, Severity: string(row.Severity), Symptoms: row.Symptoms, CreatedAt: row.CreatedAt.Time})
	}
	return result, nil
}

func (r *MonitoringRepository) ListGroupRitualProgress(ctx context.Context, operatorID, seasonID string) (map[string]domain.GroupRitualProgress, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListGroupRitualProgressForSeason(ctx, db.ListGroupRitualProgressForSeasonParams{OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: scope})
	if err != nil {
		return nil, err
	}
	result := make(map[string]domain.GroupRitualProgress, len(rows))
	for _, row := range rows {
		id := uuidString(row.GroupID)
		result[id] = domain.GroupRitualProgress{GroupID: id, TemplateCount: row.TemplateCount, PilgrimCount: row.PilgrimCount, CompletedCount: row.CompletedCount}
	}
	return result, nil
}

func (r *MonitoringRepository) ListReturnTimeline(ctx context.Context, operatorID, seasonID string) ([]domain.ReturnTimelineItem, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListReturnTimelineForSeason(ctx, db.ListReturnTimelineForSeasonParams{OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: scope})
	if err != nil {
		return nil, err
	}
	result := make([]domain.ReturnTimelineItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.ReturnTimelineItem{KloterID: uuidString(row.KloterID), KloterCode: row.KloterCode, ReturnAt: row.ReturnAt.Time, TotalPilgrims: row.TotalPilgrims, ReadyCount: row.ReadyCount})
	}
	return result, nil
}
