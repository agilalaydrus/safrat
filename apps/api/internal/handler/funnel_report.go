package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type FunnelReportHandler struct {
	funnelReportService *service.FunnelReportService
}

func NewFunnelReportHandler(funnelReportService *service.FunnelReportService) *FunnelReportHandler {
	return &FunnelReportHandler{funnelReportService: funnelReportService}
}

func (h *FunnelReportHandler) GetFunnelReport(ctx context.Context, req *connect.Request[hajjv1.GetFunnelReportRequest]) (*connect.Response[hajjv1.FunnelReport], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.funnelReportService.GetReport(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
