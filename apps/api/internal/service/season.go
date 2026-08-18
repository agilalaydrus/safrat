package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SeasonService struct {
	operatorRepository *repository.OperatorRepository
	seasonRepository   *repository.SeasonRepository
	auditRepository    *repository.AuditRepository
}

func NewSeasonService(operatorRepository *repository.OperatorRepository, seasonRepository *repository.SeasonRepository, auditRepository *repository.AuditRepository) *SeasonService {
	return &SeasonService{operatorRepository: operatorRepository, seasonRepository: seasonRepository, auditRepository: auditRepository}
}

func (s *SeasonService) Create(ctx context.Context, authenticatedOrgID string, request *hajjv1.CreateSeasonRequest) (*hajjv1.Season, error) {
	if request == nil || request.StartDate == nil || request.EndDate == nil || request.StartDate.AsTime().After(request.EndDate.AsTime()) {
		return nil, serviceError("SeasonService.Create", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("SeasonService.Create", err)
	}
	season, err := s.seasonRepository.Create(ctx, operator.ID, request.Name, seasonType(request.Type), request.StartDate.AsTime(), request.EndDate.AsTime())
	if err != nil {
		return nil, serviceError("SeasonService.Create", err)
	}
	return seasonMessage(season), nil
}

func (s *SeasonService) List(ctx context.Context, authenticatedOrgID string) (*hajjv1.ListSeasonsResponse, error) {
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("SeasonService.List", err)
	}
	seasons, err := s.seasonRepository.ListByOperatorID(ctx, operator.ID)
	if err != nil {
		return nil, serviceError("SeasonService.List", err)
	}
	response := &hajjv1.ListSeasonsResponse{Seasons: make([]*hajjv1.Season, 0, len(seasons))}
	for _, season := range seasons {
		response.Seasons = append(response.Seasons, seasonMessage(season))
	}
	return response, nil
}

func (s *SeasonService) Update(ctx context.Context, authenticatedOrgID string, request *hajjv1.UpdateSeasonRequest) (*hajjv1.Season, error) {
	if request == nil || !isUUID(request.SeasonId) || request.StartDate == nil || request.EndDate == nil || request.StartDate.AsTime().After(request.EndDate.AsTime()) {
		return nil, serviceError("SeasonService.Update", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("SeasonService.Update", err)
	}
	season, err := s.seasonRepository.Update(ctx, operator.ID, request.SeasonId, request.Name, seasonType(request.Type), request.StartDate.AsTime(), request.EndDate.AsTime())
	if err != nil {
		return nil, serviceError("SeasonService.Update", err)
	}
	_ = s.auditRepository.Write(ctx, operator.ID, middleware.UserIDFromCtx(ctx), "season_updated", "season", season.ID, season.Name)
	return seasonMessage(season), nil
}

// Delete refuses two ways a season could be destroyed by mistake: while
// it's still the active season (an operator always needs an active season
// to land on), or while any pilgrim/group/kloter/hotel/movement/product/
// order still references it — every one of those cascades on the season
// FK, so an unguarded delete would silently wipe that whole history.
func (s *SeasonService) Delete(ctx context.Context, authenticatedOrgID string, request *hajjv1.DeleteSeasonRequest) (*hajjv1.DeleteSeasonResponse, error) {
	if request == nil || !isUUID(request.SeasonId) {
		return nil, serviceError("SeasonService.Delete", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("SeasonService.Delete", err)
	}
	seasons, err := s.seasonRepository.ListByOperatorID(ctx, operator.ID)
	if err != nil {
		return nil, serviceError("SeasonService.Delete", err)
	}
	var target *domain.Season
	for _, season := range seasons {
		if season.ID == request.SeasonId {
			target = season
			break
		}
	}
	if target == nil {
		return nil, serviceError("SeasonService.Delete", apperror.ErrNotFound)
	}
	if target.IsActive {
		return nil, serviceError("SeasonService.Delete", preconditionError("cannot delete the active season — set another season active first"))
	}
	hasData, err := s.seasonRepository.HasData(ctx, request.SeasonId)
	if err != nil {
		return nil, serviceError("SeasonService.Delete", err)
	}
	if hasData {
		return nil, serviceError("SeasonService.Delete", preconditionError("season still has pilgrims, groups, products, or orders — remove them first"))
	}
	if err := s.seasonRepository.Delete(ctx, operator.ID, request.SeasonId); err != nil {
		return nil, serviceError("SeasonService.Delete", err)
	}
	_ = s.auditRepository.Write(ctx, operator.ID, middleware.UserIDFromCtx(ctx), "season_deleted", "season", target.ID, fmt.Sprintf("deleted empty season %q", target.Name))
	return &hajjv1.DeleteSeasonResponse{}, nil
}

func (s *SeasonService) SetActive(ctx context.Context, authenticatedOrgID string, request *hajjv1.SetActiveSeasonRequest) (*hajjv1.Season, error) {
	if request == nil {
		return nil, serviceError("SeasonService.SetActive", apperror.ErrValidation)
	}
	if _, err := uuid.Parse(request.SeasonId); err != nil {
		return nil, serviceError("SeasonService.SetActive", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("SeasonService.SetActive", err)
	}
	seasons, err := s.seasonRepository.SetActive(ctx, operator.ID, request.SeasonId)
	if err != nil {
		return nil, serviceError("SeasonService.SetActive", err)
	}
	for _, season := range seasons {
		if season.IsActive {
			return seasonMessage(season), nil
		}
	}
	return nil, serviceError("SeasonService.SetActive", apperror.ErrNotFound)
}

func seasonType(value hajjv1.SeasonType) domain.SeasonType {
	if value == hajjv1.SeasonType_SEASON_TYPE_UMRAH {
		return domain.SeasonTypeUmrah
	}
	return domain.SeasonTypeHajj
}

func seasonMessage(value *domain.Season) *hajjv1.Season {
	seasonType := hajjv1.SeasonType_SEASON_TYPE_HAJJ
	if value.Type == domain.SeasonTypeUmrah {
		seasonType = hajjv1.SeasonType_SEASON_TYPE_UMRAH
	}
	return &hajjv1.Season{
		Id:         value.ID,
		OperatorId: value.OperatorID,
		Name:       value.Name,
		Type:       seasonType,
		StartDate:  timestamppb.New(value.StartDate),
		EndDate:    timestamppb.New(value.EndDate),
		IsActive:   value.IsActive,
		CreatedAt:  timestamppb.New(value.CreatedAt),
	}
}
