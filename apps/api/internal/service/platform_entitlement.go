package service

import (
	"context"
	"errors"
	"fmt"
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

func (s *PlatformService) ListSubscriptionInvoices(ctx context.Context, req *hajjv1.ListSubscriptionInvoicesRequest) (*hajjv1.ListSubscriptionInvoicesResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	operatorID := ""
	if req != nil {
		operatorID = strings.TrimSpace(req.OperatorId)
		if operatorID != "" && !isUUID(operatorID) {
			return nil, serviceError("PlatformService.ListSubscriptionInvoices", apperror.ErrValidation)
		}
	}
	limit := int32(0)
	if req != nil {
		limit = req.Limit
	}
	invoices, err := s.subscriptionRepository.ListSubscriptionInvoices(ctx, operatorID, limit)
	if err != nil {
		return nil, serviceError("PlatformService.ListSubscriptionInvoices", err)
	}
	response := &hajjv1.ListSubscriptionInvoicesResponse{Invoices: make([]*hajjv1.SubscriptionInvoiceRow, 0, len(invoices))}
	for _, invoice := range invoices {
		row := &hajjv1.SubscriptionInvoiceRow{
			Id: invoice.ID, OperatorId: invoice.OperatorID, OperatorName: invoice.OperatorName,
			Plan: invoice.Plan, Status: invoice.Status, Channel: invoice.Channel,
			AmountIdr: invoice.AmountIDR, DueAt: timestamppb.New(invoice.DueAt),
			VoidedReason: invoice.VoidedReason, CreatedAt: timestamppb.New(invoice.CreatedAt),
		}
		if invoice.PaidAt != nil {
			row.PaidAt = timestamppb.New(*invoice.PaidAt)
		}
		if invoice.VoidedAt != nil {
			row.VoidedAt = timestamppb.New(*invoice.VoidedAt)
		}
		response.Invoices = append(response.Invoices, row)
	}
	return response, nil
}

const platformBillingLeadTime = 7 * 24 * time.Hour

func (s *PlatformService) PreviewSubscriptionBilling(ctx context.Context) (*hajjv1.PreviewSubscriptionBillingResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	candidates, err := s.subscriptionRepository.ListBillingCandidates(ctx, platformBillingLeadTime)
	if err != nil {
		return nil, serviceError("PlatformService.PreviewSubscriptionBilling", err)
	}
	response := &hajjv1.PreviewSubscriptionBillingResponse{
		Candidates: make([]*hajjv1.SubscriptionBillingCandidate, 0, len(candidates)),
	}
	for _, candidate := range candidates {
		response.Candidates = append(response.Candidates, &hajjv1.SubscriptionBillingCandidate{
			OperatorId: candidate.OperatorID, OperatorName: candidate.OperatorName,
			Plan: candidate.Plan, PeriodStart: timestamppb.New(candidate.PeriodStart),
			PeriodEnd: timestamppb.New(candidate.PeriodEnd), DueAt: timestamppb.New(candidate.DueAt),
			BaseAmountIdr: candidate.BaseAmount,
		})
		response.TotalBaseAmountIdr += candidate.BaseAmount
	}
	return response, nil
}

func (s *PlatformService) IssueSubscriptionBilling(ctx context.Context, req *hajjv1.IssueSubscriptionBillingRequest) (*hajjv1.IssueSubscriptionBillingResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || len(req.Targets) == 0 || len(req.Targets) > 200 {
		return nil, serviceError("PlatformService.IssueSubscriptionBilling", apperror.ErrValidation)
	}
	response := &hajjv1.IssueSubscriptionBillingResponse{
		Results: make([]*hajjv1.SubscriptionBillingResult, 0, len(req.Targets)),
	}
	for _, target := range req.Targets {
		result := &hajjv1.SubscriptionBillingResult{}
		if target != nil {
			result.OperatorId = target.OperatorId
		}
		if target == nil || !isUUID(target.OperatorId) || !validPlan(target.Plan) ||
			target.PeriodStart == nil || target.PeriodStart.CheckValid() != nil || target.ExpectedBaseAmountIdr <= 0 {
			result.ErrorCode = "INVALID_TARGET"
			result.Message = "Data pratinjau tidak valid. Muat ulang siklus tagihan."
			response.FailedCount++
			response.Results = append(response.Results, result)
			continue
		}
		invoice, operatorName, created, issueErr := s.subscriptionRepository.IssueBillingPeriod(ctx,
			target.OperatorId, target.Plan, target.PeriodStart.AsTime(), target.ExpectedBaseAmountIdr, userID)
		result.OperatorName = operatorName
		if issueErr != nil {
			result.ErrorCode, result.Message = billingIssueExplanation(issueErr)
			response.FailedCount++
		} else {
			result.InvoiceId = invoice.ID
			result.AmountIdr = invoice.Amount
			result.Issued = created
			result.AlreadyIssued = !created
			if created {
				result.Message = "Invoice berhasil diterbitkan."
				response.IssuedCount++
			} else {
				result.Message = "Invoice periode ini sudah pernah diterbitkan; tidak dibuat ulang."
			}
		}
		response.Results = append(response.Results, result)
	}
	return response, nil
}

func billingIssueExplanation(err error) (string, string) {
	switch {
	case errors.Is(err, repository.ErrBillingPreviewChanged):
		return "PREVIEW_CHANGED", "Paket, periode, atau harga berubah sejak pratinjau. Muat ulang sebelum menerbitkan."
	case errors.Is(err, repository.ErrPendingInvoice):
		return "PENDING_INVOICE", "Travel masih memiliki invoice tertunda untuk periode lain."
	case errors.Is(err, repository.ErrTransferAmountUnavailable):
		return "TRANSFER_AMOUNT_UNAVAILABLE", "Nominal transfer unik tidak tersedia. Coba lagi pada siklus berikutnya."
	default:
		return "INTERNAL_ERROR", "Invoice gagal diterbitkan. Periksa log server dengan ID travel ini."
	}
}

func (s *PlatformService) GetSubscriptionBillingSettings(ctx context.Context) (*hajjv1.GetSubscriptionBillingSettingsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	settings, err := s.subscriptionRepository.Settings(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.GetSubscriptionBillingSettings", err)
	}
	response := &hajjv1.GetSubscriptionBillingSettingsResponse{
		DefaultGracePeriodDays: int32(settings.GracePeriodDays),
		SuspendAfterDays:       int32(settings.SuspendAfterDays),
		TrialDays:              int32(settings.TrialDays),
		DunningDays:            make([]int32, 0, len(settings.ReminderDays)),
	}
	for _, day := range settings.ReminderDays {
		response.DunningDays = append(response.DunningDays, int32(day))
	}
	return response, nil
}

func (s *PlatformService) SetTrialDays(ctx context.Context, req *hajjv1.SetTrialDaysRequest) (*hajjv1.SetTrialDaysResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || req.TrialDays < 1 || req.TrialDays > 90 || strings.TrimSpace(req.Reason) == "" ||
		!strings.EqualFold(strings.TrimSpace(req.Confirmation), "TRIAL") || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("PlatformService.SetTrialDays", apperror.ErrValidation)
	}
	days, err := s.subscriptionRepository.SetTrialDays(ctx, repository.TrialDaysChange{
		Days: req.TrialDays, Reason: strings.TrimSpace(req.Reason),
		Confirmation: strings.TrimSpace(req.Confirmation), ActorUserID: userID,
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		return nil, serviceError("PlatformService.SetTrialDays", err)
	}
	return &hajjv1.SetTrialDaysResponse{TrialDays: days}, nil
}

func (s *PlatformService) SetSubscriptionGracePeriod(ctx context.Context, req *hajjv1.SetSubscriptionGracePeriodRequest) (*hajjv1.SetSubscriptionGracePeriodResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Reason) == "" || strings.TrimSpace(req.Confirmation) == "" ||
		strings.TrimSpace(req.IdempotencyKey) == "" || (req.OperatorId != "" && !isUUID(req.OperatorId)) ||
		(req.OperatorId == "" && (req.UsePlatformDefault || req.GracePeriodDays == nil)) ||
		(req.OperatorId != "" && ((req.UsePlatformDefault && req.GracePeriodDays != nil) || (!req.UsePlatformDefault && req.GracePeriodDays == nil))) {
		return nil, serviceError("PlatformService.SetSubscriptionGracePeriod", apperror.ErrValidation)
	}
	result, err := s.subscriptionRepository.SetGracePeriod(ctx, repository.GracePeriodChange{
		OperatorID: strings.TrimSpace(req.OperatorId), Days: req.GracePeriodDays,
		UseDefault: req.UsePlatformDefault, Reason: strings.TrimSpace(req.Reason),
		Confirmation: strings.TrimSpace(req.Confirmation), ActorUserID: userID,
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		return nil, serviceError("PlatformService.SetSubscriptionGracePeriod", err)
	}
	return &hajjv1.SetSubscriptionGracePeriodResponse{
		OperatorId: result.OperatorID, EffectiveGracePeriodDays: result.EffectiveDays,
		OverrideGracePeriodDays: result.OverrideDays,
	}, nil
}

func (s *PlatformService) PreviewSubscriptionPlanChange(ctx context.Context, req *hajjv1.PreviewSubscriptionPlanChangeRequest) (*hajjv1.PreviewSubscriptionPlanChangeResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) || !validPlan(req.NewPlan) {
		return nil, serviceError("PlatformService.PreviewSubscriptionPlanChange", apperror.ErrValidation)
	}
	preview, err := s.subscriptionRepository.PreviewPlanChange(ctx, req.OperatorId, req.NewPlan)
	if err != nil {
		if errors.Is(err, repository.ErrNoProration) {
			err = fmt.Errorf("%w: langganan tidak memiliki waktu berbayar yang dapat diprorata", apperror.ErrFailedPrecondition)
		}
		return nil, serviceError("PlatformService.PreviewSubscriptionPlanChange", err)
	}
	return &hajjv1.PreviewSubscriptionPlanChangeResponse{
		OperatorId: preview.OperatorID, OperatorName: preview.OperatorName,
		CurrentPlan: preview.CurrentPlan, NewPlan: preview.NewPlan,
		CurrentMonthlyIdr: preview.CurrentMonthly, NewMonthlyIdr: preview.NewMonthly,
		RemainingDays:     (preview.RemainingSeconds + 86399) / 86400,
		BillingPeriodDays: int32(repository.BillingPeriodDays), AdjustmentIdr: preview.AdjustmentIDR,
		CurrentCreditBalanceIdr: preview.CreditBalanceIDR,
		AccessUntil:             timestamppb.New(preview.AccessUntil),
	}, nil
}

