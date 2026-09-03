package handler

import (
	"net/http"

	"context"
	"github.com/hajj-saas/api/internal/funnel"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type WaitlistHandler struct {
	waitlistService *service.WaitlistService
	// Optional, like everywhere else it appears: a nil recorder means the
	// funnel is not measured here, and joining a waitlist still works.
	funnelRecorder funnel.Recorder
}

func NewWaitlistHandler(waitlistService *service.WaitlistService, funnelRecorder funnel.Recorder) *WaitlistHandler {
	return &WaitlistHandler{waitlistService: waitlistService, funnelRecorder: funnelRecorder}
}

func (h *WaitlistHandler) JoinWaitlist(ctx context.Context, req *connect.Request[hajjv1.JoinWaitlistRequest]) (*connect.Response[hajjv1.JoinWaitlistResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Same split as registration: the attempt and the completion are separate
	// events, because the gap between them is people a full season turned away.
	// A waitlist join that fails is a pilgrim who wanted in and could not.
	h.recordFunnel(ctx, req.Msg.GetOperatorId(), "KIRIM", "", req.Header(), req.Peer().Addr)
	result, err := h.waitlistService.JoinWaitlist(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	// Only when an entry actually exists. is_full=false means the season still
	// had room and the visitor is being sent to the registration form instead —
	// that is a redirect, not a completed waitlist join, and counting it would
	// report a conversion nobody made.
	if result.GetIsFull() && result.GetEntry() != nil {
		h.recordFunnel(ctx, req.Msg.GetOperatorId(), "SELESAI", result.GetEntry().GetId(), req.Header(), req.Peer().Addr)
	}
	return connect.NewResponse(result), nil
}

// recordFunnel never fails the request it is measuring.
func (h *WaitlistHandler) recordFunnel(ctx context.Context, operatorID, step, entityID string, header http.Header, peerAddr string) {
	if h.funnelRecorder == nil {
		return
	}
	h.funnelRecorder.Record(ctx, funnel.Step{
		OperatorID: operatorID, Step: step, Path: "/waitlist", EntityID: entityID,
		ClientIP: funnelClientIP(header, peerAddr), UserAgent: header.Get("User-Agent"),
	})
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

func (h *WaitlistHandler) ConfirmWaitlistEntry(ctx context.Context, req *connect.Request[hajjv1.ConfirmWaitlistEntryRequest]) (*connect.Response[hajjv1.WaitlistEntry], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.waitlistService.ConfirmWaitlistEntry(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
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
