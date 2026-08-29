package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type ShipmentHandler struct {
	shipmentService *service.ShipmentService
}

func NewShipmentHandler(shipments *service.ShipmentService) *ShipmentHandler {
	return &ShipmentHandler{shipmentService: shipments}
}

func (h *ShipmentHandler) ListShipments(ctx context.Context, req *connect.Request[hajjv1.ListShipmentsRequest]) (*connect.Response[hajjv1.ListShipmentsResponse], error) {
	result, err := h.shipmentService.ListShipments(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ShipmentHandler) SaveShipmentDestination(ctx context.Context, req *connect.Request[hajjv1.SaveShipmentDestinationRequest]) (*connect.Response[hajjv1.Shipment], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.shipmentService.SaveDestination(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ShipmentHandler) MarkShipmentSent(ctx context.Context, req *connect.Request[hajjv1.MarkShipmentSentRequest]) (*connect.Response[hajjv1.Shipment], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.shipmentService.MarkSent(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ShipmentHandler) MarkShipmentHandedOver(ctx context.Context, req *connect.Request[hajjv1.MarkShipmentHandedOverRequest]) (*connect.Response[hajjv1.Shipment], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.shipmentService.MarkHandedOver(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
