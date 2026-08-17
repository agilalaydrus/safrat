package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type SOSHandler struct{ sosService *service.SOSService }

func NewSOSHandler(sosService *service.SOSService) *SOSHandler {
	return &SOSHandler{sosService: sosService}
}
func (h *SOSHandler) CreateSOSAlert(ctx context.Context, req *connect.Request[hajjv1.CreateSOSAlertRequest]) (*connect.Response[hajjv1.SOSAlert], error) {
	result, err := h.sosService.CreateSOSAlert(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *SOSHandler) ListActiveSOSAlerts(ctx context.Context, _ *connect.Request[hajjv1.ListActiveSOSAlertsRequest]) (*connect.Response[hajjv1.ListSOSAlertsResponse], error) {
	result, err := h.sosService.ListActiveSOSAlerts(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *SOSHandler) AcknowledgeSOSAlert(ctx context.Context, req *connect.Request[hajjv1.AcknowledgeSOSAlertRequest]) (*connect.Response[hajjv1.SOSAlert], error) {
	result, err := h.sosService.AcknowledgeSOSAlert(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *SOSHandler) ResolveSOSAlert(ctx context.Context, req *connect.Request[hajjv1.ResolveSOSAlertRequest]) (*connect.Response[hajjv1.SOSAlert], error) {
	result, err := h.sosService.ResolveSOSAlert(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *SOSHandler) ListMyPilgrimSOSAlerts(ctx context.Context, req *connect.Request[hajjv1.ListMyPilgrimSOSAlertsRequest]) (*connect.Response[hajjv1.ListSOSAlertsResponse], error) {
	result, err := h.sosService.ListMyPilgrimSOSAlerts(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
