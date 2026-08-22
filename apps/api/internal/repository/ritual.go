package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type RitualRepository struct{ queries *db.Queries }

func NewRitualRepository(queries *db.Queries) *RitualRepository {
	return &RitualRepository{queries: queries}
}

func (r *RitualRepository) ListTemplates(ctx context.Context, operatorID, seasonType string) ([]domain.RitualTemplate, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListRitualTemplates(ctx, db.ListRitualTemplatesParams{OperatorID: opUUID, SeasonType: seasonType})
	if err != nil {
		return nil, err
	}
	result := make([]domain.RitualTemplate, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.RitualTemplate{ID: uuidString(row.ID), SeasonType: row.SeasonType, Name: row.Name, Description: row.Description, OrderNum: row.OrderNum, IsRequired: row.IsRequired})
	}
	return result, nil
}

func (r *RitualRepository) CountTemplates(ctx context.Context, operatorID, seasonType string) (int32, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return 0, err
	}
	return r.queries.CountRitualTemplates(ctx, db.CountRitualTemplatesParams{OperatorID: opUUID, SeasonType: seasonType})
}

func (r *RitualRepository) CreateTemplate(ctx context.Context, operatorID, seasonType, name, description string, orderNum int32, isRequired bool) (*domain.RitualTemplate, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	v, err := r.queries.CreateRitualTemplate(ctx, db.CreateRitualTemplateParams{OperatorID: opUUID, SeasonType: seasonType, Name: name, Description: description, OrderNum: orderNum, IsRequired: isRequired})
	if err != nil {
		return nil, err
	}
	return &domain.RitualTemplate{ID: uuidString(v.ID), SeasonType: v.SeasonType, Name: v.Name, Description: v.Description, OrderNum: v.OrderNum, IsRequired: v.IsRequired}, nil
}

// CompletePilgrimRitual upserts the completion — RitualService.BulkCompleteRitual
// calls this once per pilgrim in the group.
func (r *RitualRepository) CompletePilgrimRitual(ctx context.Context, operatorID, pilgrimID, ritualID, completedByUserID, notes string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return err
	}
	ritualUUID, err := pgUUID(ritualID)
	if err != nil {
		return err
	}
	_, err = r.queries.UpsertPilgrimRitual(ctx, db.UpsertPilgrimRitualParams{
		OperatorID: opUUID, PilgrimID: pilgrimUUID, RitualID: ritualUUID,
		CompletedBy: pgtype.Text{String: completedByUserID, Valid: completedByUserID != ""}, Notes: notes,
	})
	return err
}

func (r *RitualRepository) GetPilgrimStatusByAccessCode(ctx context.Context, appAccessCode string) ([]domain.PilgrimRitualStatus, error) {
	rows, err := r.queries.GetPilgrimRitualStatusByAccessCode(ctx, appAccessCode)
	if err != nil {
		return nil, err
	}
	result := make([]domain.PilgrimRitualStatus, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.PilgrimRitualStatus{
			RitualID: uuidString(row.RitualID), Name: row.Name, Description: row.Description, OrderNum: row.OrderNum,
			IsRequired: row.IsRequired, Completed: row.Completed, CompletedAt: timestamptzPtr(row.CompletedAt), CompletedByName: row.CompletedByName,
		})
	}
	return result, nil
}

func (r *RitualRepository) GetGroupProgress(ctx context.Context, operatorID, groupID string) ([]domain.RitualProgressItem, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.CountRitualCompletionByGroup(ctx, db.CountRitualCompletionByGroupParams{ID: groupUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	result := make([]domain.RitualProgressItem, 0, len(rows))
	for _, row := range rows {
		item := domain.RitualProgressItem{RitualID: uuidString(row.RitualID), Name: row.Name, OrderNum: row.OrderNum, TotalPilgrims: row.TotalPilgrims, CompletedCount: row.CompletedCount}
		if row.CompletedCount < row.TotalPilgrims {
			names, err := r.queries.ListIncompletePilgrimNamesForRitual(ctx, db.ListIncompletePilgrimNamesForRitualParams{GroupID: groupUUID, OperatorID: opUUID, RitualID: row.RitualID})
			if err == nil {
				for _, n := range names {
					item.IncompletePilgrimNames = append(item.IncompletePilgrimNames, n.FullName)
				}
			}
		}
		result = append(result, item)
	}
	return result, nil
}

// SeasonTypeBucket resolves a season's HAJJ/UMRAH bucket (folding every
// UMRAH_* subtype into plain "UMRAH") for matching against ritual_templates.
func (r *RitualRepository) SeasonTypeBucket(ctx context.Context, seasonID string) (string, error) {
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return "", err
	}
	return r.queries.SeasonTypeBucket(ctx, seasonUUID)
}
