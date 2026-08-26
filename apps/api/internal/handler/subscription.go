package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type SubscriptionHandler struct {
	subscriptionService *service.SubscriptionService
}

func NewSubscriptionHandler(subscriptionService *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{subscriptionService: subscriptionService}
}

func (h *SubscriptionHandler) GetMySubscription(ctx context.Context, _ *connect.Request[hajjv1.GetMySubscriptionRequest]) (*connect.Response[hajjv1.GetMySubscriptionResponse], error) {
	result, err := h.subscriptionService.GetMine(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SubscriptionHandler) ListMyInvoices(ctx context.Context, req *connect.Request[hajjv1.ListMyInvoicesRequest]) (*connect.Response[hajjv1.ListMyInvoicesResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.subscriptionService.ListMine(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg.Limit)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SubscriptionHandler) CreateInvoice(ctx context.Context, req *connect.Request[hajjv1.CreateInvoiceRequest]) (*connect.Response[hajjv1.SubscriptionInvoice], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.subscriptionService.CreateInvoice(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
