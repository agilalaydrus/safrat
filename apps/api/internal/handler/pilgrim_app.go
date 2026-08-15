package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/service"
)

type PilgrimAppHandler struct{ pilgrimAppService *service.PilgrimAppService }

func NewPilgrimAppHandler(pilgrimAppService *service.PilgrimAppService) *PilgrimAppHandler {
	return &PilgrimAppHandler{pilgrimAppService: pilgrimAppService}
}
func (h *PilgrimAppHandler) GetMyInfo(ctx context.Context, req *connect.Request[hajjv1.PilgrimAppRequest]) (*connect.Response[hajjv1.PilgrimAppInfo], error) {
	result, err := h.pilgrimAppService.GetMyInfo(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *PilgrimAppHandler) ListMySchedule(ctx context.Context, req *connect.Request[hajjv1.PilgrimAppRequest]) (*connect.Response[hajjv1.ListMyScheduleResponse], error) {
	result, err := h.pilgrimAppService.ListMySchedule(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *PilgrimAppHandler) UpdateMyLocation(ctx context.Context, req *connect.Request[hajjv1.UpdateMyLocationRequest]) (*connect.Response[hajjv1.UpdateMyLocationResponse], error) {
	result, err := h.pilgrimAppService.UpdateMyLocation(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *PilgrimAppHandler) ListMyProducts(ctx context.Context, req *connect.Request[hajjv1.PilgrimAppRequest]) (*connect.Response[hajjv1.ListMyProductsResponse], error) {
	result, err := h.pilgrimAppService.ListMyProducts(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
