package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type KloterHandler struct{ kloterService *service.KloterService }

func NewKloterHandler(kloterService *service.KloterService) *KloterHandler {
	return &KloterHandler{kloterService: kloterService}
}
func (h *KloterHandler) ListKloters(ctx context.Context, req *connect.Request[hajjv1.ListKlotersRequest]) (*connect.Response[hajjv1.ListKlotersResponse], error) {
	result, err := h.kloterService.ListKloters(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *KloterHandler) CreateKloter(ctx context.Context, req *connect.Request[hajjv1.CreateKloterRequest]) (*connect.Response[hajjv1.Kloter], error) {
	result, err := h.kloterService.CreateKloter(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *KloterHandler) UpdateKloter(ctx context.Context, req *connect.Request[hajjv1.UpdateKloterRequest]) (*connect.Response[hajjv1.Kloter], error) {
	result, err := h.kloterService.UpdateKloter(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *KloterHandler) DeleteKloter(ctx context.Context, req *connect.Request[hajjv1.DeleteKloterRequest]) (*connect.Response[emptypb.Empty], error) {
	result, err := h.kloterService.DeleteKloter(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
