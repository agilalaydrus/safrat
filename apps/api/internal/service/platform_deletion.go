package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RequestTenantDataExport is D6's "wajib ditawarkan" step (TUGAS-PANEL-SAAS.md,
// §7.3 DESAIN) — the same operator_data_exports mechanism
// DataExportService.RequestDataExport already uses for an operator's own
// self-service export, just requested on their behalf by a platform admin
// instead of by themselves. Reuses the exact same worker, storage and
// download-URL path; nothing new to build there.
func (s *PlatformService) RequestTenantDataExport(ctx context.Context, req *hajjv1.RequestTenantDataExportRequest) (*hajjv1.DataExportRow, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("PlatformService.RequestTenantDataExport", apperror.ErrValidation)
	}
	row, err := s.dataExportRepository.Request(ctx, req.OperatorId, userID, strings.TrimSpace(req.IdempotencyKey))
	if err != nil {
		return nil, serviceError("PlatformService.RequestTenantDataExport", err)
	}
	_ = s.auditRepository.Write(ctx, req.OperatorId, userID, "tenant_data_export_requested", "operator", req.OperatorId,
		"diminta oleh platform sebagai bagian dari proses penghapusan tenant")
	return dataExportMessage(row), nil
}

// DeleteTenant is D6 (TUGAS-PANEL-SAAS.md, §7.3 DESAIN) — the most
// irreversible action in this system. All three preconditions (90 days
// since access lapsed, a READY export on file, and the tenant's own name
// typed as confirmation) are re-checked inside
// PlatformRepository.DeleteTenant's own transaction; the HasReadyExport
// check here happens first only so a request that will obviously be
// refused fails before taking the advisory lock at all.
func (s *PlatformService) DeleteTenant(ctx context.Context, req *hajjv1.DeleteTenantRequest) (*hajjv1.DeleteTenantResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) || len(strings.TrimSpace(req.Reason)) < 10 ||
		strings.TrimSpace(req.Confirmation) == "" || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("PlatformService.DeleteTenant", apperror.ErrValidation)
	}

	hasExport, err := s.dataExportRepository.HasReadyExport(ctx, req.OperatorId)
	if err != nil {
		return nil, serviceError("PlatformService.DeleteTenant", err)
	}

	// Recorded on the action rather than inferred later — same reasoning as
	// Suspend/Reinstate: how many people could have objected is a fact
	// about this moment, not something to compute after the fact.
	admins, err := s.platformRepository.CountPlatformAdmins(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.DeleteTenant", err)
	}

	result, err := s.platformRepository.DeleteTenant(ctx, repository.DeletionChange{
		OperatorID: req.OperatorId, ActorUserID: userID, Reason: strings.TrimSpace(req.Reason),
		Confirmation: strings.TrimSpace(req.Confirmation), IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		AdminCount: int32(admins),
	}, hasExport)
	if err != nil {
		return nil, serviceError("PlatformService.DeleteTenant", err)
	}
	return &hajjv1.DeleteTenantResponse{
		OperatorId: result.OperatorID, OperatorName: result.OperatorName,
		DeletedAt: timestamppb.New(result.DeletedAt),
	}, nil
}
