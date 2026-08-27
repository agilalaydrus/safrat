package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/service"
)

type PlatformHandler struct{ platformService *service.PlatformService }

func NewPlatformHandler(platformService *service.PlatformService) *PlatformHandler {
	return &PlatformHandler{platformService: platformService}
}

func (h *PlatformHandler) AmIPlatformAdmin(ctx context.Context, _ *connect.Request[hajjv1.AmIPlatformAdminRequest]) (*connect.Response[hajjv1.AmIPlatformAdminResponse], error) {
	result, err := h.platformService.AmIPlatformAdmin(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListOperators(ctx context.Context, _ *connect.Request[hajjv1.ListPlatformOperatorsRequest]) (*connect.Response[hajjv1.ListPlatformOperatorsResponse], error) {
	result, err := h.platformService.ListOperators(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListProductsNeedingCost(ctx context.Context, req *connect.Request[hajjv1.ListProductsNeedingCostRequest]) (*connect.Response[hajjv1.ListProductsNeedingCostResponse], error) {
	result, err := h.platformService.ListProductsNeedingCost(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SetProductSupplierCost(ctx context.Context, req *connect.Request[hajjv1.SetProductSupplierCostRequest]) (*connect.Response[hajjv1.SetProductSupplierCostResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SetProductSupplierCost(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
