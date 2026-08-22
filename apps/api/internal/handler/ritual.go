package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

type RitualHandler struct{ ritualService *service.RitualService }

func NewRitualHandler(ritualService *service.RitualService) *RitualHandler {
	return &RitualHandler{ritualService: ritualService}
}
func (h *RitualHandler) ListRitualTemplates(ctx context.Context, req *connect.Request[hajjv1.ListRitualTemplatesRequest]) (*connect.Response[hajjv1.ListRitualTemplatesResponse], error) {
	result, err := h.ritualService.ListRitualTemplates(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *RitualHandler) CreateRitualTemplate(ctx context.Context, req *connect.Request[hajjv1.CreateRitualTemplateRequest]) (*connect.Response[hajjv1.RitualTemplate], error) {
	result, err := h.ritualService.CreateRitualTemplate(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *RitualHandler) SeedDefaultTemplates(ctx context.Context, req *connect.Request[hajjv1.SeedDefaultTemplatesRequest]) (*connect.Response[emptypb.Empty], error) {
	result, err := h.ritualService.SeedDefaultTemplates(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *RitualHandler) GetGroupRitualProgress(ctx context.Context, req *connect.Request[hajjv1.GetGroupRitualProgressRequest]) (*connect.Response[hajjv1.GroupRitualProgress], error) {
	result, err := h.ritualService.GetGroupRitualProgress(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *RitualHandler) BulkCompleteRitual(ctx context.Context, req *connect.Request[hajjv1.BulkCompleteRitualRequest]) (*connect.Response[hajjv1.BulkCompleteRitualResponse], error) {
	result, err := h.ritualService.BulkCompleteRitual(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *RitualHandler) CompleteRitualForPilgrim(ctx context.Context, req *connect.Request[hajjv1.CompleteRitualForPilgrimRequest]) (*connect.Response[emptypb.Empty], error) {
	result, err := h.ritualService.CompleteRitualForPilgrim(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
