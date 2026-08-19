package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type InsuranceHandler struct {
	insuranceService *service.InsuranceService
}

func NewInsuranceHandler(insuranceService *service.InsuranceService) *InsuranceHandler {
	return &InsuranceHandler{insuranceService: insuranceService}
}

func (h *InsuranceHandler) CreateInsuranceClaim(ctx context.Context, req *connect.Request[hajjv1.CreateInsuranceClaimRequest]) (*connect.Response[hajjv1.InsuranceClaim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.insuranceService.CreateClaim(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InsuranceHandler) ListInsuranceClaims(ctx context.Context, req *connect.Request[hajjv1.ListInsuranceClaimsRequest]) (*connect.Response[hajjv1.ListInsuranceClaimsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.insuranceService.ListClaims(ctx, middleware.OperatorIDFromCtx(ctx))
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InsuranceHandler) UpdateInsuranceClaimStatus(ctx context.Context, req *connect.Request[hajjv1.UpdateInsuranceClaimStatusRequest]) (*connect.Response[hajjv1.InsuranceClaim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.insuranceService.UpdateClaimStatus(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *InsuranceHandler) GetInsuranceClaimExportData(ctx context.Context, req *connect.Request[hajjv1.GetInsuranceClaimExportDataRequest]) (*connect.Response[hajjv1.InsuranceClaimExportData], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.insuranceService.GetExportData(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
