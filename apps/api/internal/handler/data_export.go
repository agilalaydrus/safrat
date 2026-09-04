package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type DataExportHandler struct {
	dataExportService *service.DataExportService
}

func NewDataExportHandler(dataExportService *service.DataExportService) *DataExportHandler {
	return &DataExportHandler{dataExportService: dataExportService}
}

func (h *DataExportHandler) RequestDataExport(ctx context.Context, req *connect.Request[hajjv1.RequestDataExportRequest]) (*connect.Response[hajjv1.DataExportRow], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.dataExportService.RequestDataExport(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *DataExportHandler) ListDataExports(ctx context.Context, req *connect.Request[hajjv1.ListDataExportsRequest]) (*connect.Response[hajjv1.ListDataExportsResponse], error) {
	result, err := h.dataExportService.ListDataExports(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *DataExportHandler) GetDataExportDownloadUrl(ctx context.Context, req *connect.Request[hajjv1.GetDataExportDownloadUrlRequest]) (*connect.Response[hajjv1.GetDataExportDownloadUrlResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.dataExportService.GetDataExportDownloadUrl(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
