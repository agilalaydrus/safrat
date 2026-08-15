package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type GroupHandler struct{ groupService *service.GroupService }

func NewGroupHandler(groupService *service.GroupService) *GroupHandler {
	return &GroupHandler{groupService: groupService}
}
func (h *GroupHandler) ListGroups(ctx context.Context, req *connect.Request[hajjv1.ListGroupsRequest]) (*connect.Response[hajjv1.ListGroupsResponse], error) {
	result, err := h.groupService.ListGroups(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupHandler) CreateGroup(ctx context.Context, req *connect.Request[hajjv1.CreateGroupRequest]) (*connect.Response[hajjv1.Group], error) {
	result, err := h.groupService.CreateGroup(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupHandler) UpdateGroup(ctx context.Context, req *connect.Request[hajjv1.UpdateGroupRequest]) (*connect.Response[hajjv1.Group], error) {
	result, err := h.groupService.UpdateGroup(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupHandler) DeleteGroup(ctx context.Context, req *connect.Request[hajjv1.DeleteGroupRequest]) (*connect.Response[emptypb.Empty], error) {
	result, err := h.groupService.DeleteGroup(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupHandler) ListOperatorMembers(ctx context.Context, _ *connect.Request[hajjv1.ListOperatorMembersRequest]) (*connect.Response[hajjv1.ListOperatorMembersResponse], error) {
	result, err := h.groupService.ListOperatorMembers(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *GroupHandler) GetGroupRoster(ctx context.Context, req *connect.Request[hajjv1.GetGroupRosterRequest]) (*connect.Response[hajjv1.GetGroupRosterResponse], error) {
	result, err := h.groupService.GetGroupRoster(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
