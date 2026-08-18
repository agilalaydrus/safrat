package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type SeasonHandler struct {
	seasonService *service.SeasonService
}

func NewSeasonHandler(seasonService *service.SeasonService) *SeasonHandler {
	return &SeasonHandler{seasonService: seasonService}
}

func (h *SeasonHandler) CreateSeason(ctx context.Context, req *connect.Request[hajjv1.CreateSeasonRequest]) (*connect.Response[hajjv1.Season], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.seasonService.Create(ctx, operatorID, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SeasonHandler) ListSeasons(ctx context.Context, _ *connect.Request[hajjv1.ListSeasonsRequest]) (*connect.Response[hajjv1.ListSeasonsResponse], error) {
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.seasonService.List(ctx, operatorID)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SeasonHandler) SetActiveSeason(ctx context.Context, req *connect.Request[hajjv1.SetActiveSeasonRequest]) (*connect.Response[hajjv1.Season], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	operatorID := middleware.OperatorIDFromCtx(ctx)
	result, err := h.seasonService.SetActive(ctx, operatorID, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
