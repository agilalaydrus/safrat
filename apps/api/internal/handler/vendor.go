package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type VendorHandler struct {
	vendorService *service.VendorService
}

func NewVendorHandler(vendorService *service.VendorService) *VendorHandler {
	return &VendorHandler{vendorService: vendorService}
}

func (h *VendorHandler) CreateVendorContract(ctx context.Context, req *connect.Request[hajjv1.CreateVendorContractRequest]) (*connect.Response[hajjv1.VendorContract], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.vendorService.CreateContract(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *VendorHandler) ListVendorContracts(ctx context.Context, req *connect.Request[hajjv1.ListVendorContractsRequest]) (*connect.Response[hajjv1.ListVendorContractsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.vendorService.ListContracts(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *VendorHandler) UpdateVendorContract(ctx context.Context, req *connect.Request[hajjv1.UpdateVendorContractRequest]) (*connect.Response[hajjv1.VendorContract], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.vendorService.UpdateContract(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *VendorHandler) DeleteVendorContract(ctx context.Context, req *connect.Request[hajjv1.DeleteVendorContractRequest]) (*connect.Response[hajjv1.DeleteVendorContractResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.vendorService.DeleteContract(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *VendorHandler) AddContractEvent(ctx context.Context, req *connect.Request[hajjv1.AddContractEventRequest]) (*connect.Response[hajjv1.ContractEvent], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.vendorService.AddContractEvent(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *VendorHandler) ListContractEvents(ctx context.Context, req *connect.Request[hajjv1.ListContractEventsRequest]) (*connect.Response[hajjv1.ListContractEventsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.vendorService.ListContractEvents(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *VendorHandler) GetVendorSLAStatus(ctx context.Context, req *connect.Request[hajjv1.GetVendorSLAStatusRequest]) (*connect.Response[hajjv1.GetVendorSLAStatusResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.vendorService.GetSLAStatus(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
