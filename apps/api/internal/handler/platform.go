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

func (h *PlatformHandler) ListOperators(ctx context.Context, _ *connect.Request[hajjv1.ListOperatorsRequest]) (*connect.Response[hajjv1.ListOperatorsResponse], error) {
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

func (h *PlatformHandler) ListSuppliers(ctx context.Context, _ *connect.Request[hajjv1.ListSuppliersRequest]) (*connect.Response[hajjv1.ListSuppliersResponse], error) {
	result, err := h.platformService.ListSuppliers(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SaveSupplier(ctx context.Context, req *connect.Request[hajjv1.SaveSupplierRequest]) (*connect.Response[hajjv1.SaveSupplierResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SaveSupplier(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListProductRoutes(ctx context.Context, _ *connect.Request[hajjv1.ListProductRoutesRequest]) (*connect.Response[hajjv1.ListProductRoutesResponse], error) {
	result, err := h.platformService.ListProductRoutes(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SaveProductRoute(ctx context.Context, req *connect.Request[hajjv1.SaveProductRouteRequest]) (*connect.Response[hajjv1.SaveProductRouteResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SaveProductRoute(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListResponseRules(ctx context.Context, req *connect.Request[hajjv1.ListResponseRulesRequest]) (*connect.Response[hajjv1.ListResponseRulesResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.ListResponseRules(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) CreateResponseRule(ctx context.Context, req *connect.Request[hajjv1.CreateResponseRuleRequest]) (*connect.Response[hajjv1.CreateResponseRuleResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.CreateResponseRule(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SetResponseRuleActive(ctx context.Context, req *connect.Request[hajjv1.SetResponseRuleActiveRequest]) (*connect.Response[hajjv1.SetResponseRuleActiveResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SetResponseRuleActive(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) TestResponseRules(ctx context.Context, req *connect.Request[hajjv1.TestResponseRulesRequest]) (*connect.Response[hajjv1.TestResponseRulesResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.TestResponseRules(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListSupplierLogs(ctx context.Context, req *connect.Request[hajjv1.ListSupplierLogsRequest]) (*connect.Response[hajjv1.ListSupplierLogsResponse], error) {
	result, err := h.platformService.ListSupplierLogs(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListTransactions(ctx context.Context, req *connect.Request[hajjv1.ListTransactionsRequest]) (*connect.Response[hajjv1.ListTransactionsResponse], error) {
	result, err := h.platformService.ListTransactions(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
