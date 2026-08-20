package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type TripHandler struct {
	tripService *service.TripService
}

func NewTripHandler(tripService *service.TripService) *TripHandler {
	return &TripHandler{tripService: tripService}
}

func (h *TripHandler) GetTripRoster(ctx context.Context, req *connect.Request[hajjv1.GetTripRosterRequest]) (*connect.Response[hajjv1.GetTripRosterResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.tripService.GetTripRoster(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *TripHandler) SetTripHotelCheckIn(ctx context.Context, req *connect.Request[hajjv1.SetTripHotelCheckInRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.tripService.SetTripHotelCheckIn(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *TripHandler) ListTripMovements(ctx context.Context, req *connect.Request[hajjv1.ListTripMovementsRequest]) (*connect.Response[hajjv1.ListTripMovementsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.tripService.ListTripMovements(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *TripHandler) ListTripCheckIns(ctx context.Context, req *connect.Request[hajjv1.ListTripCheckInsRequest]) (*connect.Response[hajjv1.ListTripCheckInsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.tripService.ListTripCheckIns(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *TripHandler) CreateTripCheckIn(ctx context.Context, req *connect.Request[hajjv1.CreateTripCheckInRequest]) (*connect.Response[hajjv1.CheckIn], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.tripService.CreateTripCheckIn(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *TripHandler) ListTripSOSAlerts(ctx context.Context, req *connect.Request[hajjv1.ListTripSOSAlertsRequest]) (*connect.Response[hajjv1.ListTripSOSAlertsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.tripService.ListTripSOSAlerts(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *TripHandler) AcknowledgeTripSOSAlert(ctx context.Context, req *connect.Request[hajjv1.AcknowledgeTripSOSAlertRequest]) (*connect.Response[hajjv1.SOSAlert], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.tripService.AcknowledgeTripSOSAlert(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *TripHandler) ResolveTripSOSAlert(ctx context.Context, req *connect.Request[hajjv1.ResolveTripSOSAlertRequest]) (*connect.Response[hajjv1.SOSAlert], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.tripService.ResolveTripSOSAlert(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
