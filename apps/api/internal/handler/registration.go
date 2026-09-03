package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/funnel"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type RegistrationHandler struct {
	registrationService *service.RegistrationService
	// Optional. A nil recorder means the funnel is simply not measured here,
	// which is the correct behaviour when no salt is configured — and it keeps
	// registration working in any environment that has not set one up.
	funnelRecorder funnel.Recorder
}

func NewRegistrationHandler(registrationService *service.RegistrationService, funnelRecorder funnel.Recorder) *RegistrationHandler {
	return &RegistrationHandler{registrationService: registrationService, funnelRecorder: funnelRecorder}
}

func (h *RegistrationHandler) SubmitRegistration(ctx context.Context, req *connect.Request[hajjv1.SubmitRegistrationRequest]) (*connect.Response[hajjv1.SubmitRegistrationResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// KIRIM before the attempt, SELESAI only if it lands. The gap between the
	// two is people who tried to register and were refused by our own
	// validation — the most actionable number in the funnel, and one that
	// disappears entirely if the two are recorded together.
	h.recordFunnel(ctx, req, "KIRIM", "")
	result, err := h.registrationService.Submit(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	h.recordFunnel(ctx, req, "SELESAI", result.GetRegistrationId())
	return connect.NewResponse(result), nil
}

// recordFunnel never fails the request it is measuring. A registration that
// goes uncounted costs a row in a report; a registration that fails because
// analytics was unhappy costs the agency a pilgrim.
func (h *RegistrationHandler) recordFunnel(ctx context.Context, req *connect.Request[hajjv1.SubmitRegistrationRequest], step, entityID string) {
	if h.funnelRecorder == nil {
		return
	}
	h.funnelRecorder.Record(ctx, funnel.Step{
		OperatorID: req.Msg.GetOperatorId(), Step: step, Path: "/register",
		UTMSource: req.Msg.GetUtmSource(), UTMCampaign: req.Msg.GetUtmCampaign(),
		EntityID: entityID,
		ClientIP: funnelClientIP(req.Header(), req.Peer().Addr), UserAgent: req.Header().Get("User-Agent"),
	})
}

func (h *RegistrationHandler) GetRegistrationForm(ctx context.Context, req *connect.Request[hajjv1.GetRegistrationFormRequest]) (*connect.Response[hajjv1.RegistrationFormInfo], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.registrationService.GetForm(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *RegistrationHandler) ListRegistrations(ctx context.Context, req *connect.Request[hajjv1.ListRegistrationsRequest]) (*connect.Response[hajjv1.ListRegistrationsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.registrationService.List(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *RegistrationHandler) ApproveRegistration(ctx context.Context, req *connect.Request[hajjv1.ApproveRegistrationRequest]) (*connect.Response[hajjv1.PilgrimRegistration], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.registrationService.Approve(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *RegistrationHandler) RejectRegistration(ctx context.Context, req *connect.Request[hajjv1.RejectRegistrationRequest]) (*connect.Response[hajjv1.PilgrimRegistration], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.registrationService.Reject(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
