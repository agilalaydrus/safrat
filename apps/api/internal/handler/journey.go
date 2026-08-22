package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type JourneyHandler struct{ journeyService *service.JourneyService }

func NewJourneyHandler(journeyService *service.JourneyService) *JourneyHandler {
	return &JourneyHandler{journeyService: journeyService}
}
func (h *JourneyHandler) UpdatePilgrimStatus(ctx context.Context, req *connect.Request[hajjv1.UpdatePilgrimStatusRequest]) (*connect.Response[hajjv1.PilgrimJourneyStatus], error) {
	result, err := h.journeyService.UpdatePilgrimStatus(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *JourneyHandler) BulkUpdateStatus(ctx context.Context, req *connect.Request[hajjv1.BulkUpdateStatusRequest]) (*connect.Response[hajjv1.BulkUpdateStatusResponse], error) {
	result, err := h.journeyService.BulkUpdateStatus(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *JourneyHandler) GetPilgrimStatus(ctx context.Context, req *connect.Request[hajjv1.GetPilgrimStatusRequest]) (*connect.Response[hajjv1.PilgrimJourneyStatus], error) {
	result, err := h.journeyService.GetPilgrimStatus(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *JourneyHandler) GetKloterJourneyOverview(ctx context.Context, req *connect.Request[hajjv1.GetKloterJourneyOverviewRequest]) (*connect.Response[hajjv1.KloterJourneyOverview], error) {
	result, err := h.journeyService.GetKloterJourneyOverview(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
