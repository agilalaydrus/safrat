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

type SecuritySettingsHandler struct {
	securitySettingsService *service.SecuritySettingsService
}

func NewSecuritySettingsHandler(securitySettingsService *service.SecuritySettingsService) *SecuritySettingsHandler {
	return &SecuritySettingsHandler{securitySettingsService: securitySettingsService}
}

func (h *SecuritySettingsHandler) GetSecurityPosture(ctx context.Context, req *connect.Request[hajjv1.GetSecurityPostureRequest]) (*connect.Response[hajjv1.SecurityPosture], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.securitySettingsService.GetSecurityPosture(ctx, middleware.OperatorIDFromCtx(ctx), middleware.ClientIPFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SecuritySettingsHandler) SetIpAllowlist(ctx context.Context, req *connect.Request[hajjv1.SetIpAllowlistRequest]) (*connect.Response[hajjv1.SecurityPosture], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.securitySettingsService.SetIpAllowlist(ctx, middleware.OperatorIDFromCtx(ctx), middleware.UserIDFromCtx(ctx),
		middleware.ClientIPFromCtx(ctx), middleware.OrgRoleFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SecuritySettingsHandler) ListActiveSessions(ctx context.Context, req *connect.Request[hajjv1.ListActiveSessionsRequest]) (*connect.Response[hajjv1.ListActiveSessionsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.securitySettingsService.ListActiveSessions(ctx, middleware.OperatorIDFromCtx(ctx), middleware.SessionIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *SecuritySettingsHandler) RevokeSession(ctx context.Context, req *connect.Request[hajjv1.RevokeSessionRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.securitySettingsService.RevokeSession(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}
