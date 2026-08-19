package service

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
)

type IdentityService struct {
	identityRepository *repository.IdentityRepository
}

func NewIdentityService(identity *repository.IdentityRepository) *IdentityService {
	return &IdentityService{identityRepository: identity}
}

// GetMyAccess runs through the session-only (not org-scoped) auth lane —
// same reasoning as PilgrimAppService.LinkGoogleAccount: a pure leader or
// pilgrim identity is never an organization member, so the default
// org-required interceptor lane would reject them before this even runs.
func (s *IdentityService) GetMyAccess(ctx context.Context, _ *hajjv1.GetMyAccessRequest) (*hajjv1.MyAccess, error) {
	userID := middleware.UserIDFromCtx(ctx)
	if userID == "" {
		return nil, serviceError("IdentityService.GetMyAccess", apperror.ErrUnauthorized)
	}
	access, err := s.identityRepository.GetMyAccess(ctx, userID)
	if err != nil {
		return nil, serviceError("IdentityService.GetMyAccess", err)
	}
	result := &hajjv1.MyAccess{
		IsOrgMember:  access.IsOrgMember,
		OrgRole:      access.OrgRole,
		OperatorId:   access.OperatorID,
		OperatorName: access.OperatorName,
		LeaderGroups: make([]*hajjv1.LeaderGroupSummary, 0, len(access.LeaderGroups)),
	}
	for _, g := range access.LeaderGroups {
		result.LeaderGroups = append(result.LeaderGroups, &hajjv1.LeaderGroupSummary{Id: g.ID, Name: g.Name})
	}
	if access.LinkedPilgrim != nil {
		result.LinkedPilgrim = &hajjv1.PilgrimSummary{Id: access.LinkedPilgrim.ID, AppAccessCode: access.LinkedPilgrim.AppAccessCode, FullName: access.LinkedPilgrim.FullName}
	}
	if access.LinkedAgent != nil {
		result.LinkedAgent = &hajjv1.AgentSummary{Id: access.LinkedAgent.ID, Name: access.LinkedAgent.Name, ReferralCode: access.LinkedAgent.ReferralCode, IsActive: access.LinkedAgent.IsActive}
	}
	return result, nil
}

// InvalidateMyAccess lets the caller drop their own cached MyAccess ahead
// of the TTL — see the proto comment for why this is needed at all.
func (s *IdentityService) InvalidateMyAccess(ctx context.Context, _ *hajjv1.InvalidateMyAccessRequest) (*hajjv1.InvalidateMyAccessResponse, error) {
	userID := middleware.UserIDFromCtx(ctx)
	if userID == "" {
		return nil, serviceError("IdentityService.InvalidateMyAccess", apperror.ErrUnauthorized)
	}
	s.identityRepository.InvalidateAccessCache(userID)
	return &hajjv1.InvalidateMyAccessResponse{}, nil
}
