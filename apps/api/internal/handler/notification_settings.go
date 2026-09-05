package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type NotificationSettingsHandler struct {
	notificationSettingsService *service.NotificationSettingsService
}

func NewNotificationSettingsHandler(notificationSettingsService *service.NotificationSettingsService) *NotificationSettingsHandler {
	return &NotificationSettingsHandler{notificationSettingsService: notificationSettingsService}
}

func (h *NotificationSettingsHandler) GetNotificationSettings(ctx context.Context, req *connect.Request[hajjv1.GetNotificationSettingsRequest]) (*connect.Response[hajjv1.NotificationSettings], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.notificationSettingsService.GetNotificationSettings(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *NotificationSettingsHandler) SetNotificationSettings(ctx context.Context, req *connect.Request[hajjv1.SetNotificationSettingsRequest]) (*connect.Response[hajjv1.NotificationSettings], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.notificationSettingsService.SetNotificationSettings(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
