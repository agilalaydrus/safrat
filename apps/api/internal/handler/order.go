package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type OrderHandler struct{ orderService *service.OrderService }

func NewOrderHandler(orderService *service.OrderService) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *connect.Request[hajjv1.CreateOrderRequest]) (*connect.Response[hajjv1.CreateOrderResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.CreateOrder(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) CreateManualOrder(ctx context.Context, req *connect.Request[hajjv1.CreateManualOrderRequest]) (*connect.Response[hajjv1.CreateOrderResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.CreateManualOrder(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ListOrders(ctx context.Context, req *connect.Request[hajjv1.ListOrdersRequest]) (*connect.Response[hajjv1.ListOrdersResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.ListOrders(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) GetOrder(ctx context.Context, req *connect.Request[hajjv1.GetOrderRequest]) (*connect.Response[hajjv1.Order], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.GetOrder(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) RefundOrder(ctx context.Context, req *connect.Request[hajjv1.RefundOrderRequest]) (*connect.Response[hajjv1.RefundOrderResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.RefundOrder(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ListOrderRefunds(ctx context.Context, req *connect.Request[hajjv1.ListOrderRefundsRequest]) (*connect.Response[hajjv1.ListOrderRefundsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.ListOrderRefunds(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) CreateOrderForPilgrim(ctx context.Context, req *connect.Request[hajjv1.CreateOrderForPilgrimRequest]) (*connect.Response[hajjv1.CreateOrderResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.CreateOrderForPilgrim(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ListMyPurchaseCatalogue(ctx context.Context, req *connect.Request[hajjv1.ListMyPurchaseCatalogueRequest]) (*connect.Response[hajjv1.ListMyPurchaseCatalogueResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.ListMyPurchaseCatalogue(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) CreateOrderForSelf(ctx context.Context, req *connect.Request[hajjv1.CreateOrderForSelfRequest]) (*connect.Response[hajjv1.CreateOrderResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.CreateOrderForSelf(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ListMyOrders(ctx context.Context, req *connect.Request[hajjv1.ListMyOrdersRequest]) (*connect.Response[hajjv1.ListOrdersResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.ListMyOrders(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ResolveHeldOrder(ctx context.Context, req *connect.Request[hajjv1.ResolveHeldOrderRequest]) (*connect.Response[hajjv1.Order], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.ResolveHeldOrder(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ChangeOrderProduct(ctx context.Context, req *connect.Request[hajjv1.ChangeOrderProductRequest]) (*connect.Response[hajjv1.ChangeOrderProductResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.ChangeOrderProduct(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ListPlanChanges(ctx context.Context, req *connect.Request[hajjv1.ListPlanChangesRequest]) (*connect.Response[hajjv1.ListPlanChangesResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.ListPlanChanges(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ListPilgrimCredits(ctx context.Context, req *connect.Request[hajjv1.ListPilgrimCreditsRequest]) (*connect.Response[hajjv1.ListPilgrimCreditsResponse], error) {
	result, err := h.orderService.ListPilgrimCredits(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ResolvePilgrimCredit(ctx context.Context, req *connect.Request[hajjv1.ResolvePilgrimCreditRequest]) (*connect.Response[hajjv1.ResolvePilgrimCreditResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.ResolvePilgrimCredit(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ListOrdersForPilgrim(ctx context.Context, req *connect.Request[hajjv1.ListOrdersForPilgrimRequest]) (*connect.Response[hajjv1.ListOrdersResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.orderService.ListOrdersForPilgrim(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
