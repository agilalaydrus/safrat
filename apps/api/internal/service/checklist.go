package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChecklistService struct {
	operatorRepository  *repository.OperatorRepository
	pilgrimRepository   *repository.PilgrimRepository
	checklistRepository *repository.ChecklistRepository
}

func NewChecklistService(operators *repository.OperatorRepository, pilgrims *repository.PilgrimRepository, checklist *repository.ChecklistRepository) *ChecklistService {
	return &ChecklistService{operatorRepository: operators, pilgrimRepository: pilgrims, checklistRepository: checklist}
}

func (s *ChecklistService) CreateTemplate(ctx context.Context, authenticatedOrgID string, req *hajjv1.CreateChecklistTemplateRequest) (*hajjv1.ChecklistTemplate, error) {
	if req == nil || !isUUID(req.SeasonId) || req.Title == "" {
		return nil, serviceError("ChecklistService.CreateTemplate", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ChecklistService.CreateTemplate", err)
	}
	template, err := s.checklistRepository.CreateTemplate(ctx, operator.ID, req.SeasonId, req.Title, req.Description, req.Category, req.IsRequired, req.SortOrder)
	if err != nil {
		return nil, serviceError("ChecklistService.CreateTemplate", err)
	}
	return checklistTemplateMessage(template), nil
}

func (s *ChecklistService) ListTemplates(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListChecklistTemplatesRequest) (*hajjv1.ListChecklistTemplatesResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("ChecklistService.ListTemplates", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ChecklistService.ListTemplates", err)
	}
	templates, err := s.checklistRepository.ListTemplates(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("ChecklistService.ListTemplates", err)
	}
	result := &hajjv1.ListChecklistTemplatesResponse{Templates: make([]*hajjv1.ChecklistTemplate, 0, len(templates))}
	for _, template := range templates {
		result.Templates = append(result.Templates, checklistTemplateMessage(template))
	}
	return result, nil
}

func (s *ChecklistService) DeleteTemplate(ctx context.Context, authenticatedOrgID string, req *hajjv1.DeleteChecklistTemplateRequest) (*hajjv1.DeleteChecklistTemplateResponse, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("ChecklistService.DeleteTemplate", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ChecklistService.DeleteTemplate", err)
	}
	if err := s.checklistRepository.DeleteTemplate(ctx, operator.ID, req.Id); err != nil {
		return nil, serviceError("ChecklistService.DeleteTemplate", err)
	}
	return &hajjv1.DeleteChecklistTemplateResponse{}, nil
}

func (s *ChecklistService) GetPilgrimChecklist(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetPilgrimChecklistRequest) (*hajjv1.GetPilgrimChecklistResponse, error) {
	if req == nil || !isUUID(req.PilgrimId) || !isUUID(req.SeasonId) {
		return nil, serviceError("ChecklistService.GetPilgrimChecklist", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ChecklistService.GetPilgrimChecklist", err)
	}
	items, err := s.checklistRepository.GetPilgrimChecklist(ctx, operator.ID, req.SeasonId, req.PilgrimId)
	if err != nil {
		return nil, serviceError("ChecklistService.GetPilgrimChecklist", err)
	}
	result := &hajjv1.GetPilgrimChecklistResponse{Items: make([]*hajjv1.ChecklistItem, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, checklistItemMessage(item))
	}
	return result, nil
}

func (s *ChecklistService) UpdateChecklistItem(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdateChecklistItemRequest) (*hajjv1.ChecklistItem, error) {
	if req == nil || !isUUID(req.TemplateId) || !isUUID(req.PilgrimId) {
		return nil, serviceError("ChecklistService.UpdateChecklistItem", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ChecklistService.UpdateChecklistItem", err)
	}
	item, err := s.checklistRepository.UpsertItem(ctx, req.TemplateId, req.PilgrimId, operator.ID, req.IsCompleted, "operator", req.Notes)
	if err != nil {
		return nil, serviceError("ChecklistService.UpdateChecklistItem", err)
	}
	return checklistItemMessage(item), nil
}

func (s *ChecklistService) GetStats(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetChecklistStatsRequest) (*hajjv1.GetChecklistStatsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("ChecklistService.GetStats", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("ChecklistService.GetStats", err)
	}
	stats, err := s.checklistRepository.GetStats(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("ChecklistService.GetStats", err)
	}
	result := &hajjv1.GetChecklistStatsResponse{Stats: make([]*hajjv1.ChecklistStat, 0, len(stats))}
	for _, stat := range stats {
		result.Stats = append(result.Stats, &hajjv1.ChecklistStat{
			TemplateId: stat.TemplateID, Title: stat.Title, Category: stat.Category, IsRequired: stat.IsRequired,
			CompletedCount: int32(stat.CompletedCount), TotalPilgrims: int32(stat.TotalPilgrims),
		})
	}
	return result, nil
}

// GetMyChecklist is public — app_access_code is the only identity token;
// pilgrim_id/season_id/operator_id are always resolved server-side from it.
func (s *ChecklistService) GetMyChecklist(ctx context.Context, req *hajjv1.GetMyChecklistRequest) (*hajjv1.GetMyChecklistResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("ChecklistService.GetMyChecklist", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("ChecklistService.GetMyChecklist", apperror.ErrNotFound)
	}
	items, err := s.checklistRepository.GetPilgrimChecklist(ctx, info.OperatorID, info.SeasonID, info.ID)
	if err != nil {
		return nil, serviceError("ChecklistService.GetMyChecklist", err)
	}
	result := &hajjv1.GetMyChecklistResponse{Items: make([]*hajjv1.ChecklistItem, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, checklistItemMessage(item))
	}
	return result, nil
}

func (s *ChecklistService) CompleteMyChecklistItem(ctx context.Context, req *hajjv1.CompleteMyChecklistItemRequest) (*hajjv1.ChecklistItem, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" || !isUUID(req.TemplateId) {
		return nil, serviceError("ChecklistService.CompleteMyChecklistItem", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("ChecklistService.CompleteMyChecklistItem", apperror.ErrNotFound)
	}
	item, err := s.checklistRepository.UpsertItem(ctx, req.TemplateId, info.ID, info.OperatorID, req.IsCompleted, "pilgrim", req.Notes)
	if err != nil {
		return nil, serviceError("ChecklistService.CompleteMyChecklistItem", err)
	}
	return checklistItemMessage(item), nil
}

func checklistTemplateMessage(value *domain.ChecklistTemplate) *hajjv1.ChecklistTemplate {
	return &hajjv1.ChecklistTemplate{
		Id: value.ID, Title: value.Title, Description: value.Description, Category: value.Category,
		IsRequired: value.IsRequired, SortOrder: value.SortOrder,
	}
}

func checklistItemMessage(value *domain.ChecklistItem) *hajjv1.ChecklistItem {
	item := &hajjv1.ChecklistItem{
		TemplateId: value.TemplateID, Title: value.Title, Description: value.Description, Category: value.Category,
		IsRequired: value.IsRequired, IsCompleted: value.IsCompleted, CompletedBy: value.CompletedBy, Notes: value.Notes,
	}
	if value.CompletedAt != nil {
		item.CompletedAt = timestamppb.New(*value.CompletedAt)
	}
	return item
}
