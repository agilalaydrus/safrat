package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
)

type ChecklistRepository struct {
	queries *db.Queries
}

func NewChecklistRepository(queries *db.Queries) *ChecklistRepository {
	return &ChecklistRepository{queries: queries}
}

func (r *ChecklistRepository) CreateTemplate(ctx context.Context, operatorID, seasonID, title, description, category string, isRequired bool, sortOrder int32) (*domain.ChecklistTemplate, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if category == "" {
		category = "DOCUMENT"
	}
	row, err := r.queries.CreateChecklistTemplate(ctx, db.CreateChecklistTemplateParams{
		OperatorID: opUUID, SeasonID: seasonUUID, Title: title, Description: description, Category: category, IsRequired: isRequired, SortOrder: sortOrder,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toChecklistTemplate(row), nil
}

func (r *ChecklistRepository) ListTemplates(ctx context.Context, operatorID, seasonID string) ([]*domain.ChecklistTemplate, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListChecklistTemplates(ctx, db.ListChecklistTemplatesParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.ChecklistTemplate, 0, len(rows))
	for _, row := range rows {
		result = append(result, toChecklistTemplate(row))
	}
	return result, nil
}

func (r *ChecklistRepository) DeleteTemplate(ctx context.Context, operatorID, id string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	return databaseError(r.queries.DeleteChecklistTemplate(ctx, db.DeleteChecklistTemplateParams{ID: idUUID, OperatorID: opUUID}))
}

func (r *ChecklistRepository) UpsertItem(ctx context.Context, templateID, pilgrimID, operatorID string, isCompleted bool, completedBy, notes string) (*domain.ChecklistItem, error) {
	templateUUID, err := pgUUID(templateID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpsertPilgrimChecklistItem(ctx, db.UpsertPilgrimChecklistItemParams{
		TemplateID: templateUUID, PilgrimID: pilgrimUUID, OperatorID: opUUID, IsCompleted: isCompleted, CompletedBy: completedBy, Notes: notes,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	item := &domain.ChecklistItem{TemplateID: uuidString(row.TemplateID), IsCompleted: row.IsCompleted, CompletedBy: row.CompletedBy, Notes: row.Notes}
	if row.CompletedAt.Valid {
		t := row.CompletedAt.Time
		item.CompletedAt = &t
	}
	return item, nil
}

func (r *ChecklistRepository) GetPilgrimChecklist(ctx context.Context, operatorID, seasonID, pilgrimID string) ([]*domain.ChecklistItem, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.GetPilgrimChecklist(ctx, db.GetPilgrimChecklistParams{OperatorID: opUUID, SeasonID: seasonUUID, PilgrimID: pilgrimUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.ChecklistItem, 0, len(rows))
	for _, row := range rows {
		item := &domain.ChecklistItem{
			TemplateID: uuidString(row.TemplateID), Title: row.Title, Description: row.Description, Category: row.Category,
			IsRequired: row.IsRequired, IsCompleted: row.IsCompleted, CompletedBy: row.CompletedBy.String, Notes: row.Notes.String,
		}
		if row.CompletedAt.Valid {
			t := row.CompletedAt.Time
			item.CompletedAt = &t
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *ChecklistRepository) GetStats(ctx context.Context, operatorID, seasonID string) ([]*domain.ChecklistStat, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.GetChecklistCompletionStats(ctx, db.GetChecklistCompletionStatsParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.ChecklistStat, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.ChecklistStat{
			TemplateID: uuidString(row.ID), Title: row.Title, Category: row.Category, IsRequired: row.IsRequired,
			CompletedCount: row.CompletedCount, TotalPilgrims: row.TotalPilgrims,
		})
	}
	return result, nil
}

func toChecklistTemplate(value db.ChecklistTemplate) *domain.ChecklistTemplate {
	return &domain.ChecklistTemplate{
		ID: uuidString(value.ID), OperatorID: uuidString(value.OperatorID), SeasonID: uuidString(value.SeasonID),
		Title: value.Title, Description: value.Description, Category: value.Category, IsRequired: value.IsRequired, SortOrder: value.SortOrder,
	}
}
