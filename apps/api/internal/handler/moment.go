package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MomentHandler struct {
	momentService *service.MomentService
}

func NewMomentHandler(momentService *service.MomentService) *MomentHandler {
	return &MomentHandler{momentService: momentService}
}

func (h *MomentHandler) CreateMomentUpload(ctx context.Context, req *connect.Request[hajjv1.CreateMomentUploadRequest]) (*connect.Response[hajjv1.CreateMomentUploadResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.momentService.CreateMomentUpload(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *MomentHandler) CreateMoment(ctx context.Context, req *connect.Request[hajjv1.CreateMomentRequest]) (*connect.Response[hajjv1.Moment], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.momentService.CreateMoment(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserNameFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *MomentHandler) DeleteMoment(ctx context.Context, req *connect.Request[hajjv1.DeleteMomentRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.momentService.DeleteMoment(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *MomentHandler) ListMoments(ctx context.Context, req *connect.Request[hajjv1.ListMomentsRequest]) (*connect.Response[hajjv1.ListMomentsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.momentService.ListMoments(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
