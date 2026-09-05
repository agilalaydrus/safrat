package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type SupportHandler struct {
	supportService *service.SupportService
}

func NewSupportHandler(supportService *service.SupportService) *SupportHandler {
	return &SupportHandler{supportService: supportService}
}

func (h *SupportHandler) CreateSupportTicket(ctx context.Context, req *connect.Request[hajjv1.CreateSupportTicketRequest]) (*connect.Response[hajjv1.SupportTicket], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.supportService.CreateSupportTicket(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), middleware.UserNameFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SupportHandler) ListMySupportTickets(ctx context.Context, req *connect.Request[hajjv1.ListMySupportTicketsRequest]) (*connect.Response[hajjv1.ListMySupportTicketsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.supportService.ListMySupportTickets(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SupportHandler) GetSupportTicket(ctx context.Context, req *connect.Request[hajjv1.GetSupportTicketRequest]) (*connect.Response[hajjv1.SupportTicketDetail], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.supportService.GetSupportTicket(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SupportHandler) AddSupportTicketMessage(ctx context.Context, req *connect.Request[hajjv1.AddSupportTicketMessageRequest]) (*connect.Response[hajjv1.SupportTicketMessage], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.supportService.AddSupportTicketMessage(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), middleware.UserNameFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SupportHandler) CloseSupportTicket(ctx context.Context, req *connect.Request[hajjv1.CloseSupportTicketRequest]) (*connect.Response[hajjv1.SupportTicket], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.supportService.CloseSupportTicket(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
