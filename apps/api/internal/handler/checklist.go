package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type ChecklistHandler struct {
	checklistService *service.ChecklistService
}

func NewChecklistHandler(checklistService *service.ChecklistService) *ChecklistHandler {
	return &ChecklistHandler{checklistService: checklistService}
}

func (h *ChecklistHandler) CreateChecklistTemplate(ctx context.Context, req *connect.Request[hajjv1.CreateChecklistTemplateRequest]) (*connect.Response[hajjv1.ChecklistTemplate], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.checklistService.CreateTemplate(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ChecklistHandler) ListChecklistTemplates(ctx context.Context, req *connect.Request[hajjv1.ListChecklistTemplatesRequest]) (*connect.Response[hajjv1.ListChecklistTemplatesResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.checklistService.ListTemplates(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ChecklistHandler) DeleteChecklistTemplate(ctx context.Context, req *connect.Request[hajjv1.DeleteChecklistTemplateRequest]) (*connect.Response[hajjv1.DeleteChecklistTemplateResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.checklistService.DeleteTemplate(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ChecklistHandler) GetPilgrimChecklist(ctx context.Context, req *connect.Request[hajjv1.GetPilgrimChecklistRequest]) (*connect.Response[hajjv1.GetPilgrimChecklistResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.checklistService.GetPilgrimChecklist(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ChecklistHandler) UpdateChecklistItem(ctx context.Context, req *connect.Request[hajjv1.UpdateChecklistItemRequest]) (*connect.Response[hajjv1.ChecklistItem], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.checklistService.UpdateChecklistItem(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ChecklistHandler) GetChecklistStats(ctx context.Context, req *connect.Request[hajjv1.GetChecklistStatsRequest]) (*connect.Response[hajjv1.GetChecklistStatsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.checklistService.GetStats(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ChecklistHandler) GetMyChecklist(ctx context.Context, req *connect.Request[hajjv1.GetMyChecklistRequest]) (*connect.Response[hajjv1.GetMyChecklistResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.checklistService.GetMyChecklist(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ChecklistHandler) CompleteMyChecklistItem(ctx context.Context, req *connect.Request[hajjv1.CompleteMyChecklistItemRequest]) (*connect.Response[hajjv1.ChecklistItem], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.checklistService.CompleteMyChecklistItem(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
