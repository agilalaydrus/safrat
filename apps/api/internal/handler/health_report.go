package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type HealthReportHandler struct{ healthReportService *service.HealthReportService }

func NewHealthReportHandler(healthReportService *service.HealthReportService) *HealthReportHandler {
	return &HealthReportHandler{healthReportService: healthReportService}
}
func (h *HealthReportHandler) CreateHealthReport(ctx context.Context, req *connect.Request[hajjv1.CreateHealthReportRequest]) (*connect.Response[hajjv1.HealthReport], error) {
	result, err := h.healthReportService.CreateHealthReport(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *HealthReportHandler) ListHealthReports(ctx context.Context, req *connect.Request[hajjv1.ListHealthReportsRequest]) (*connect.Response[hajjv1.ListHealthReportsResponse], error) {
	result, err := h.healthReportService.ListHealthReports(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *HealthReportHandler) ResolveHealthReport(ctx context.Context, req *connect.Request[hajjv1.ResolveHealthReportRequest]) (*connect.Response[hajjv1.HealthReport], error) {
	result, err := h.healthReportService.ResolveHealthReport(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
