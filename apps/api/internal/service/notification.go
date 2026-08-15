package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/emptypb"
)

type NotificationService struct {
	operatorRepository *repository.OperatorRepository
	pushRepository     *repository.NotificationRepository
}

func NewNotificationService(operators *repository.OperatorRepository, push *repository.NotificationRepository) *NotificationService {
	return &NotificationService{operatorRepository: operators, pushRepository: push}
}

func (s *NotificationService) RegisterPushSubscription(ctx context.Context, orgID string, req *hajjv1.RegisterPushSubscriptionRequest) (*emptypb.Empty, error) {
	if req == nil || strings.TrimSpace(req.FcmToken) == "" {
		return nil, serviceError("NotificationService.RegisterPushSubscription", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("NotificationService.RegisterPushSubscription", err)
	}
	if err := s.pushRepository.RegisterToken(ctx, op.ID, middleware.UserIDFromCtx(ctx), req.FcmToken); err != nil {
		return nil, serviceError("NotificationService.RegisterPushSubscription", err)
	}
	return &emptypb.Empty{}, nil
}
