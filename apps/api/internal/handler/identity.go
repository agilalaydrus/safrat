package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/service"
)

type IdentityHandler struct{ identityService *service.IdentityService }

func NewIdentityHandler(identityService *service.IdentityService) *IdentityHandler {
	return &IdentityHandler{identityService: identityService}
}

func (h *IdentityHandler) GetMyAccess(ctx context.Context, req *connect.Request[hajjv1.GetMyAccessRequest]) (*connect.Response[hajjv1.MyAccess], error) {
	result, err := h.identityService.GetMyAccess(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
