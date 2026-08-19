package service

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type BroadcastService struct {
	operatorRepository  *repository.OperatorRepository
	broadcastRepository *repository.BroadcastRepository
	auditRepository     *repository.AuditRepository
}

func NewBroadcastService(operators *repository.OperatorRepository, broadcasts *repository.BroadcastRepository, audit *repository.AuditRepository) *BroadcastService {
	return &BroadcastService{operatorRepository: operators, broadcastRepository: broadcasts, auditRepository: audit}
}

func (s *BroadcastService) Create(ctx context.Context, orgID string, req *hajjv1.CreateBroadcastRequest) (*hajjv1.Broadcast, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("BroadcastService.Create", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("BroadcastService.Create", err)
	}
	broadcast, err := s.broadcastRepository.Create(ctx, op.ID, req.SeasonId, req.Title, req.Body)
	if err != nil {
		return nil, serviceError("BroadcastService.Create", err)
	}
	_ = s.auditRepository.Write(ctx, op.ID, middleware.UserIDFromCtx(ctx), "broadcast_sent", "broadcast", broadcast.ID, broadcast.Title)
	return broadcastMessage(broadcast), nil
}

func (s *BroadcastService) List(ctx context.Context, orgID string, req *hajjv1.ListBroadcastsRequest) (*hajjv1.ListBroadcastsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("BroadcastService.List", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("BroadcastService.List", err)
	}
	broadcasts, err := s.broadcastRepository.List(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("BroadcastService.List", err)
	}
	return broadcastsResponse(broadcasts), nil
}

func (s *BroadcastService) Delete(ctx context.Context, orgID string, req *hajjv1.DeleteBroadcastRequest) (*hajjv1.DeleteBroadcastResponse, error) {
	if req == nil || !isUUID(req.BroadcastId) {
		return nil, serviceError("BroadcastService.Delete", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("BroadcastService.Delete", err)
	}
	if err := s.broadcastRepository.Delete(ctx, op.ID, req.BroadcastId); err != nil {
		return nil, serviceError("BroadcastService.Delete", err)
	}
	return &hajjv1.DeleteBroadcastResponse{}, nil
}

func broadcastsResponse(broadcasts []*domain.Broadcast) *hajjv1.ListBroadcastsResponse {
	result := &hajjv1.ListBroadcastsResponse{Broadcasts: make([]*hajjv1.Broadcast, 0, len(broadcasts))}
	for _, b := range broadcasts {
		result.Broadcasts = append(result.Broadcasts, broadcastMessage(b))
	}
	return result
}

func broadcastMessage(value *domain.Broadcast) *hajjv1.Broadcast {
	return &hajjv1.Broadcast{Id: value.ID, OperatorId: value.OperatorID, SeasonId: value.SeasonID, Title: value.Title, Body: value.Body, CreatedAt: timestamppb.New(value.CreatedAt)}
}
