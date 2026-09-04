package service

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SecuritySettingsService struct {
	operatorRepository *repository.OperatorRepository
	repository         *repository.SecuritySettingsRepository
}

func NewSecuritySettingsService(operators *repository.OperatorRepository, repo *repository.SecuritySettingsRepository) *SecuritySettingsService {
	return &SecuritySettingsService{operatorRepository: operators, repository: repo}
}

func (s *SecuritySettingsService) GetSecurityPosture(ctx context.Context, orgID, callerIP string, req *hajjv1.GetSecurityPostureRequest) (*hajjv1.SecurityPosture, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SecuritySettingsService.GetSecurityPosture", err)
	}
	settings, err := s.repository.Get(ctx, op.ID)
	if err != nil {
		return nil, serviceError("SecuritySettingsService.GetSecurityPosture", err)
	}
	return &hajjv1.SecurityPosture{IpAllowlistEnabled: settings.Enabled, IpAllowlistCidrs: settings.CIDRs, YourIp: callerIP}, nil
}

// SetIpAllowlist is where a self-lockout gets refused instead of saved. Every
// CIDR must parse, and when enabling, the caller's own current IP must match
// at least one of them — the settings screen's own request is the only
// available proof that the ranges being saved actually reach the person
// saving them.
func (s *SecuritySettingsService) SetIpAllowlist(ctx context.Context, orgID, userID, callerIP, orgRole string, req *hajjv1.SetIpAllowlistRequest) (*hajjv1.SecurityPosture, error) {
	if req == nil {
		return nil, serviceError("SecuritySettingsService.SetIpAllowlist", apperror.ErrValidation)
	}
	if orgRole != "owner" && orgRole != "admin" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("hanya owner atau admin yang dapat mengubah kebijakan keamanan"))
	}
	cidrs := make([]string, 0, len(req.Cidrs))
	for _, raw := range req.Cidrs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, err := netip.ParsePrefix(trimmed); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("rentang IP tidak valid: %s", trimmed))
		}
		cidrs = append(cidrs, trimmed)
	}
	if req.Enabled {
		if len(cidrs) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("daftar IP tidak boleh kosong saat diaktifkan"))
		}
		if !ipInAnyCIDR(callerIP, cidrs) {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("IP Anda saat ini (%s) tidak termasuk dalam daftar yang akan disimpan — ini akan mengunci Anda sendiri, jadi tidak disimpan", callerIP))
		}
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SecuritySettingsService.SetIpAllowlist", err)
	}
	settings, err := s.repository.Set(ctx, op.ID, req.Enabled, cidrs, userID)
	if err != nil {
		return nil, serviceError("SecuritySettingsService.SetIpAllowlist", err)
	}
	return &hajjv1.SecurityPosture{IpAllowlistEnabled: settings.Enabled, IpAllowlistCidrs: settings.CIDRs, YourIp: callerIP}, nil
}

func ipInAnyCIDR(ip string, cidrs []string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	for _, cidr := range cidrs {
		if prefix, err := netip.ParsePrefix(cidr); err == nil && prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (s *SecuritySettingsService) ListActiveSessions(ctx context.Context, orgID, currentSessionID string, req *hajjv1.ListActiveSessionsRequest) (*hajjv1.ListActiveSessionsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SecuritySettingsService.ListActiveSessions", err)
	}
	sessions, err := s.repository.ListActiveSessions(ctx, op.ID)
	if err != nil {
		return nil, serviceError("SecuritySettingsService.ListActiveSessions", err)
	}
	response := &hajjv1.ListActiveSessionsResponse{}
	for _, session := range sessions {
		response.Sessions = append(response.Sessions, &hajjv1.ActiveSession{
			Id: session.ID, UserName: session.UserName, UserEmail: session.UserEmail,
			IpAddress: session.IPAddress, UserAgent: session.UserAgent, CreatedAt: timestamppb.New(session.CreatedAt),
			IsCurrentSession: session.ID == currentSessionID,
		})
	}
	return response, nil
}

func (s *SecuritySettingsService) RevokeSession(ctx context.Context, orgID string, req *hajjv1.RevokeSessionRequest) error {
	if req == nil || strings.TrimSpace(req.SessionId) == "" {
		return serviceError("SecuritySettingsService.RevokeSession", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("SecuritySettingsService.RevokeSession", err)
	}
	if err := s.repository.RevokeSession(ctx, op.ID, req.SessionId); err != nil {
		return serviceError("SecuritySettingsService.RevokeSession", err)
	}
	return nil
}
