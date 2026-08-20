package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type GroupLeaderHandler struct{ groupLeaderService *service.GroupLeaderService }

func NewGroupLeaderHandler(groupLeaderService *service.GroupLeaderService) *GroupLeaderHandler {
	return &GroupLeaderHandler{groupLeaderService: groupLeaderService}
}
func (h *GroupLeaderHandler) ListMyGroups(ctx context.Context, _ *connect.Request[hajjv1.ListMyGroupsRequest]) (*connect.Response[hajjv1.ListMyGroupsResponse], error) {
	result, err := h.groupLeaderService.ListMyGroups(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupLeaderHandler) GetGroupRoster(ctx context.Context, req *connect.Request[hajjv1.GetGroupRosterRequest]) (*connect.Response[hajjv1.GetGroupRosterResponse], error) {
	result, err := h.groupLeaderService.GetGroupRoster(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupLeaderHandler) ListCheckIns(ctx context.Context, req *connect.Request[hajjv1.ListCheckInsRequest]) (*connect.Response[hajjv1.ListCheckInsResponse], error) {
	result, err := h.groupLeaderService.ListCheckIns(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupLeaderHandler) CreateCheckIn(ctx context.Context, req *connect.Request[hajjv1.CreateCheckInRequest]) (*connect.Response[hajjv1.CheckIn], error) {
	result, err := h.groupLeaderService.CreateCheckIn(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupLeaderHandler) ListMySOSAlerts(ctx context.Context, _ *connect.Request[hajjv1.ListMySOSAlertsRequest]) (*connect.Response[hajjv1.ListSOSAlertsResponse], error) {
	result, err := h.groupLeaderService.ListMySOSAlerts(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupLeaderHandler) CheckInGroupPilgrimHotel(ctx context.Context, req *connect.Request[hajjv1.CheckInGroupPilgrimHotelRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	result, err := h.groupLeaderService.CheckInGroupPilgrimHotel(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupLeaderHandler) AcknowledgeMySOSAlert(ctx context.Context, req *connect.Request[hajjv1.AcknowledgeMySOSAlertRequest]) (*connect.Response[hajjv1.SOSAlert], error) {
	result, err := h.groupLeaderService.AcknowledgeMySOSAlert(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupLeaderHandler) ResolveMySOSAlert(ctx context.Context, req *connect.Request[hajjv1.ResolveMySOSAlertRequest]) (*connect.Response[hajjv1.SOSAlert], error) {
	result, err := h.groupLeaderService.ResolveMySOSAlert(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
