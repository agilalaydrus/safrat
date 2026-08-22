package service

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WaitlistService struct {
	operatorRepository *repository.OperatorRepository
	waitlistRepository *repository.WaitlistRepository
	auditRepository    *repository.AuditRepository
}

func NewWaitlistService(operators *repository.OperatorRepository, waitlist *repository.WaitlistRepository, audit *repository.AuditRepository) *WaitlistService {
	return &WaitlistService{operatorRepository: operators, waitlistRepository: waitlist, auditRepository: audit}
}

// JoinWaitlist is public — operator_id/season_id come from the request
// body with no session, re-validated server-side by the repository
// (capacity check, duplicate email check) before anything is written.
func (s *WaitlistService) JoinWaitlist(ctx context.Context, req *hajjv1.JoinWaitlistRequest) (*hajjv1.JoinWaitlistResponse, error) {
	if req == nil || !isUUID(req.OperatorId) || !isUUID(req.SeasonId) {
		return nil, serviceError("WaitlistService.JoinWaitlist", apperror.ErrValidation)
	}
	entry, isFull, err := s.waitlistRepository.Join(ctx, req.OperatorId, req.SeasonId, req.FullName, req.Email, req.Phone, req.ProductId)
	if err != nil {
		return nil, serviceError("WaitlistService.JoinWaitlist", err)
	}
	if !isFull {
		return &hajjv1.JoinWaitlistResponse{IsFull: false}, nil
	}
	return &hajjv1.JoinWaitlistResponse{Entry: waitlistMessage(entry), IsFull: true, Position: entry.Position}, nil
}

// LeaveWaitlist is public — authenticated by email match only.
func (s *WaitlistService) LeaveWaitlist(ctx context.Context, req *hajjv1.LeaveWaitlistRequest) (*hajjv1.LeaveWaitlistResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("WaitlistService.LeaveWaitlist", apperror.ErrValidation)
	}
	if err := s.waitlistRepository.Leave(ctx, req.SeasonId, req.Email); err != nil {
		return nil, serviceError("WaitlistService.LeaveWaitlist", err)
	}
	return &hajjv1.LeaveWaitlistResponse{}, nil
}

// ConfirmWaitlistSlot is public — a promoted entry confirms their slot by
// id+season+email; the repository query only flips PROMOTED rows whose
// 48h window hasn't lapsed, so a stale confirm attempt fails naturally.
func (s *WaitlistService) ConfirmWaitlistSlot(ctx context.Context, req *hajjv1.ConfirmWaitlistSlotRequest) (*hajjv1.ConfirmWaitlistSlotResponse, error) {
	if req == nil || !isUUID(req.Id) || !isUUID(req.SeasonId) {
		return nil, serviceError("WaitlistService.ConfirmWaitlistSlot", apperror.ErrValidation)
	}
	entry, err := s.waitlistRepository.ConfirmSlot(ctx, req.Id, req.SeasonId, req.Email)
	if err != nil {
		return nil, serviceError("WaitlistService.ConfirmWaitlistSlot", err)
	}
	return &hajjv1.ConfirmWaitlistSlotResponse{Entry: waitlistMessage(entry)}, nil
}

func (s *WaitlistService) ListWaitlist(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListWaitlistRequest) (*hajjv1.ListWaitlistResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("WaitlistService.ListWaitlist", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("WaitlistService.ListWaitlist", err)
	}
	entries, err := s.waitlistRepository.List(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("WaitlistService.ListWaitlist", err)
	}
	result := &hajjv1.ListWaitlistResponse{Entries: make([]*hajjv1.WaitlistEntry, 0, len(entries))}
	for _, entry := range entries {
		result.Entries = append(result.Entries, waitlistMessage(entry))
		if entry.Status == "WAITING" {
			result.TotalWaiting++
		}
	}
	return result, nil
}

func (s *WaitlistService) PromoteFromWaitlist(ctx context.Context, authenticatedOrgID string, req *hajjv1.PromoteFromWaitlistRequest) (*hajjv1.WaitlistEntry, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("WaitlistService.PromoteFromWaitlist", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("WaitlistService.PromoteFromWaitlist", err)
	}
	entry, err := s.waitlistRepository.Promote(ctx, operator.ID, req.Id)
	if err != nil {
		return nil, serviceError("WaitlistService.PromoteFromWaitlist", err)
	}
	_ = s.auditRepository.Write(ctx, operator.ID, middleware.UserIDFromCtx(ctx), "waitlist_promoted", "season_waitlist", entry.ID, entry.FullName)
	return waitlistMessage(entry), nil
}

func (s *WaitlistService) ConfirmWaitlistEntry(ctx context.Context, authenticatedOrgID string, req *hajjv1.ConfirmWaitlistEntryRequest) (*hajjv1.WaitlistEntry, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("WaitlistService.ConfirmWaitlistEntry", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("WaitlistService.ConfirmWaitlistEntry", err)
	}
	entry, err := s.waitlistRepository.AdminConfirm(ctx, operator.ID, req.Id)
	if err != nil {
		return nil, serviceError("WaitlistService.ConfirmWaitlistEntry", err)
	}
	_ = s.auditRepository.Write(ctx, operator.ID, middleware.UserIDFromCtx(ctx), "waitlist_confirmed", "season_waitlist", entry.ID, entry.FullName)
	return waitlistMessage(entry), nil
}

func (s *WaitlistService) RemoveFromWaitlist(ctx context.Context, authenticatedOrgID string, req *hajjv1.RemoveFromWaitlistRequest) (*hajjv1.RemoveFromWaitlistResponse, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("WaitlistService.RemoveFromWaitlist", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("WaitlistService.RemoveFromWaitlist", err)
	}
	if err := s.waitlistRepository.Remove(ctx, operator.ID, req.Id); err != nil {
		return nil, serviceError("WaitlistService.RemoveFromWaitlist", err)
	}
	return &hajjv1.RemoveFromWaitlistResponse{}, nil
}

func waitlistMessage(value *domain.WaitlistEntry) *hajjv1.WaitlistEntry {
	if value == nil {
		return nil
	}
	entry := &hajjv1.WaitlistEntry{
		Id: value.ID, SeasonId: value.SeasonID, FullName: value.FullName, Email: value.Email,
		Phone: value.Phone, ProductId: value.ProductID, Position: value.Position, Status: value.Status,
		CreatedAt: timestamppb.New(value.CreatedAt),
	}
	if value.PromotedAt != nil {
		entry.PromotedAt = timestamppb.New(*value.PromotedAt)
	}
	if value.ExpiresAt != nil {
		entry.ExpiresAt = timestamppb.New(*value.ExpiresAt)
	}
	return entry
}
