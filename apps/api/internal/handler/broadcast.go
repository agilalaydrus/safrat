package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type BroadcastHandler struct{ broadcastService *service.BroadcastService }

func NewBroadcastHandler(broadcastService *service.BroadcastService) *BroadcastHandler {
	return &BroadcastHandler{broadcastService: broadcastService}
}

func (h *BroadcastHandler) CreateBroadcast(ctx context.Context, req *connect.Request[hajjv1.CreateBroadcastRequest]) (*connect.Response[hajjv1.Broadcast], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.broadcastService.Create(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *BroadcastHandler) ListBroadcasts(ctx context.Context, req *connect.Request[hajjv1.ListBroadcastsRequest]) (*connect.Response[hajjv1.ListBroadcastsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.broadcastService.List(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *BroadcastHandler) DeleteBroadcast(ctx context.Context, req *connect.Request[hajjv1.DeleteBroadcastRequest]) (*connect.Response[hajjv1.DeleteBroadcastResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.broadcastService.Delete(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
