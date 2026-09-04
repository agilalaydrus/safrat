package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type InventoryHandler struct {
	inventoryService *service.InventoryService
}

func NewInventoryHandler(inventoryService *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{inventoryService: inventoryService}
}

func (h *InventoryHandler) CreateInventoryItem(ctx context.Context, req *connect.Request[hajjv1.CreateInventoryItemRequest]) (*connect.Response[hajjv1.InventoryItem], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.inventoryService.CreateItem(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) UpdateInventoryItem(ctx context.Context, req *connect.Request[hajjv1.UpdateInventoryItemRequest]) (*connect.Response[hajjv1.InventoryItem], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.inventoryService.UpdateItem(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) DeleteInventoryItem(ctx context.Context, req *connect.Request[hajjv1.DeleteInventoryItemRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.inventoryService.DeleteItem(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *InventoryHandler) ListInventoryItems(ctx context.Context, req *connect.Request[hajjv1.ListInventoryItemsRequest]) (*connect.Response[hajjv1.ListInventoryItemsResponse], error) {
	result, err := h.inventoryService.ListItems(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) AdjustStock(ctx context.Context, req *connect.Request[hajjv1.AdjustStockRequest]) (*connect.Response[hajjv1.InventoryItem], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.inventoryService.AdjustStock(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) ListStockMovements(ctx context.Context, req *connect.Request[hajjv1.ListStockMovementsRequest]) (*connect.Response[hajjv1.ListStockMovementsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.inventoryService.ListStockMovements(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) CreatePurchaseOrder(ctx context.Context, req *connect.Request[hajjv1.CreatePurchaseOrderRequest]) (*connect.Response[hajjv1.PurchaseOrder], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.inventoryService.CreatePurchaseOrder(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) UpdatePurchaseOrderStatus(ctx context.Context, req *connect.Request[hajjv1.UpdatePurchaseOrderStatusRequest]) (*connect.Response[hajjv1.PurchaseOrder], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.inventoryService.UpdatePurchaseOrderStatus(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) ListPurchaseOrders(ctx context.Context, req *connect.Request[hajjv1.ListPurchaseOrdersRequest]) (*connect.Response[hajjv1.ListPurchaseOrdersResponse], error) {
	result, err := h.inventoryService.ListPurchaseOrders(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) AddPurchaseOrderItem(ctx context.Context, req *connect.Request[hajjv1.AddPurchaseOrderItemRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.inventoryService.AddPurchaseOrderItem(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *InventoryHandler) ListPurchaseOrderItems(ctx context.Context, req *connect.Request[hajjv1.ListPurchaseOrderItemsRequest]) (*connect.Response[hajjv1.ListPurchaseOrderItemsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.inventoryService.ListPurchaseOrderItems(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) ReceivePurchaseOrderItem(ctx context.Context, req *connect.Request[hajjv1.ReceivePurchaseOrderItemRequest]) (*connect.Response[hajjv1.PurchaseOrderItem], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.inventoryService.ReceivePurchaseOrderItem(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InventoryHandler) GetInventorySummary(ctx context.Context, req *connect.Request[hajjv1.GetInventorySummaryRequest]) (*connect.Response[hajjv1.GetInventorySummaryResponse], error) {
	result, err := h.inventoryService.GetSummary(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
