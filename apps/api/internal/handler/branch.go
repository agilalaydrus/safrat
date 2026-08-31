package handler

import (
	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"context"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type BranchHandler struct{ service *service.BranchService }

func NewBranchHandler(s *service.BranchService) *BranchHandler { return &BranchHandler{service: s} }
func (h *BranchHandler) ListBranches(c context.Context, r *connect.Request[hajjv1.ListBranchesRequest]) (*connect.Response[hajjv1.ListBranchesResponse], error) {
	v, e := h.service.List(c, middleware.OperatorIDFromCtx(c), r.Msg.IncludeInactive)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *BranchHandler) CreateBranch(c context.Context, r *connect.Request[hajjv1.CreateBranchRequest]) (*connect.Response[hajjv1.Branch], error) {
	if e := protovalidate.Validate(r.Msg); e != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, e)
	}
	v, e := h.service.Create(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *BranchHandler) UpdateBranch(c context.Context, r *connect.Request[hajjv1.UpdateBranchRequest]) (*connect.Response[hajjv1.Branch], error) {
	if e := protovalidate.Validate(r.Msg); e != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, e)
	}
	v, e := h.service.Update(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *BranchHandler) AssignBranchHead(c context.Context, r *connect.Request[hajjv1.AssignBranchHeadRequest]) (*connect.Response[hajjv1.Branch], error) {
	if e := protovalidate.Validate(r.Msg); e != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, e)
	}
	v, e := h.service.AssignHead(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
func (h *BranchHandler) GetBranchPerformance(c context.Context, r *connect.Request[hajjv1.GetBranchPerformanceRequest]) (*connect.Response[hajjv1.GetBranchPerformanceResponse], error) {
	if e := protovalidate.Validate(r.Msg); e != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, e)
	}
	v, e := h.service.Performance(c, middleware.OperatorIDFromCtx(c), r.Msg)
	if e != nil {
		return nil, connectError(e)
	}
	return connect.NewResponse(v), nil
}
