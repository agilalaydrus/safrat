package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type CRMHandler struct{ service *service.CRMService }

func NewCRMHandler(value *service.CRMService) *CRMHandler { return &CRMHandler{service: value} }

func (h *CRMHandler) CreateLead(ctx context.Context, req *connect.Request[hajjv1.CreateLeadRequest]) (*connect.Response[hajjv1.CreateLeadResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.CreateLead(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CRMHandler) ListLeads(ctx context.Context, req *connect.Request[hajjv1.ListLeadsRequest]) (*connect.Response[hajjv1.ListLeadsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.ListLeads(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CRMHandler) GetLead(ctx context.Context, req *connect.Request[hajjv1.GetLeadRequest]) (*connect.Response[hajjv1.CRMLeadDetail], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.GetLead(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CRMHandler) UpdateLead(ctx context.Context, req *connect.Request[hajjv1.UpdateLeadRequest]) (*connect.Response[hajjv1.CRMLead], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.UpdateLead(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CRMHandler) MoveLeadStage(ctx context.Context, req *connect.Request[hajjv1.MoveLeadStageRequest]) (*connect.Response[hajjv1.CRMLead], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.MoveLeadStage(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CRMHandler) AddLeadActivity(ctx context.Context, req *connect.Request[hajjv1.AddLeadActivityRequest]) (*connect.Response[hajjv1.CRMLeadActivity], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.AddLeadActivity(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CRMHandler) GetDashboard(ctx context.Context, req *connect.Request[hajjv1.GetDashboardRequest]) (*connect.Response[hajjv1.CRMDashboard], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.GetDashboard(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *CRMHandler) ListAssignees(ctx context.Context, req *connect.Request[hajjv1.ListAssigneesRequest]) (*connect.Response[hajjv1.ListAssigneesResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.ListAssignees(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
