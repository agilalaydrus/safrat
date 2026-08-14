package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SeasonService struct {
	operatorRepository *repository.OperatorRepository
	seasonRepository   *repository.SeasonRepository
}

func NewSeasonService(operatorRepository *repository.OperatorRepository, seasonRepository *repository.SeasonRepository) *SeasonService {
	return &SeasonService{operatorRepository: operatorRepository, seasonRepository: seasonRepository}
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
