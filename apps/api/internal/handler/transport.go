package handler

import (
	"connectrpc.com/connect"
	"context"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type TransportHandler struct{ s *service.TransportService }

func NewTransportHandler(s *service.TransportService) *TransportHandler { return &TransportHandler{s} }
func (h *TransportHandler) CreateMovement(c context.Context, r *connect.Request[hajjv1.CreateMovementRequest]) (*connect.Response[hajjv1.Movement], error) {
	v, e := h.s.CreateMovement(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) GetMovement(c context.Context, r *connect.Request[hajjv1.GetMovementRequest]) (*connect.Response[hajjv1.Movement], error) {
	v, e := h.s.GetMovement(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) ListMovements(c context.Context, r *connect.Request[hajjv1.ListMovementsRequest]) (*connect.Response[hajjv1.ListMovementsResponse], error) {
	v, e := h.s.ListMovements(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) UpdateMovementStatus(c context.Context, r *connect.Request[hajjv1.UpdateMovementStatusRequest]) (*connect.Response[hajjv1.Movement], error) {
	v, e := h.s.UpdateMovementStatus(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) DeleteMovement(c context.Context, r *connect.Request[hajjv1.DeleteMovementRequest]) (*connect.Response[emptypb.Empty], error) {
	v, e := h.s.DeleteMovement(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) CreateVehicle(c context.Context, r *connect.Request[hajjv1.CreateVehicleRequest]) (*connect.Response[hajjv1.Vehicle], error) {
	v, e := h.s.CreateVehicle(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) ListVehicles(c context.Context, r *connect.Request[hajjv1.ListVehiclesRequest]) (*connect.Response[hajjv1.ListVehiclesResponse], error) {
	v, e := h.s.ListVehicles(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) UpdateVehicleStatus(c context.Context, r *connect.Request[hajjv1.UpdateVehicleStatusRequest]) (*connect.Response[hajjv1.Vehicle], error) {
	v, e := h.s.UpdateVehicleStatus(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) DeleteVehicle(c context.Context, r *connect.Request[hajjv1.DeleteVehicleRequest]) (*connect.Response[emptypb.Empty], error) {
	v, e := h.s.DeleteVehicle(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) AssignSeat(c context.Context, r *connect.Request[hajjv1.AssignSeatRequest]) (*connect.Response[hajjv1.SeatAssignment], error) {
	v, e := h.s.AssignSeat(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) UnassignSeat(c context.Context, r *connect.Request[hajjv1.UnassignSeatRequest]) (*connect.Response[emptypb.Empty], error) {
	v, e := h.s.UnassignSeat(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *TransportHandler) GetVehicleManifest(c context.Context, r *connect.Request[hajjv1.GetVehicleManifestRequest]) (*connect.Response[hajjv1.VehicleManifest], error) {
	v, e := h.s.Manifest(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
