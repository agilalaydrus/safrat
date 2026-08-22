package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type LostReportHandler struct {
	lostReportService *service.LostReportService
}

func NewLostReportHandler(lostReportService *service.LostReportService) *LostReportHandler {
	return &LostReportHandler{lostReportService: lostReportService}
}

func (h *LostReportHandler) ReportLost(ctx context.Context, req *connect.Request[hajjv1.ReportLostRequest]) (*connect.Response[hajjv1.LostReport], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.lostReportService.ReportLost(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *LostReportHandler) ListActiveLostReports(ctx context.Context, req *connect.Request[hajjv1.ListActiveLostReportsRequest]) (*connect.Response[hajjv1.ListActiveLostReportsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.lostReportService.ListActive(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *LostReportHandler) ResolveLostReport(ctx context.Context, req *connect.Request[hajjv1.ResolveLostReportRequest]) (*connect.Response[hajjv1.ResolveLostReportResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.lostReportService.Resolve(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *LostReportHandler) ResolveGroupLostReport(ctx context.Context, req *connect.Request[hajjv1.ResolveGroupLostReportRequest]) (*connect.Response[hajjv1.ResolveLostReportResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.lostReportService.ResolveForGroup(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *LostReportHandler) ListGroupLostReports(ctx context.Context, req *connect.Request[hajjv1.ListGroupLostReportsRequest]) (*connect.Response[hajjv1.ListGroupLostReportsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.lostReportService.ListForGroup(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
