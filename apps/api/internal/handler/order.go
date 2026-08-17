package handler

import (
	"context"

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
	result, err := h.orderService.CreateOrder(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) ListOrders(ctx context.Context, req *connect.Request[hajjv1.ListOrdersRequest]) (*connect.Response[hajjv1.ListOrdersResponse], error) {
	result, err := h.orderService.ListOrders(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *OrderHandler) GetOrder(ctx context.Context, req *connect.Request[hajjv1.GetOrderRequest]) (*connect.Response[hajjv1.Order], error) {
	result, err := h.orderService.GetOrder(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
