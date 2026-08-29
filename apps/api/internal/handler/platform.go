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

func (h *PlatformHandler) ListAccounts(ctx context.Context, req *connect.Request[hajjv1.ListAccountsRequest]) (*connect.Response[hajjv1.ListAccountsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.ListAccounts(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) GrantPlatformAdmin(ctx context.Context, req *connect.Request[hajjv1.GrantPlatformAdminRequest]) (*connect.Response[hajjv1.GrantPlatformAdminResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.GrantPlatformAdmin(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) RevokePlatformAdmin(ctx context.Context, req *connect.Request[hajjv1.RevokePlatformAdminRequest]) (*connect.Response[hajjv1.RevokePlatformAdminResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.RevokePlatformAdmin(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) RevokeSessions(ctx context.Context, req *connect.Request[hajjv1.RevokeSessionsRequest]) (*connect.Response[hajjv1.RevokeSessionsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.RevokeSessions(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListKycRecords(ctx context.Context, req *connect.Request[hajjv1.ListKycRecordsRequest]) (*connect.Response[hajjv1.ListKycRecordsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.ListKycRecords(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) GetKycRecord(ctx context.Context, req *connect.Request[hajjv1.GetKycRecordRequest]) (*connect.Response[hajjv1.GetKycRecordResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.GetKycRecord(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SetKycStatus(ctx context.Context, req *connect.Request[hajjv1.SetKycStatusRequest]) (*connect.Response[hajjv1.SetKycStatusResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SetKycStatus(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SetProductBasePrice(ctx context.Context, req *connect.Request[hajjv1.SetProductBasePriceRequest]) (*connect.Response[hajjv1.SetProductBasePriceResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SetProductBasePrice(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SavePlatformProduct(ctx context.Context, req *connect.Request[hajjv1.SavePlatformProductRequest]) (*connect.Response[hajjv1.SavePlatformProductResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SavePlatformProduct(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListPlatformCatalogue(ctx context.Context, req *connect.Request[hajjv1.ListPlatformCatalogueRequest]) (*connect.Response[hajjv1.ListPlatformCatalogueResponse], error) {
	result, err := h.platformService.ListPlatformCatalogue(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ResolveFulfilment(ctx context.Context, req *connect.Request[hajjv1.ResolveFulfilmentRequest]) (*connect.Response[hajjv1.ResolveFulfilmentResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.ResolveFulfilment(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
