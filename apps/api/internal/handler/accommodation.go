package handler

import (
	"connectrpc.com/connect"
	"context"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AccommodationHandler struct{ s *service.AccommodationService }

func NewAccommodationHandler(s *service.AccommodationService) *AccommodationHandler {
	return &AccommodationHandler{s}
}
func (h *AccommodationHandler) CreateHotel(c context.Context, r *connect.Request[hajjv1.CreateHotelRequest]) (*connect.Response[hajjv1.Hotel], error) {
	v, e := h.s.CreateHotel(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *AccommodationHandler) GetHotel(c context.Context, r *connect.Request[hajjv1.GetHotelRequest]) (*connect.Response[hajjv1.Hotel], error) {
	v, e := h.s.GetHotel(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *AccommodationHandler) ListHotels(c context.Context, r *connect.Request[hajjv1.ListHotelsRequest]) (*connect.Response[hajjv1.ListHotelsResponse], error) {
	v, e := h.s.ListHotels(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *AccommodationHandler) CreateRoom(c context.Context, r *connect.Request[hajjv1.CreateRoomRequest]) (*connect.Response[hajjv1.Room], error) {
	v, e := h.s.CreateRoom(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *AccommodationHandler) BulkCreateRooms(c context.Context, r *connect.Request[hajjv1.BulkCreateRoomsRequest]) (*connect.Response[hajjv1.BulkCreateRoomsResponse], error) {
	v, e := h.s.BulkCreateRooms(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *AccommodationHandler) ListRooms(c context.Context, r *connect.Request[hajjv1.ListRoomsRequest]) (*connect.Response[hajjv1.ListRoomsResponse], error) {
	v, e := h.s.ListRooms(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *AccommodationHandler) AllocatePilgrim(c context.Context, r *connect.Request[hajjv1.AllocatePilgrimRequest]) (*connect.Response[hajjv1.RoomAllocation], error) {
	v, e := h.s.AllocatePilgrim(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *AccommodationHandler) DeallocatePilgrim(c context.Context, r *connect.Request[hajjv1.DeallocatePilgrimRequest]) (*connect.Response[emptypb.Empty], error) {
	v, e := h.s.DeallocatePilgrim(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *AccommodationHandler) GetRoomManifest(c context.Context, r *connect.Request[hajjv1.GetRoomManifestRequest]) (*connect.Response[hajjv1.RoomManifest], error) {
	v, e := h.s.Manifest(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
