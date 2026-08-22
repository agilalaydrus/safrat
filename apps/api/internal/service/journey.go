package service

import (
	"errors"
	"fmt"
	"strings"

	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type JourneyService struct {
	operatorRepository *repository.OperatorRepository
	journeyRepository  *repository.JourneyRepository
	auditRepository    *repository.AuditRepository
}

func NewJourneyService(operators *repository.OperatorRepository, journeys *repository.JourneyRepository, audit *repository.AuditRepository) *JourneyService {
	return &JourneyService{operatorRepository: operators, journeyRepository: journeys, auditRepository: audit}
}

func (s *JourneyService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "pilgrim_journey", entityID, message)
}

// currentStatus defaults to REGISTERED (not an error) when a pilgrim has no
// journey row yet — every pilgrim that existed at migration-069 was
// backfilled, but one created afterward starts without a row until its
// first transition.
func (s *JourneyService) currentStatus(ctx context.Context, operatorID, pilgrimID string) string {
	existing, err := s.journeyRepository.GetStatus(ctx, operatorID, pilgrimID)
	if err != nil || existing == nil {
		return "REGISTERED"
	}
	return existing.Status
}

func (s *JourneyService) UpdatePilgrimStatus(ctx context.Context, orgID string, req *hajjv1.UpdatePilgrimStatusRequest) (*hajjv1.PilgrimJourneyStatus, error) {
	if req == nil || strings.TrimSpace(req.PilgrimId) == "" || strings.TrimSpace(req.Status) == "" {
		return nil, serviceError("JourneyService.UpdatePilgrimStatus", apperror.ErrValidation)
	}
	targetIdx := domain.JourneyStatusIndex(req.Status)
	if targetIdx == -1 {
		return nil, serviceError("JourneyService.UpdatePilgrimStatus", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("JourneyService.UpdatePilgrimStatus", err)
	}
	from := s.currentStatus(ctx, op.ID, req.PilgrimId)
	// Forward-only. Not "exactly +1 step" — Umrah's shorter lifecycle
	// legitimately skips several Hajj-only steps (Arafah/Muzdalifah/Mina),
	// so index-monotonic is the rule, not adjacency.
	if targetIdx <= domain.JourneyStatusIndex(from) {
		return nil, serviceError("JourneyService.UpdatePilgrimStatus", errors.New("status hanya bisa maju"))
	}
	updated, err := s.journeyRepository.UpdateStatus(ctx, op.ID, req.PilgrimId, from, req.Status, middleware.UserIDFromCtx(ctx), req.Notes)
	if err != nil {
		return nil, serviceError("JourneyService.UpdatePilgrimStatus", err)
	}
	s.logActivity(ctx, op.ID, "journey_status_updated", req.PilgrimId, fmt.Sprintf("Status perjalanan: %s -> %s", from, req.Status))
	return journeyStatusMessage(updated), nil
}

// BulkUpdateStatus is the kloter-wide cascade action — called directly by
// operators/Tour Leaders, and internally (best-effort) from
// KloterService.UpdateKloterStatus and GroupService.UpdateGroupCity. Sets
// every pilgrim in the kloter to the target status unconditionally (an
// authoritative kloter-wide event may legitimately jump several pilgrims
// past where they individually were), skipping only pilgrims already at
// or past the target.
func (s *JourneyService) BulkUpdateStatus(ctx context.Context, orgID string, req *hajjv1.BulkUpdateStatusRequest) (*hajjv1.BulkUpdateStatusResponse, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" || strings.TrimSpace(req.Status) == "" {
		return nil, serviceError("JourneyService.BulkUpdateStatus", apperror.ErrValidation)
	}
	if domain.JourneyStatusIndex(req.Status) == -1 {
		return nil, serviceError("JourneyService.BulkUpdateStatus", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("JourneyService.BulkUpdateStatus", err)
	}
	count, err := s.bulkUpdateStatus(ctx, op.ID, req.KloterId, req.Status, req.Notes)
	if err != nil {
		return nil, serviceError("JourneyService.BulkUpdateStatus", err)
	}
	return &hajjv1.BulkUpdateStatusResponse{UpdatedCount: count}, nil
}

// bulkUpdateStatus is the internal, org-independent entry point reused by
// KloterService/GroupService cascades — they already have op.ID and a
// user id from context, no need to re-resolve the operator.
func (s *JourneyService) bulkUpdateStatus(ctx context.Context, operatorID, kloterID, status, notes string) (int32, error) {
	statuses, err := s.journeyRepository.ListByKloter(ctx, operatorID, kloterID)
	if err != nil {
		return 0, err
	}
	targetIdx := domain.JourneyStatusIndex(status)
	userID := middleware.UserIDFromCtx(ctx)
	var count int32
	for pilgrimID, from := range statuses {
		if domain.JourneyStatusIndex(from) >= targetIdx {
			continue
		}
		if _, err := s.journeyRepository.UpdateStatus(ctx, operatorID, pilgrimID, from, status, userID, notes); err != nil {
			return count, err
		}
		count++
	}
	if count > 0 {
		s.logActivity(ctx, operatorID, "journey_status_bulk_updated", kloterID, fmt.Sprintf("%d jamaah diperbarui ke status %s", count, status))
	}
	return count, nil
}

// BulkUpdateForGroup is the group-scoped cascade used by
// GroupService.UpdateGroupCity — same semantics as bulkUpdateStatus but
// resolving pilgrim ids from a group instead of a kloter.
func (s *JourneyService) BulkUpdateForGroup(ctx context.Context, operatorID, groupID, status, notes string) (int32, error) {
	targetIdx := domain.JourneyStatusIndex(status)
	if targetIdx == -1 {
		return 0, apperror.ErrValidation
	}
	pilgrimIDs, err := s.journeyRepository.ListPilgrimIDsByGroup(ctx, operatorID, groupID)
	if err != nil {
		return 0, err
	}
	userID := middleware.UserIDFromCtx(ctx)
	var count int32
	for _, pilgrimID := range pilgrimIDs {
		from := s.currentStatus(ctx, operatorID, pilgrimID)
		if domain.JourneyStatusIndex(from) >= targetIdx {
			continue
		}
		if _, err := s.journeyRepository.UpdateStatus(ctx, operatorID, pilgrimID, from, status, userID, notes); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *JourneyService) GetPilgrimStatus(ctx context.Context, orgID string, req *hajjv1.GetPilgrimStatusRequest) (*hajjv1.PilgrimJourneyStatus, error) {
	if req == nil || strings.TrimSpace(req.PilgrimId) == "" {
		return nil, serviceError("JourneyService.GetPilgrimStatus", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("JourneyService.GetPilgrimStatus", err)
	}
	existing, err := s.journeyRepository.GetStatus(ctx, op.ID, req.PilgrimId)
	if err != nil {
		return &hajjv1.PilgrimJourneyStatus{PilgrimId: req.PilgrimId, Status: "REGISTERED"}, nil
	}
	return journeyStatusMessage(existing), nil
}

func (s *JourneyService) GetKloterJourneyOverview(ctx context.Context, orgID string, req *hajjv1.GetKloterJourneyOverviewRequest) (*hajjv1.KloterJourneyOverview, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("JourneyService.GetKloterJourneyOverview", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("JourneyService.GetKloterJourneyOverview", err)
	}
	counts, err := s.journeyRepository.CountByKloter(ctx, op.ID, req.KloterId)
	if err != nil {
		return nil, serviceError("JourneyService.GetKloterJourneyOverview", err)
	}
	return &hajjv1.KloterJourneyOverview{KloterId: req.KloterId, StatusCounts: counts}, nil
}

func journeyStatusMessage(v *domain.PilgrimJourneyStatus) *hajjv1.PilgrimJourneyStatus {
	msg := &hajjv1.PilgrimJourneyStatus{PilgrimId: v.PilgrimID, Status: v.Status, UpdatedByName: v.UpdatedByName, Notes: v.Notes}
	if !v.UpdatedAt.IsZero() {
		msg.UpdatedAt = timestamppb.New(v.UpdatedAt)
	}
	return msg
}
