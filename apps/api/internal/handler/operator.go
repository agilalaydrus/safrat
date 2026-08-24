package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type OperatorHandler struct {
	operatorService *service.OperatorService
}

func NewOperatorHandler(operatorService *service.OperatorService) *OperatorHandler {
	return &OperatorHandler{operatorService: operatorService}
}

func (h *OperatorHandler) CreateOperator(ctx context.Context, req *connect.Request[hajjv1.CreateOperatorRequest]) (*connect.Response[hajjv1.Operator], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.operatorService.Create(ctx, operatorID, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OperatorHandler) CheckOperatorSlug(ctx context.Context, req *connect.Request[hajjv1.CheckOperatorSlugRequest]) (*connect.Response[hajjv1.CheckOperatorSlugResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.operatorService.CheckSlug(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OperatorHandler) GetMyOperator(ctx context.Context, _ *connect.Request[hajjv1.GetMyOperatorRequest]) (*connect.Response[hajjv1.Operator], error) {
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.operatorService.GetMy(ctx, operatorID)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OperatorHandler) UpdateOperator(ctx context.Context, req *connect.Request[hajjv1.UpdateOperatorRequest]) (*connect.Response[hajjv1.Operator], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.operatorService.Update(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OperatorHandler) ListAuditLogs(ctx context.Context, req *connect.Request[hajjv1.ListAuditLogsRequest]) (*connect.Response[hajjv1.ListAuditLogsResponse], error) {
	logs, err := h.operatorService.ListAuditLogs(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg.Limit)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&hajjv1.ListAuditLogsResponse{Logs: logs}), nil
}

func (h *OperatorHandler) UpdateMyProfile(ctx context.Context, req *connect.Request[hajjv1.UpdateOperatorProfileRequest]) (*connect.Response[hajjv1.Operator], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.operatorService.UpdateMyProfile(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OperatorHandler) GetPublicProfile(ctx context.Context, req *connect.Request[hajjv1.GetPublicProfileRequest]) (*connect.Response[hajjv1.GetPublicProfileResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.operatorService.GetPublicProfile(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OperatorHandler) ResolveOperatorSlug(ctx context.Context, req *connect.Request[hajjv1.ResolveOperatorSlugRequest]) (*connect.Response[hajjv1.ResolveOperatorSlugResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.operatorService.ResolveSlug(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
