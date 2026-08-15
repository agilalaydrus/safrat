package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type NotificationHandler struct{ notificationService *service.NotificationService }

func NewNotificationHandler(notificationService *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}
func (h *NotificationHandler) RegisterPushSubscription(ctx context.Context, req *connect.Request[hajjv1.RegisterPushSubscriptionRequest]) (*connect.Response[emptypb.Empty], error) {
	result, err := h.notificationService.RegisterPushSubscription(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
