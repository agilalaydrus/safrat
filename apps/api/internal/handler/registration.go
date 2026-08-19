package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type RegistrationHandler struct{ registrationService *service.RegistrationService }

func NewRegistrationHandler(registrationService *service.RegistrationService) *RegistrationHandler {
	return &RegistrationHandler{registrationService: registrationService}
}

func (h *RegistrationHandler) SubmitRegistration(ctx context.Context, req *connect.Request[hajjv1.SubmitRegistrationRequest]) (*connect.Response[hajjv1.SubmitRegistrationResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.registrationService.Submit(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
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
