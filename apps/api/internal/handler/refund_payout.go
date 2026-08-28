package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type RefundPayoutHandler struct{ service *service.RefundPayoutService }

func NewRefundPayoutHandler(refundPayoutService *service.RefundPayoutService) *RefundPayoutHandler {
	return &RefundPayoutHandler{service: refundPayoutService}
}

func (h *RefundPayoutHandler) GetMyRefundWallet(ctx context.Context, req *connect.Request[hajjv1.GetMyRefundWalletRequest]) (*connect.Response[hajjv1.RefundWallet], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.GetMyRefundWallet(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *RefundPayoutHandler) RequestRefundPayout(ctx context.Context, req *connect.Request[hajjv1.RequestRefundPayoutRequest]) (*connect.Response[hajjv1.RefundPayoutRequest], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.RequestRefundPayout(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *RefundPayoutHandler) GetMyAgentRefundWallet(ctx context.Context, req *connect.Request[hajjv1.GetMyAgentRefundWalletRequest]) (*connect.Response[hajjv1.RefundWallet], error) {
	result, err := h.service.GetMyAgentRefundWallet(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *RefundPayoutHandler) RequestAgentRefundPayout(ctx context.Context, req *connect.Request[hajjv1.RequestAgentRefundPayoutRequest]) (*connect.Response[hajjv1.RefundPayoutRequest], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.RequestAgentRefundPayout(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *RefundPayoutHandler) ListRefundPayoutRequests(ctx context.Context, req *connect.Request[hajjv1.ListRefundPayoutRequestsRequest]) (*connect.Response[hajjv1.ListRefundPayoutRequestsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.ListRefundPayoutRequests(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *RefundPayoutHandler) TransitionRefundPayout(ctx context.Context, req *connect.Request[hajjv1.TransitionRefundPayoutRequest]) (*connect.Response[hajjv1.RefundPayoutRequest], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.TransitionRefundPayout(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
