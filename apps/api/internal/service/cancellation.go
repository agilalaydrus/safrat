package service

import (
	"context"
	"math"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CancellationService struct {
	operatorRepository     *repository.OperatorRepository
	pilgrimRepository      *repository.PilgrimRepository
	seasonRepository       *repository.SeasonRepository
	cancellationRepository *repository.CancellationRepository
	waitlistRepository     *repository.WaitlistRepository
	auditRepository        *repository.AuditRepository
}

func NewCancellationService(
	operators *repository.OperatorRepository,
	pilgrims *repository.PilgrimRepository,
	seasons *repository.SeasonRepository,
	cancellations *repository.CancellationRepository,
	waitlist *repository.WaitlistRepository,
	audit *repository.AuditRepository,
) *CancellationService {
	return &CancellationService{
		operatorRepository: operators, pilgrimRepository: pilgrims, seasonRepository: seasons,
		cancellationRepository: cancellations, waitlistRepository: waitlist, auditRepository: audit,
	}
}

func (s *CancellationService) SetPolicy(ctx context.Context, authenticatedOrgID string, req *hajjv1.SetCancellationPolicyRequest) (*hajjv1.CancellationPolicy, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("CancellationService.SetPolicy", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CancellationService.SetPolicy", err)
	}
	policy, err := s.cancellationRepository.CreatePolicy(ctx, operator.ID, req.SeasonId, req.Name, req.MinDays, req.RefundPct, req.SortOrder)
	if err != nil {
		return nil, serviceError("CancellationService.SetPolicy", err)
	}
	return policyMessage(policy), nil
}

func (s *CancellationService) ListPolicies(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListCancellationPoliciesRequest) (*hajjv1.ListCancellationPoliciesResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("CancellationService.ListPolicies", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CancellationService.ListPolicies", err)
	}
	policies, err := s.cancellationRepository.ListPolicies(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("CancellationService.ListPolicies", err)
	}
	result := &hajjv1.ListCancellationPoliciesResponse{Policies: make([]*hajjv1.CancellationPolicy, 0, len(policies))}
	for _, policy := range policies {
		result.Policies = append(result.Policies, policyMessage(policy))
	}
	return result, nil
}

func (s *CancellationService) DeletePolicy(ctx context.Context, authenticatedOrgID string, req *hajjv1.DeleteCancellationPolicyRequest) (*hajjv1.DeleteCancellationPolicyResponse, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("CancellationService.DeletePolicy", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CancellationService.DeletePolicy", err)
	}
	if err := s.cancellationRepository.DeletePolicy(ctx, operator.ID, req.Id); err != nil {
		return nil, serviceError("CancellationService.DeletePolicy", err)
	}
	return &hajjv1.DeleteCancellationPolicyResponse{}, nil
}

// PreviewCancellation is read-only — it computes what ConfirmCancellation
// would do without writing anything, so the operator sees the refund
// before committing to an irreversible action.
func (s *CancellationService) PreviewCancellation(ctx context.Context, authenticatedOrgID string, req *hajjv1.PreviewCancellationRequest) (*hajjv1.CancellationPreview, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("CancellationService.PreviewCancellation", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CancellationService.PreviewCancellation", err)
	}
	preview, err := s.computePreview(ctx, operator.ID, req.PilgrimId)
	if err != nil {
		return nil, serviceError("CancellationService.PreviewCancellation", err)
	}
	return &hajjv1.CancellationPreview{
		PilgrimId: preview.PilgrimID, PilgrimName: preview.PilgrimName, DaysBefore: preview.DaysBefore,
		RefundPct: preview.RefundPct, TotalPaidIdr: preview.TotalPaidIDR, RefundAmountIdr: preview.RefundAmountIDR,
		PolicyName: preview.PolicyName,
	}, nil
}

func (s *CancellationService) computePreview(ctx context.Context, operatorID, pilgrimID string) (*domain.CancellationPreview, error) {
	pilgrim, err := s.pilgrimRepository.Get(ctx, operatorID, pilgrimID)
	if err != nil {
		return nil, err
	}
	if pilgrim.IsSubstituted || pilgrim.Status == "CANCELLED" {
		return nil, apperror.ErrFailedPrecondition
	}
	season, err := s.seasonRepository.GetByID(ctx, operatorID, pilgrim.SeasonID)
	if err != nil {
		return nil, err
	}
	daysBefore := daysUntil(season.StartDate)
	totalPaid, err := s.cancellationRepository.GetPaidTotal(ctx, pilgrimID)
	if err != nil {
		return nil, err
	}
	policy, err := s.cancellationRepository.MatchPolicy(ctx, pilgrim.SeasonID, daysBefore)
	if err != nil {
		return nil, err
	}
	preview := &domain.CancellationPreview{PilgrimID: pilgrimID, PilgrimName: pilgrim.FullName, DaysBefore: daysBefore, TotalPaidIDR: totalPaid}
	if policy != nil {
		preview.RefundPct = policy.RefundPct
		preview.PolicyName = policy.Name
	}
	preview.RefundAmountIDR = int64(math.Round(float64(totalPaid) * preview.RefundPct / 100))
	return preview, nil
}

// ConfirmCancellation recomputes every number itself (see computePreview)
// instead of trusting whatever the client last saw in a preview response,
// then hands off to the repository's atomic transaction.
func (s *CancellationService) ConfirmCancellation(ctx context.Context, authenticatedOrgID string, req *hajjv1.ConfirmCancellationRequest) (*hajjv1.PilgrimCancellation, error) {
	if req == nil || !isUUID(req.PilgrimId) || req.Reason == "" {
		return nil, serviceError("CancellationService.ConfirmCancellation", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CancellationService.ConfirmCancellation", err)
	}
	pilgrim, err := s.pilgrimRepository.Get(ctx, operator.ID, req.PilgrimId)
	if err != nil {
		return nil, serviceError("CancellationService.ConfirmCancellation", err)
	}
	preview, err := s.computePreview(ctx, operator.ID, req.PilgrimId)
	if err != nil {
		return nil, serviceError("CancellationService.ConfirmCancellation", err)
	}
	var policyID string
	policy, err := s.cancellationRepository.MatchPolicy(ctx, pilgrim.SeasonID, preview.DaysBefore)
	if err != nil {
		return nil, serviceError("CancellationService.ConfirmCancellation", err)
	}
	if policy != nil {
		policyID = policy.ID
	}
	cancelledBy := middleware.UserIDFromCtx(ctx)
	cancellation, err := s.cancellationRepository.ConfirmCancellation(
		ctx, operator.ID, req.PilgrimId, pilgrim.SeasonID, req.Reason, cancelledBy,
		preview.DaysBefore, preview.RefundPct, preview.RefundAmountIDR, preview.TotalPaidIDR, policyID,
	)
	if err != nil {
		return nil, serviceError("CancellationService.ConfirmCancellation", err)
	}
	_ = s.auditRepository.Write(ctx, operator.ID, cancelledBy, "pilgrim_cancelled", "pilgrim", req.PilgrimId, req.Reason)

	// Best-effort — a promotion failure here shouldn't roll back an
	// already-committed cancellation. The 1m worker sweep (waitlist
	// expiry) also promotes, so a missed promotion here isn't permanent.
	go func() {
		bg := context.Background()
		_, _ = s.waitlistRepository.PromoteNextWaiting(bg, operator.ID, pilgrim.SeasonID)
	}()

	cancellation.PilgrimName = pilgrim.FullName
	return cancellationMessage(cancellation), nil
}

func (s *CancellationService) ListCancellations(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListCancellationsRequest) (*hajjv1.ListCancellationsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("CancellationService.ListCancellations", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CancellationService.ListCancellations", err)
	}
	cancellations, err := s.cancellationRepository.ListCancellations(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("CancellationService.ListCancellations", err)
	}
	result := &hajjv1.ListCancellationsResponse{Cancellations: make([]*hajjv1.PilgrimCancellation, 0, len(cancellations))}
	for _, cancellation := range cancellations {
		result.Cancellations = append(result.Cancellations, cancellationMessage(cancellation))
	}
	return result, nil
}

// daysUntil floors at 0 — a departure date in the past (or today) is
// treated as "0 days before", matching the strictest refund tier.
func daysUntil(target time.Time) int32 {
	days := int32(math.Floor(time.Until(target).Hours() / 24))
	if days < 0 {
		return 0
	}
	return days
}

func policyMessage(value *domain.CancellationPolicy) *hajjv1.CancellationPolicy {
	return &hajjv1.CancellationPolicy{
		Id: value.ID, SeasonId: value.SeasonID, Name: value.Name,
		MinDays: value.MinDays, RefundPct: value.RefundPct, SortOrder: value.SortOrder,
	}
}

func cancellationMessage(value *domain.PilgrimCancellation) *hajjv1.PilgrimCancellation {
	return &hajjv1.PilgrimCancellation{
		Id: value.ID, PilgrimId: value.PilgrimID, PilgrimName: value.PilgrimName,
		RefundPct: value.RefundPct, RefundAmountIdr: value.RefundAmountIDR, TotalPaidIdr: value.TotalPaidIDR,
		Reason: value.Reason, CancelledBy: value.CancelledBy, CancelledAt: timestamppb.New(value.CancelledAt),
	}
}
