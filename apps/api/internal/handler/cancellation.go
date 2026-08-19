package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type CancellationHandler struct {
	cancellationService *service.CancellationService
}

func NewCancellationHandler(cancellationService *service.CancellationService) *CancellationHandler {
	return &CancellationHandler{cancellationService: cancellationService}
}

func (h *CancellationHandler) SetCancellationPolicy(ctx context.Context, req *connect.Request[hajjv1.SetCancellationPolicyRequest]) (*connect.Response[hajjv1.CancellationPolicy], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cancellationService.SetPolicy(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CancellationHandler) ListCancellationPolicies(ctx context.Context, req *connect.Request[hajjv1.ListCancellationPoliciesRequest]) (*connect.Response[hajjv1.ListCancellationPoliciesResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cancellationService.ListPolicies(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CancellationHandler) DeleteCancellationPolicy(ctx context.Context, req *connect.Request[hajjv1.DeleteCancellationPolicyRequest]) (*connect.Response[hajjv1.DeleteCancellationPolicyResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cancellationService.DeletePolicy(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CancellationHandler) PreviewCancellation(ctx context.Context, req *connect.Request[hajjv1.PreviewCancellationRequest]) (*connect.Response[hajjv1.CancellationPreview], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cancellationService.PreviewCancellation(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CancellationHandler) ConfirmCancellation(ctx context.Context, req *connect.Request[hajjv1.ConfirmCancellationRequest]) (*connect.Response[hajjv1.PilgrimCancellation], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cancellationService.ConfirmCancellation(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CancellationHandler) ListCancellations(ctx context.Context, req *connect.Request[hajjv1.ListCancellationsRequest]) (*connect.Response[hajjv1.ListCancellationsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.cancellationService.ListCancellations(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