func (s *PlatformService) ApplySubscriptionPlanChange(ctx context.Context, req *hajjv1.ApplySubscriptionPlanChangeRequest) (*hajjv1.ApplySubscriptionPlanChangeResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) || !validPlan(req.NewPlan) || strings.TrimSpace(req.Reason) == "" ||
		strings.TrimSpace(req.Confirmation) == "" || strings.TrimSpace(req.IdempotencyKey) == "" || req.ExpectedAdjustmentIdr == 0 {
		return nil, serviceError("PlatformService.ApplySubscriptionPlanChange", apperror.ErrValidation)
	}
	result, err := s.subscriptionRepository.ApplyPlanChange(ctx, repository.PlanChange{
		OperatorID: req.OperatorId, NewPlan: req.NewPlan, ExpectedAdjustment: req.ExpectedAdjustmentIdr,
		Reason: strings.TrimSpace(req.Reason), Confirmation: strings.TrimSpace(req.Confirmation),
		ActorUserID: userID, IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		if errors.Is(err, repository.ErrNoProration) || errors.Is(err, repository.ErrPendingInvoice) ||
			errors.Is(err, repository.ErrBillingPreviewChanged) || errors.Is(err, repository.ErrTransferAmountUnavailable) {
			err = fmt.Errorf("%w: %v", apperror.ErrFailedPrecondition, err)
		}
		return nil, serviceError("PlatformService.ApplySubscriptionPlanChange", err)
	}
	return &hajjv1.ApplySubscriptionPlanChangeResponse{
		AdjustmentId: result.AdjustmentID, OperatorId: result.OperatorID,
		FromPlan: result.FromPlan, ToPlan: result.ToPlan, AdjustmentIdr: result.AdjustmentIDR,
		InvoiceId: result.InvoiceID, InvoiceAmountIdr: result.InvoiceAmountIDR,
		Status: result.Status, CreditBalanceIdr: result.CreditBalanceIDR,
	}, nil
}

