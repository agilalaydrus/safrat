package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type WaitlistHandler struct {
	waitlistService *service.WaitlistService
}

func NewWaitlistHandler(waitlistService *service.WaitlistService) *WaitlistHandler {
	return &WaitlistHandler{waitlistService: waitlistService}
}

func (h *WaitlistHandler) JoinWaitlist(ctx context.Context, req *connect.Request[hajjv1.JoinWaitlistRequest]) (*connect.Response[hajjv1.JoinWaitlistResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.waitlistService.JoinWaitlist(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *WaitlistHandler) LeaveWaitlist(ctx context.Context, req *connect.Request[hajjv1.LeaveWaitlistRequest]) (*connect.Response[hajjv1.LeaveWaitlistResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.waitlistService.LeaveWaitlist(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *WaitlistHandler) ConfirmWaitlistSlot(ctx context.Context, req *connect.Request[hajjv1.ConfirmWaitlistSlotRequest]) (*connect.Response[hajjv1.ConfirmWaitlistSlotResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.waitlistService.ConfirmWaitlistSlot(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *WaitlistHandler) ListWaitlist(ctx context.Context, req *connect.Request[hajjv1.ListWaitlistRequest]) (*connect.Response[hajjv1.ListWaitlistResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.waitlistService.ListWaitlist(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *WaitlistHandler) PromoteFromWaitlist(ctx context.Context, req *connect.Request[hajjv1.PromoteFromWaitlistRequest]) (*connect.Response[hajjv1.WaitlistEntry], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.waitlistService.PromoteFromWaitlist(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *WaitlistHandler) RemoveFromWaitlist(ctx context.Context, req *connect.Request[hajjv1.RemoveFromWaitlistRequest]) (*connect.Response[hajjv1.RemoveFromWaitlistResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.waitlistService.RemoveFromWaitlist(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
