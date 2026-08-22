package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type MonitoringHandler struct{ monitoringService *service.MonitoringService }

func NewMonitoringHandler(monitoringService *service.MonitoringService) *MonitoringHandler {
	return &MonitoringHandler{monitoringService: monitoringService}
}

func (h *MonitoringHandler) GetSnapshot(ctx context.Context, req *connect.Request[hajjv1.GetSnapshotRequest]) (*connect.Response[hajjv1.MonitoringSnapshot], error) {
	result, err := h.monitoringService.GetSnapshot(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *MonitoringHandler) StreamEvents(ctx context.Context, req *connect.Request[hajjv1.StreamEventsRequest], stream *connect.ServerStream[hajjv1.MonitoringPing]) error {
	if err := h.monitoringService.StreamEvents(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg, stream); err != nil {
		return connectError(err)
	}
	return nil
}
