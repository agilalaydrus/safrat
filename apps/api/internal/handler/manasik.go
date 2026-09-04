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

type ManasikHandler struct {
	manasikService *service.ManasikService
}

func NewManasikHandler(manasikService *service.ManasikService) *ManasikHandler {
	return &ManasikHandler{manasikService: manasikService}
}

func (h *ManasikHandler) CreateManasikCurriculum(ctx context.Context, req *connect.Request[hajjv1.CreateManasikCurriculumRequest]) (*connect.Response[hajjv1.ManasikCurriculum], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.manasikService.CreateCurriculum(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ManasikHandler) UpdateManasikCurriculum(ctx context.Context, req *connect.Request[hajjv1.UpdateManasikCurriculumRequest]) (*connect.Response[hajjv1.ManasikCurriculum], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.manasikService.UpdateCurriculum(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ManasikHandler) DeleteManasikCurriculum(ctx context.Context, req *connect.Request[hajjv1.DeleteManasikCurriculumRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.manasikService.DeleteCurriculum(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *ManasikHandler) ListManasikCurricula(ctx context.Context, req *connect.Request[hajjv1.ListManasikCurriculaRequest]) (*connect.Response[hajjv1.ListManasikCurriculaResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.manasikService.ListCurricula(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ManasikHandler) CreateManasikSession(ctx context.Context, req *connect.Request[hajjv1.CreateManasikSessionRequest]) (*connect.Response[hajjv1.ManasikSession], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.manasikService.CreateSession(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ManasikHandler) UpdateManasikSession(ctx context.Context, req *connect.Request[hajjv1.UpdateManasikSessionRequest]) (*connect.Response[hajjv1.ManasikSession], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.manasikService.UpdateSession(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ManasikHandler) UpdateManasikSessionStatus(ctx context.Context, req *connect.Request[hajjv1.UpdateManasikSessionStatusRequest]) (*connect.Response[hajjv1.ManasikSession], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.manasikService.UpdateSessionStatus(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ManasikHandler) DeleteManasikSession(ctx context.Context, req *connect.Request[hajjv1.DeleteManasikSessionRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.manasikService.DeleteSession(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *ManasikHandler) ListManasikSessions(ctx context.Context, req *connect.Request[hajjv1.ListManasikSessionsRequest]) (*connect.Response[hajjv1.ListManasikSessionsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.manasikService.ListSessions(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *ManasikHandler) RecordManasikAttendance(ctx context.Context, req *connect.Request[hajjv1.RecordManasikAttendanceRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.manasikService.RecordAttendance(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *ManasikHandler) ListManasikAttendance(ctx context.Context, req *connect.Request[hajjv1.ListManasikAttendanceRequest]) (*connect.Response[hajjv1.ListManasikAttendanceResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.manasikService.ListAttendance(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
