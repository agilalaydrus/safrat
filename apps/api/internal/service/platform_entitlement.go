package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var featureFlagName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func (s *PlatformService) ListPlanLimits(ctx context.Context) (*hajjv1.ListPlanLimitsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	limits, err := s.platformRepository.ListPlanLimits(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.ListPlanLimits", err)
	}
	response := &hajjv1.ListPlanLimitsResponse{Limits: make([]*hajjv1.PlatformPlanLimit, 0, len(limits))}
	for _, limit := range limits {
		response.Limits = append(response.Limits, platformPlanLimitMessage(limit))
	}
	return response, nil
}

func (s *PlatformService) PreviewPlanLimitChange(ctx context.Context, req *hajjv1.PreviewPlanLimitChangeRequest) (*hajjv1.PreviewPlanLimitChangeResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, serviceError("PlatformService.PreviewPlanLimitChange", apperror.ErrValidation)
	}
	change, err := s.planLimitChange(ctx, req.GetPlan(), req.GetMaxPilgrims(), req.GetMaxBranches(), req.GetFeatureFlags())
	if err != nil {
		return nil, serviceError("PlatformService.PreviewPlanLimitChange", err)
	}
	affected, err := s.platformRepository.PreviewPlanLimitChange(ctx, change)
	if err != nil {
		return nil, serviceError("PlatformService.PreviewPlanLimitChange", err)
	}
	response := &hajjv1.PreviewPlanLimitChangeResponse{AffectedTenants: make([]*hajjv1.AffectedTenant, 0, len(affected))}
	for _, tenant := range affected {
		response.AffectedTenants = append(response.AffectedTenants, affectedTenantMessage(tenant))
	}
	return response, nil
}

func (s *PlatformService) SetPlanLimit(ctx context.Context, req *hajjv1.SetPlanLimitRequest) (*hajjv1.SetPlanLimitResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !strings.EqualFold(strings.TrimSpace(req.Confirmation), strings.TrimSpace(req.Plan)) ||
		strings.TrimSpace(req.Reason) == "" || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("PlatformService.SetPlanLimit", apperror.ErrValidation)
	}
	change, err := s.planLimitChange(ctx, req.Plan, req.MaxPilgrims, req.MaxBranches, req.FeatureFlags)
	if err != nil {
		return nil, serviceError("PlatformService.SetPlanLimit", err)
	}
	change.Reason = strings.TrimSpace(req.Reason)
	change.ActorUserID = userID
	change.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	change.GrandfatherAffected = req.GrandfatherAffected
	limit, count, err := s.platformRepository.SetPlanLimit(ctx, change)
	if err != nil {
		return nil, serviceError("PlatformService.SetPlanLimit", err)
	}
	return &hajjv1.SetPlanLimitResponse{Limit: platformPlanLimitMessage(limit), GrandfatheredTenants: count}, nil
}

func (s *PlatformService) ListPlanOverrides(ctx context.Context, req *hajjv1.ListPlanOverridesRequest) (*hajjv1.ListPlanOverridesResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	overrides, err := s.platformRepository.ListPlanOverrides(ctx, req != nil && req.IncludeExpired)
	if err != nil {
		return nil, serviceError("PlatformService.ListPlanOverrides", err)
	}
	response := &hajjv1.ListPlanOverridesResponse{Overrides: make([]*hajjv1.PlatformPlanOverride, 0, len(overrides))}
	for _, override := range overrides {
		response.Overrides = append(response.Overrides, platformPlanOverrideMessage(override))
	}
	return response, nil
}

func (s *PlatformService) SetPlanOverride(ctx context.Context, req *hajjv1.SetPlanOverrideRequest) (*hajjv1.SetPlanOverrideResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) || strings.TrimSpace(req.Note) == "" ||
		strings.TrimSpace(req.IdempotencyKey) == "" ||
		(req.MaxPilgrims == nil && req.MaxBranches == nil && len(req.FeatureFlagOverrides) == 0) ||
		!validFeatureFlags(req.FeatureFlagOverrides) {
		return nil, serviceError("PlatformService.SetPlanOverride", apperror.ErrValidation)
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		if err := req.ExpiresAt.CheckValid(); err != nil || !req.ExpiresAt.AsTime().After(time.Now()) {
			return nil, serviceError("PlatformService.SetPlanOverride", apperror.ErrValidation)
		}
		value := req.ExpiresAt.AsTime()
		expiresAt = &value
	}
	change := repository.PlanOverrideChange{
		OperatorID: req.OperatorId, MaxPilgrims: req.MaxPilgrims, MaxBranches: req.MaxBranches,
		FeatureFlagOverrides: cloneFlags(req.FeatureFlagOverrides), Note: strings.TrimSpace(req.Note),
		ExpiresAt: expiresAt, ActorUserID: userID, IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	}
	override, err := s.platformRepository.SetPlanOverride(ctx, change)
	if err != nil {
		return nil, serviceError("PlatformService.SetPlanOverride", err)
	}
	return &hajjv1.SetPlanOverrideResponse{Override: platformPlanOverrideMessage(override)}, nil
}

