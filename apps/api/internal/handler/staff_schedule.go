package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type StaffScheduleHandler struct {
	staffScheduleService *service.StaffScheduleService
}

func NewStaffScheduleHandler(staffScheduleService *service.StaffScheduleService) *StaffScheduleHandler {
	return &StaffScheduleHandler{staffScheduleService: staffScheduleService}
}

func (h *StaffScheduleHandler) AssignStaffToKloter(ctx context.Context, req *connect.Request[hajjv1.AssignStaffToKloterRequest]) (*connect.Response[hajjv1.KloterStaff], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.staffScheduleService.Assign(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *StaffScheduleHandler) ListKloterStaff(ctx context.Context, req *connect.Request[hajjv1.ListKloterStaffRequest]) (*connect.Response[hajjv1.ListKloterStaffResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.staffScheduleService.ListForKloter(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *StaffScheduleHandler) RemoveStaffFromKloter(ctx context.Context, req *connect.Request[hajjv1.RemoveStaffFromKloterRequest]) (*connect.Response[hajjv1.RemoveStaffFromKloterResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.staffScheduleService.Remove(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *StaffScheduleHandler) ListAllStaffSchedule(ctx context.Context, req *connect.Request[hajjv1.ListAllStaffScheduleRequest]) (*connect.Response[hajjv1.ListAllStaffScheduleResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.staffScheduleService.ListAll(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *StaffScheduleHandler) ListMyAssignments(ctx context.Context, req *connect.Request[hajjv1.ListMyAssignmentsRequest]) (*connect.Response[hajjv1.ListMyAssignmentsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.staffScheduleService.ListMine(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
