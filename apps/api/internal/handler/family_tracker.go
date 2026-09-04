package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/service"
)

type FamilyTrackerHandler struct {
	familyTrackerService *service.FamilyTrackerService
}

func NewFamilyTrackerHandler(familyTrackerService *service.FamilyTrackerService) *FamilyTrackerHandler {
	return &FamilyTrackerHandler{familyTrackerService: familyTrackerService}
}

func (h *FamilyTrackerHandler) GetFamilyStatus(ctx context.Context, req *connect.Request[hajjv1.GetFamilyStatusRequest]) (*connect.Response[hajjv1.FamilyStatus], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.familyTrackerService.GetFamilyStatus(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *FamilyTrackerHandler) ListFamilyMoments(ctx context.Context, req *connect.Request[hajjv1.ListFamilyMomentsRequest]) (*connect.Response[hajjv1.ListFamilyMomentsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.familyTrackerService.ListFamilyMoments(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
