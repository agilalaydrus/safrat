package handler

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type PilgrimHandler struct {
	pilgrimService *service.PilgrimService
}

func NewPilgrimHandler(pilgrimService *service.PilgrimService) *PilgrimHandler {
	return &PilgrimHandler{pilgrimService: pilgrimService}
}

func (h *PilgrimHandler) CreatePilgrim(ctx context.Context, req *connect.Request[hajjv1.CreatePilgrimRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.Create(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) GetPilgrim(ctx context.Context, req *connect.Request[hajjv1.GetPilgrimRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.Get(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) ListPilgrims(ctx context.Context, req *connect.Request[hajjv1.ListPilgrimsRequest]) (*connect.Response[hajjv1.ListPilgrimsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.List(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) GetPilgrimStats(ctx context.Context, req *connect.Request[hajjv1.GetPilgrimStatsRequest]) (*connect.Response[hajjv1.PilgrimStats], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.Stats(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) UpdatePilgrim(ctx context.Context, req *connect.Request[hajjv1.UpdatePilgrimRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.Update(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) MarkSubstituted(ctx context.Context, req *connect.Request[hajjv1.MarkSubstitutedRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.MarkSubstituted(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg.PilgrimId)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) RegenerateAccessCode(ctx context.Context, req *connect.Request[hajjv1.RegenerateAccessCodeRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.RegenerateAccessCode(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg.PilgrimId)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) UpdatePilgrimPayment(ctx context.Context, req *connect.Request[hajjv1.UpdatePilgrimPaymentRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.UpdatePayment(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) UpdatePilgrimDocuments(ctx context.Context, req *connect.Request[hajjv1.UpdatePilgrimDocumentsRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.UpdateDocuments(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) UpdatePilgrimEmergencyContact(ctx context.Context, req *connect.Request[hajjv1.UpdatePilgrimEmergencyContactRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.UpdateEmergencyContact(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) CheckInPilgrimHotel(ctx context.Context, req *connect.Request[hajjv1.CheckInPilgrimHotelRequest]) (*connect.Response[hajjv1.Pilgrim], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.CheckInHotel(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) ListPilgrimsWithExpiringPassports(ctx context.Context, req *connect.Request[hajjv1.ListPilgrimsWithExpiringPassportsRequest]) (*connect.Response[hajjv1.ListPilgrimsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.ListWithExpiringPassports(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) ListPilgrimDocuments(ctx context.Context, req *connect.Request[hajjv1.ListPilgrimDocumentsRequest]) (*connect.Response[hajjv1.ListPilgrimDocumentsResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.ListDocuments(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) DeletePilgrimDocument(ctx context.Context, req *connect.Request[hajjv1.DeletePilgrimDocumentRequest]) (*connect.Response[hajjv1.DeletePilgrimDocumentResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.DeleteDocument(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}

func (h *PilgrimHandler) SubstitutePilgrim(ctx context.Context, req *connect.Request[hajjv1.SubstitutePilgrimRequest]) (*connect.Response[hajjv1.SubstitutePilgrimResult], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.pilgrimService.SubstitutePilgrim(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg.OriginalPilgrimId, req.Msg.ReplacementPilgrimId)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
