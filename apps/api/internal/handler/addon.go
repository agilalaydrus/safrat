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

type AddonHandler struct {
	addonService *service.AddonService
}

func NewAddonHandler(addonService *service.AddonService) *AddonHandler {
	return &AddonHandler{addonService: addonService}
}

func (h *AddonHandler) CreateAddonItem(ctx context.Context, req *connect.Request[hajjv1.CreateAddonItemRequest]) (*connect.Response[hajjv1.AddonItem], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.addonService.CreateAddonItem(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *AddonHandler) UpdateAddonItem(ctx context.Context, req *connect.Request[hajjv1.UpdateAddonItemRequest]) (*connect.Response[hajjv1.AddonItem], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.addonService.UpdateAddonItem(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *AddonHandler) ListAddonItems(ctx context.Context, req *connect.Request[hajjv1.ListAddonItemsRequest]) (*connect.Response[hajjv1.ListAddonItemsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.addonService.ListAddonItems(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *AddonHandler) AssignPilgrimAddon(ctx context.Context, req *connect.Request[hajjv1.AssignPilgrimAddonRequest]) (*connect.Response[hajjv1.PilgrimAddon], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.addonService.AssignPilgrimAddon(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *AddonHandler) SetPilgrimAddonPaid(ctx context.Context, req *connect.Request[hajjv1.SetPilgrimAddonPaidRequest]) (*connect.Response[hajjv1.PilgrimAddon], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.addonService.SetPilgrimAddonPaid(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *AddonHandler) RemovePilgrimAddon(ctx context.Context, req *connect.Request[hajjv1.RemovePilgrimAddonRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.addonService.RemovePilgrimAddon(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *AddonHandler) ListPilgrimAddons(ctx context.Context, req *connect.Request[hajjv1.ListPilgrimAddonsRequest]) (*connect.Response[hajjv1.ListPilgrimAddonsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.addonService.ListPilgrimAddons(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