func (s *PlatformService) VoidSubscriptionInvoice(ctx context.Context, req *hajjv1.VoidSubscriptionInvoiceRequest) (*hajjv1.VoidSubscriptionInvoiceResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.InvoiceId) || strings.TrimSpace(req.Reason) == "" {
		return nil, serviceError("PlatformService.VoidSubscriptionInvoice", apperror.ErrValidation)
	}
	if err := s.subscriptionRepository.VoidInvoice(ctx, req.InvoiceId, strings.TrimSpace(req.Reason), userID); err != nil {
		return nil, serviceError("PlatformService.VoidSubscriptionInvoice", err)
	}
	return &hajjv1.VoidSubscriptionInvoiceResponse{}, nil
}

func (s *PlatformService) ListUsage(ctx context.Context) (*hajjv1.ListUsageResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.subscriptionRepository.ListUsage(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.ListUsage", err)
	}
	response := &hajjv1.ListUsageResponse{Rows: make([]*hajjv1.UsageRow, 0, len(rows))}
	for _, row := range rows {
		message := &hajjv1.UsageRow{
			OperatorId: row.OperatorID, OperatorName: row.OperatorName, Plan: row.Plan,
			Metric: row.Metric, Value: row.Value, ComputedAt: timestamppb.New(row.ComputedAt),
		}
		if row.Limit != nil {
			message.Limit = row.Limit
		}
		response.Rows = append(response.Rows, message)
	}
	return response, nil
}
