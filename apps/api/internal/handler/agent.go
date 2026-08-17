package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type AgentHandler struct{ agentService *service.AgentService }

func NewAgentHandler(agentService *service.AgentService) *AgentHandler {
	return &AgentHandler{agentService: agentService}
}
func (h *AgentHandler) CreateAgent(ctx context.Context, req *connect.Request[hajjv1.CreateAgentRequest]) (*connect.Response[hajjv1.Agent], error) {
	result, err := h.agentService.Create(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) GetAgent(ctx context.Context, req *connect.Request[hajjv1.GetAgentRequest]) (*connect.Response[hajjv1.Agent], error) {
	result, err := h.agentService.Get(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) ListAgents(ctx context.Context, _ *connect.Request[hajjv1.ListAgentsRequest]) (*connect.Response[hajjv1.ListAgentsResponse], error) {
	result, err := h.agentService.List(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) ListAgentPayouts(ctx context.Context, _ *connect.Request[hajjv1.ListAgentPayoutsRequest]) (*connect.Response[hajjv1.ListAgentPayoutsResponse], error) {
	result, err := h.agentService.ListPayouts(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) RecordAgentPayout(ctx context.Context, req *connect.Request[hajjv1.RecordAgentPayoutRequest]) (*connect.Response[hajjv1.AgentPayout], error) {
	result, err := h.agentService.RecordPayout(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) ListAgentPayoutHistory(ctx context.Context, req *connect.Request[hajjv1.ListAgentPayoutHistoryRequest]) (*connect.Response[hajjv1.ListAgentPayoutHistoryResponse], error) {
	result, err := h.agentService.ListPayoutHistory(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) GetMyWallet(ctx context.Context, _ *connect.Request[hajjv1.GetMyWalletRequest]) (*connect.Response[hajjv1.AgentWallet], error) {
	result, err := h.agentService.GetMyWallet(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) RequestAgentPayout(ctx context.Context, req *connect.Request[hajjv1.RequestAgentPayoutRequest]) (*connect.Response[hajjv1.PayoutRequest], error) {
	result, err := h.agentService.RequestPayout(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) ListPayoutRequests(ctx context.Context, req *connect.Request[hajjv1.ListPayoutRequestsRequest]) (*connect.Response[hajjv1.ListPayoutRequestsResponse], error) {
	result, err := h.agentService.ListPayoutRequests(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) RejectPayoutRequest(ctx context.Context, req *connect.Request[hajjv1.RejectPayoutRequestRequest]) (*connect.Response[hajjv1.PayoutRequest], error) {
	result, err := h.agentService.RejectPayoutRequest(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) UpdateAgent(ctx context.Context, req *connect.Request[hajjv1.UpdateAgentRequest]) (*connect.Response[hajjv1.Agent], error) {
	result, err := h.agentService.Update(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) ApplyAsAgent(ctx context.Context, req *connect.Request[hajjv1.ApplyAsAgentRequest]) (*connect.Response[hajjv1.Agent], error) {
	result, err := h.agentService.ApplyAsAgent(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *AgentHandler) DeleteAgent(ctx context.Context, req *connect.Request[hajjv1.DeleteAgentRequest]) (*connect.Response[hajjv1.DeleteAgentResponse], error) {
	result, err := h.agentService.Delete(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
