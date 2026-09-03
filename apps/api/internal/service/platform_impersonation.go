package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// StartImpersonation opens a read-only, time-bounded session on one tenant.
//
// The audit entry is written against the tenant, not against the platform, so
// it appears on that tenant's own trail: the customer's record should show that
// somebody from TawafiqHub looked at their account, and why.
func (s *PlatformService) StartImpersonation(ctx context.Context, req *hajjv1.StartImpersonationRequest, ip, userAgent string) (*hajjv1.StartImpersonationResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) || len(strings.TrimSpace(req.Reason)) < 10 ||
		len(strings.TrimSpace(req.IdempotencyKey)) < 8 {
		return nil, serviceError("PlatformService.StartImpersonation", apperror.ErrValidation)
	}
	if s.impersonationRepository == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("impersonasi tidak aktif di server ini"))
	}

	operator, err := s.platformRepository.GetTenantDetail(ctx, req.OperatorId)
	if errors.Is(err, repository.ErrTenantNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("travel tidak ditemukan"))
	}
	if err != nil {
		return nil, serviceError("PlatformService.StartImpersonation", err)
	}

	session, token, err := s.impersonationRepository.Start(ctx, repository.StartImpersonation{
		AdminUserID: userID, OperatorID: req.OperatorId, Reason: req.Reason,
		Minutes: req.Minutes, IP: ip, UserAgent: userAgent,
		IdempotencyKey: req.IdempotencyKey,
	})
	if errors.Is(err, apperror.ErrConflict) {
		// A retry cannot be answered with the first session's token — it was
		// never stored. Saying so is better than opening a second session or
		// pretending this one succeeded.
		return nil, connect.NewError(connect.CodeAlreadyExists,
			errors.New("kunci idempotensi ini sudah dipakai; mulai sesi baru dengan kunci baru"))
	}
	if err != nil {
		return nil, serviceError("PlatformService.StartImpersonation", err)
	}

	// Written after the session exists, so a failed start leaves no entry
	// claiming somebody looked when they did not.
	_ = s.auditRepository.Write(ctx, req.OperatorId, userID, "impersonation_started", "impersonation_session", session.ID,
		fmt.Sprintf("sesi baca-saja sampai %s dari %s: %s",
			session.ExpiresAt.Format("2006-01-02 15:04"), ipOrUnknown(ip), strings.TrimSpace(req.Reason)))

	return &hajjv1.StartImpersonationResponse{
		Token: token, SessionId: session.ID, OperatorName: operator.Operator.Name,
		ExpiresAt: timestamppb.New(session.ExpiresAt),
	}, nil
}

func (s *PlatformService) EndImpersonation(ctx context.Context, req *hajjv1.EndImpersonationRequest) (*hajjv1.EndImpersonationResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Token) == "" || s.impersonationRepository == nil {
		return nil, serviceError("PlatformService.EndImpersonation", apperror.ErrValidation)
	}
	// Resolve first so the audit entry can name the tenant. A token that is
	// already dead ends quietly: closing a session the panel has forgotten
	// about must not fail.
	session, resolveErr := s.impersonationRepository.Resolve(ctx, req.Token)
	if err := s.impersonationRepository.End(ctx, req.Token, "ditutup oleh admin"); err != nil {
		return nil, serviceError("PlatformService.EndImpersonation", err)
	}
	if resolveErr == nil {
		_ = s.auditRepository.Write(ctx, session.OperatorID, userID, "impersonation_ended", "impersonation_session", session.ID,
			"sesi baca-saja ditutup lebih awal")
	}
	return &hajjv1.EndImpersonationResponse{}, nil
}

func (s *PlatformService) ListImpersonations(ctx context.Context, req *hajjv1.ListImpersonationsRequest) (*hajjv1.ListImpersonationsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) || s.impersonationRepository == nil {
		return nil, serviceError("PlatformService.ListImpersonations", apperror.ErrValidation)
	}
	sessions, err := s.impersonationRepository.ListForOperator(ctx, req.OperatorId, req.Limit)
	if err != nil {
		return nil, serviceError("PlatformService.ListImpersonations", err)
	}
	response := &hajjv1.ListImpersonationsResponse{Sessions: make([]*hajjv1.ImpersonationRow, 0, len(sessions))}
	for _, session := range sessions {
		row := &hajjv1.ImpersonationRow{
			Id: session.ID, Admin: session.OperatorName, Reason: session.Reason, Ip: session.IP,
			StartedAt: timestamppb.New(session.StartedAt), ExpiresAt: timestamppb.New(session.ExpiresAt),
			EndedReason: session.EndedReason,
		}
		if session.EndedAt != nil {
			row.EndedAt = timestamppb.New(*session.EndedAt)
		}
		response.Sessions = append(response.Sessions, row)
	}
	return response, nil
}

func ipOrUnknown(ip string) string {
	if strings.TrimSpace(ip) == "" {
		return "alamat tidak diketahui"
	}
	return ip
}
