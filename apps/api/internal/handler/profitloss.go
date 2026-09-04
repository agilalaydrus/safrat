package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type ProfitLossHandler struct {
	profitLossService *service.ProfitLossService
}

func NewProfitLossHandler(profitLossService *service.ProfitLossService) *ProfitLossHandler {
	return &ProfitLossHandler{profitLossService: profitLossService}
}

func (h *ProfitLossHandler) GetProfitLossReport(ctx context.Context, req *connect.Request[hajjv1.GetProfitLossReportRequest]) (*connect.Response[hajjv1.GetProfitLossReportResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.profitLossService.GetProfitLossReport(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ProfitLossHandler) StreamProfitLossExport(ctx context.Context, req *connect.Request[hajjv1.StreamProfitLossExportRequest], stream *connect.ServerStream[hajjv1.ProfitLossExportRow]) error {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.profitLossService.StreamProfitLossExport(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg, stream); err != nil {
		return connectError(err)
	}
	return nil
}
