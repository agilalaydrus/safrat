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

func (h *PlatformHandler) ListPlanLimits(ctx context.Context, _ *connect.Request[hajjv1.ListPlanLimitsRequest]) (*connect.Response[hajjv1.ListPlanLimitsResponse], error) {
	result, err := h.platformService.ListPlanLimits(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) PreviewPlanLimitChange(ctx context.Context, req *connect.Request[hajjv1.PreviewPlanLimitChangeRequest]) (*connect.Response[hajjv1.PreviewPlanLimitChangeResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.PreviewPlanLimitChange(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SetPlanLimit(ctx context.Context, req *connect.Request[hajjv1.SetPlanLimitRequest]) (*connect.Response[hajjv1.SetPlanLimitResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SetPlanLimit(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListPlanOverrides(ctx context.Context, req *connect.Request[hajjv1.ListPlanOverridesRequest]) (*connect.Response[hajjv1.ListPlanOverridesResponse], error) {
	result, err := h.platformService.ListPlanOverrides(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) PreviewSubscriptionBilling(ctx context.Context, _ *connect.Request[hajjv1.PreviewSubscriptionBillingRequest]) (*connect.Response[hajjv1.PreviewSubscriptionBillingResponse], error) {
	result, err := h.platformService.PreviewSubscriptionBilling(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) IssueSubscriptionBilling(ctx context.Context, req *connect.Request[hajjv1.IssueSubscriptionBillingRequest]) (*connect.Response[hajjv1.IssueSubscriptionBillingResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.IssueSubscriptionBilling(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) GetSubscriptionBillingSettings(ctx context.Context, _ *connect.Request[hajjv1.GetSubscriptionBillingSettingsRequest]) (*connect.Response[hajjv1.GetSubscriptionBillingSettingsResponse], error) {
	result, err := h.platformService.GetSubscriptionBillingSettings(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SetTrialDays(ctx context.Context, req *connect.Request[hajjv1.SetTrialDaysRequest]) (*connect.Response[hajjv1.SetTrialDaysResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SetTrialDays(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SetSubscriptionGracePeriod(ctx context.Context, req *connect.Request[hajjv1.SetSubscriptionGracePeriodRequest]) (*connect.Response[hajjv1.SetSubscriptionGracePeriodResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SetSubscriptionGracePeriod(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) PreviewSubscriptionPlanChange(ctx context.Context, req *connect.Request[hajjv1.PreviewSubscriptionPlanChangeRequest]) (*connect.Response[hajjv1.PreviewSubscriptionPlanChangeResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.PreviewSubscriptionPlanChange(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ApplySubscriptionPlanChange(ctx context.Context, req *connect.Request[hajjv1.ApplySubscriptionPlanChangeRequest]) (*connect.Response[hajjv1.ApplySubscriptionPlanChangeResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.ApplySubscriptionPlanChange(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SetPlanOverride(ctx context.Context, req *connect.Request[hajjv1.SetPlanOverrideRequest]) (*connect.Response[hajjv1.SetPlanOverrideResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SetPlanOverride(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) DeletePlanOverride(ctx context.Context, req *connect.Request[hajjv1.DeletePlanOverrideRequest]) (*connect.Response[hajjv1.DeletePlanOverrideResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.DeletePlanOverride(ctx, req.Msg)
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
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
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

func (h *PlatformHandler) ListPendingTransfers(ctx context.Context, req *connect.Request[hajjv1.ListPendingTransfersRequest]) (*connect.Response[hajjv1.ListPendingTransfersResponse], error) {
	result, err := h.platformService.ListPendingTransfers(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ConfirmBankTransfer(ctx context.Context, req *connect.Request[hajjv1.ConfirmBankTransferRequest]) (*connect.Response[hajjv1.ConfirmBankTransferResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.ConfirmBankTransfer(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListBankMutations(ctx context.Context, req *connect.Request[hajjv1.ListBankMutationsRequest]) (*connect.Response[hajjv1.ListBankMutationsResponse], error) {
	result, err := h.platformService.ListBankMutations(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) SettleInvoiceWithMutation(ctx context.Context, req *connect.Request[hajjv1.SettleInvoiceWithMutationRequest]) (*connect.Response[hajjv1.SettleInvoiceWithMutationResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.SettleInvoiceWithMutation(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) IgnoreBankMutation(ctx context.Context, req *connect.Request[hajjv1.IgnoreBankMutationRequest]) (*connect.Response[hajjv1.IgnoreBankMutationResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.IgnoreBankMutation(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListSubscriptionInvoices(ctx context.Context, req *connect.Request[hajjv1.ListSubscriptionInvoicesRequest]) (*connect.Response[hajjv1.ListSubscriptionInvoicesResponse], error) {
	result, err := h.platformService.ListSubscriptionInvoices(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) VoidSubscriptionInvoice(ctx context.Context, req *connect.Request[hajjv1.VoidSubscriptionInvoiceRequest]) (*connect.Response[hajjv1.VoidSubscriptionInvoiceResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.platformService.VoidSubscriptionInvoice(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) ListUsage(ctx context.Context, _ *connect.Request[hajjv1.ListUsageRequest]) (*connect.Response[hajjv1.ListUsageResponse], error) {
	result, err := h.platformService.ListUsage(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) GetPlatformFunnel(ctx context.Context, req *connect.Request[hajjv1.GetPlatformFunnelRequest]) (*connect.Response[hajjv1.GetPlatformFunnelResponse], error) {
	result, err := h.platformService.GetPlatformFunnel(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PlatformHandler) GetTenantDetail(ctx context.Context, req *connect.Request[hajjv1.GetTenantDetailRequest]) (*connect.Response[hajjv1.GetTenantDetailResponse], error) {
	result, err := h.platformService.GetTenantDetail(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
