package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type InquiryHandler struct{ service *service.InquiryService }

func NewInquiryHandler(value *service.InquiryService) *InquiryHandler { return &InquiryHandler{service: value} }

// SubmitInquiry is public — no operator identity in ctx, req.Msg.OperatorId
// carries it and the service re-validates it exists.
func (h *InquiryHandler) SubmitInquiry(ctx context.Context, req *connect.Request[hajjv1.SubmitInquiryRequest]) (*connect.Response[hajjv1.SubmitInquiryResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.Submit(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InquiryHandler) ListInquiries(ctx context.Context, req *connect.Request[hajjv1.ListInquiriesRequest]) (*connect.Response[hajjv1.ListInquiriesResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.List(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InquiryHandler) ConvertInquiryToLead(ctx context.Context, req *connect.Request[hajjv1.ConvertInquiryToLeadRequest]) (*connect.Response[hajjv1.CRMLead], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.ConvertToLead(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InquiryHandler) DismissInquiry(ctx context.Context, req *connect.Request[hajjv1.DismissInquiryRequest]) (*connect.Response[hajjv1.DismissInquiryResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.Dismiss(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