func (s *PlatformService) DeletePlanOverride(ctx context.Context, req *hajjv1.DeletePlanOverrideRequest) (*hajjv1.DeletePlanOverrideResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) || strings.TrimSpace(req.Reason) == "" || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("PlatformService.DeletePlanOverride", apperror.ErrValidation)
	}
	if err := s.platformRepository.DeletePlanOverride(ctx, req.OperatorId, userID,
		strings.TrimSpace(req.Reason), strings.TrimSpace(req.IdempotencyKey)); err != nil {
		return nil, serviceError("PlatformService.DeletePlanOverride", err)
	}
	return &hajjv1.DeletePlanOverrideResponse{}, nil
}

func (s *PlatformService) planLimitChange(ctx context.Context, plan string, pilgrims, branches *hajjv1.QuotaValue, requestedFlags map[string]bool) (repository.PlanLimitChange, error) {
	if !validPlan(plan) || pilgrims == nil || branches == nil ||
		(pilgrims.Unlimited && pilgrims.Value != 0) || (branches.Unlimited && branches.Value != 0) ||
		!validFeatureFlags(requestedFlags) {
		return repository.PlanLimitChange{}, apperror.ErrValidation
	}
	current, err := s.platformRepository.GetPlanLimit(ctx, plan)
	if err != nil {
		return repository.PlanLimitChange{}, err
	}
	flags := cloneFlags(current.FeatureFlags)
	for key, enabled := range requestedFlags {
		flags[key] = enabled
	}
	change := repository.PlanLimitChange{Plan: plan, FeatureFlags: flags}
	if !pilgrims.Unlimited {
		value := pilgrims.Value
		change.MaxPilgrims = &value
	}
	if !branches.Unlimited {
		value := branches.Value
		change.MaxBranches = &value
	}
	return change, nil
}

func platformPlanLimitMessage(value repository.PlatformPlanLimit) *hajjv1.PlatformPlanLimit {
	return &hajjv1.PlatformPlanLimit{
		Plan: value.Plan, MaxPilgrims: quotaValue(value.MaxPilgrims), MaxBranches: quotaValue(value.MaxBranches),
		FeatureFlags: cloneFlags(value.FeatureFlags), UpdatedAt: timestamppb.New(value.UpdatedAt),
	}
}

func platformPlanOverrideMessage(value repository.PlatformPlanOverride) *hajjv1.PlatformPlanOverride {
	message := &hajjv1.PlatformPlanOverride{
		OperatorId: value.OperatorID, OperatorName: value.OperatorName, Plan: value.Plan,
		MaxPilgrims: value.MaxPilgrims, MaxBranches: value.MaxBranches,
		FeatureFlagOverrides: cloneFlags(value.FeatureFlagOverrides), Note: value.Note,
		UpdatedBy: value.UpdatedBy, UpdatedAt: timestamppb.New(value.UpdatedAt),
	}
	if value.ExpiresAt != nil {
		message.ExpiresAt = timestamppb.New(*value.ExpiresAt)
	}
	return message
}

func affectedTenantMessage(value repository.AffectedPlanTenant) *hajjv1.AffectedTenant {
	return &hajjv1.AffectedTenant{
		OperatorId: value.OperatorID, Name: value.Name, PilgrimCount: value.PilgrimCount,
		ActiveBranchCount: value.ActiveBranchCount, CurrentMaxPilgrims: quotaValue(value.CurrentPilgrimMax),
		CurrentMaxBranches: quotaValue(value.CurrentBranchMax), Reasons: append([]string(nil), value.Reasons...),
	}
}

func quotaValue(value *int32) *hajjv1.QuotaValue {
	if value == nil {
		return &hajjv1.QuotaValue{Unlimited: true}
	}
	return &hajjv1.QuotaValue{Value: *value}
}

func validPlan(plan string) bool {
	return plan == "STARTER" || plan == "GROWTH" || plan == "PRO"
}

func validFeatureFlags(flags map[string]bool) bool {
	for key := range flags {
		if !featureFlagName.MatchString(key) {
			return false
		}
	}
	return true
}

func cloneFlags(flags map[string]bool) map[string]bool {
	copy := make(map[string]bool, len(flags))
	for key, value := range flags {
		copy[key] = value
	}
	return copy
}
