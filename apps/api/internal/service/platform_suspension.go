package service

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SuspendTenant locks a travel agency out on purpose.
//
// Four-eyes, as far as four eyes can go with one pair. There is one platform
// admin today, so approval cannot come from a second person; what stands in for
// it is a confirmation the admin has to type from the tenant's own name, and a
// row that records honestly that only one admin existed at the time. The day
// there are two, the second signature has somewhere to go and the earlier rows
// do not pretend they had one.
func (s *PlatformService) SuspendTenant(ctx context.Context, req *hajjv1.SuspendTenantRequest) (*hajjv1.SuspendTenantResponse, error) {
	result, err := s.changeSuspension(ctx, suspensionRequest{
		OperatorID: req.GetOperatorId(), Reason: req.GetReason(),
		Confirmation: req.GetConfirmation(), IdempotencyKey: req.GetIdempotencyKey(),
	}, true)
	if err != nil {
		return nil, err
	}
	response := &hajjv1.SuspendTenantResponse{OperatorId: result.OperatorID, OperatorName: result.OperatorName}
	if result.SuspendedAt != nil {
		response.SuspendedAt = timestamppb.New(*result.SuspendedAt)
	}
	if result.AccessUntil != nil {
		response.AccessUntil = timestamppb.New(*result.AccessUntil)
	}
	return response, nil
}

func (s *PlatformService) ReinstateTenant(ctx context.Context, req *hajjv1.ReinstateTenantRequest) (*hajjv1.ReinstateTenantResponse, error) {
	result, err := s.changeSuspension(ctx, suspensionRequest{
		OperatorID: req.GetOperatorId(), Reason: req.GetReason(),
		Confirmation: req.GetConfirmation(), IdempotencyKey: req.GetIdempotencyKey(),
	}, false)
	if err != nil {
		return nil, err
	}
	response := &hajjv1.ReinstateTenantResponse{OperatorId: result.OperatorID, OperatorName: result.OperatorName}
	if result.AccessUntil != nil {
		response.AccessUntil = timestamppb.New(*result.AccessUntil)
	}
	return response, nil
}

type suspensionRequest struct {
	OperatorID     string
	Reason         string
	Confirmation   string
	IdempotencyKey string
}

func (s *PlatformService) changeSuspension(ctx context.Context, req suspensionRequest, suspend bool) (repository.SuspensionResult, error) {
	result := repository.SuspensionResult{}
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return result, err
	}
	if !isUUID(req.OperatorID) || len(strings.TrimSpace(req.Reason)) < 10 ||
		strings.TrimSpace(req.Confirmation) == "" || len(strings.TrimSpace(req.IdempotencyKey)) < 8 {
		return result, serviceError("PlatformService.SuspendTenant", apperror.ErrValidation)
	}

	// Recorded on the action rather than inferred later: how many people could
	// have objected is a fact about the moment, and counting admins today would
	// answer a different question.
	admins, err := s.platformRepository.CountPlatformAdmins(ctx)
	if err != nil {
		return result, serviceError("PlatformService.SuspendTenant", err)
	}

	change := repository.SuspensionChange{
		OperatorID: req.OperatorID, ActorUserID: userID, Reason: req.Reason,
		Confirmation: req.Confirmation, IdempotencyKey: req.IdempotencyKey,
		AdminCount: int32(admins),
	}
	if suspend {
		result, err = s.platformRepository.Suspend(ctx, change)
	} else {
		result, err = s.platformRepository.Reinstate(ctx, change)
	}
	switch {
	case errors.Is(err, repository.ErrTenantNotFound):
		return result, connect.NewError(connect.CodeNotFound, errors.New("travel tidak ditemukan"))
	case errors.Is(err, apperror.ErrValidation):
		// Most often the typed name did not match. Saying so is the point of
		// the ceremony — a generic rejection would send somebody hunting for a
		// permissions problem.
		return result, connect.NewError(connect.CodeInvalidArgument,
			errors.New("konfirmasi harus persis nama travel ini, dan alasan minimal 10 huruf"))
	case errors.Is(err, apperror.ErrConflict):
		return result, connect.NewError(connect.CodeAlreadyExists,
			errors.New("kunci idempotensi ini sudah dipakai untuk travel lain"))
	case err != nil:
		return result, serviceError("PlatformService.SuspendTenant", err)
	}
	return result, nil
}

func (s *PlatformService) ListPrivilegedActions(ctx context.Context, req *hajjv1.ListPrivilegedActionsRequest) (*hajjv1.ListPrivilegedActionsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) {
		return nil, serviceError("PlatformService.ListPrivilegedActions", apperror.ErrValidation)
	}
	actions, err := s.platformRepository.ListPrivilegedActionsForOperator(ctx, req.OperatorId, req.Limit)
	if err != nil {
		return nil, serviceError("PlatformService.ListPrivilegedActions", err)
	}
	response := &hajjv1.ListPrivilegedActionsResponse{Actions: make([]*hajjv1.PrivilegedActionRow, 0, len(actions))}
	for _, action := range actions {
		row := &hajjv1.PrivilegedActionRow{
			Id: action.ID, Kind: action.Kind, Reason: action.Reason,
			RequestedBy: action.RequestedBy, ApprovedBy: action.ApprovedBy,
			RequestedAt: timestamppb.New(action.RequestedAt), AdminCountAtRequest: action.AdminCount,
		}
		if action.ExecutedAt != nil {
			row.ExecutedAt = timestamppb.New(*action.ExecutedAt)
		}
		response.Actions = append(response.Actions, row)
	}
	return response, nil
}
