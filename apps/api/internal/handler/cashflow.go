package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type CashFlowHandler struct {
	cashFlowService *service.CashFlowService
}

func NewCashFlowHandler(cashFlowService *service.CashFlowService) *CashFlowHandler {
	return &CashFlowHandler{cashFlowService: cashFlowService}
}

func (h *CashFlowHandler) CreateVendorPayment(ctx context.Context, req *connect.Request[hajjv1.CreateVendorPaymentRequest]) (*connect.Response[hajjv1.VendorPayment], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cashFlowService.CreatePayment(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CashFlowHandler) ListVendorPayments(ctx context.Context, req *connect.Request[hajjv1.ListVendorPaymentsRequest]) (*connect.Response[hajjv1.ListVendorPaymentsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cashFlowService.ListPayments(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CashFlowHandler) UpdateVendorPaymentStatus(ctx context.Context, req *connect.Request[hajjv1.UpdateVendorPaymentStatusRequest]) (*connect.Response[hajjv1.VendorPayment], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cashFlowService.UpdatePaymentStatus(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CashFlowHandler) DeleteVendorPayment(ctx context.Context, req *connect.Request[hajjv1.DeleteVendorPaymentRequest]) (*connect.Response[hajjv1.DeleteVendorPaymentResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cashFlowService.DeletePayment(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CashFlowHandler) GetCashFlowSummary(ctx context.Context, req *connect.Request[hajjv1.GetCashFlowSummaryRequest]) (*connect.Response[hajjv1.CashFlowSummary], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cashFlowService.GetSummary(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CashFlowHandler) GetMonthlyProjection(ctx context.Context, req *connect.Request[hajjv1.GetMonthlyProjectionRequest]) (*connect.Response[hajjv1.GetMonthlyProjectionResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cashFlowService.GetMonthlyProjection(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
