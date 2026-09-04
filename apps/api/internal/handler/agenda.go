package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AgendaHandler struct {
	agendaService *service.AgendaService
}

func NewAgendaHandler(agendaService *service.AgendaService) *AgendaHandler {
	return &AgendaHandler{agendaService: agendaService}
}

func (h *AgendaHandler) CreateAgendaEvent(ctx context.Context, req *connect.Request[hajjv1.CreateAgendaEventRequest]) (*connect.Response[hajjv1.AgendaEvent], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.agendaService.CreateAgendaEvent(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *AgendaHandler) UpdateAgendaEvent(ctx context.Context, req *connect.Request[hajjv1.UpdateAgendaEventRequest]) (*connect.Response[hajjv1.AgendaEvent], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.agendaService.UpdateAgendaEvent(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *AgendaHandler) DeleteAgendaEvent(ctx context.Context, req *connect.Request[hajjv1.DeleteAgendaEventRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.agendaService.DeleteAgendaEvent(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *AgendaHandler) ListAgenda(ctx context.Context, req *connect.Request[hajjv1.ListAgendaRequest]) (*connect.Response[hajjv1.ListAgendaResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.agendaService.ListAgenda(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
