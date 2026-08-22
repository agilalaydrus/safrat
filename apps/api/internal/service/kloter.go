package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type KloterService struct {
	operatorRepository *repository.OperatorRepository
	kloterRepository   *repository.KloterRepository
	auditRepository    *repository.AuditRepository
	journeyService     *JourneyService
	pushNotifier       PushNotifier
}

func NewKloterService(operators *repository.OperatorRepository, kloters *repository.KloterRepository, audit *repository.AuditRepository, journey *JourneyService, push PushNotifier) *KloterService {
	return &KloterService{operatorRepository: operators, kloterRepository: kloters, auditRepository: audit, journeyService: journey, pushNotifier: push}
}

// kloterStatusPushText is the "seluruh jamaah kloter" push copy per the
// cascade map in SPEC_GROUP_KLOTER_MUTTAWWIF.md section 4.
var kloterStatusPushText = map[string]string{
	"DEPARTED":       "Perjalanan ibadah Anda dimulai",
	"IN_SAUDI":       "Selamat tiba di Tanah Suci",
	"DEPARTED_SAUDI": "Dalam perjalanan pulang ke Indonesia",
	"COMPLETED":      "Selamat tiba! Sertifikat Anda siap diunduh",
}

// kloterStatusOrder is forward-only, except the one explicit rollback
// business rule: CONFIRMED -> DRAFT (e.g. an operator undoes an early
// confirmation before departure prep has really started).
var kloterStatusOrder = []string{"DRAFT", "CONFIRMED", "DEPARTED", "IN_SAUDI", "DEPARTED_SAUDI", "COMPLETED"}

func kloterStatusIndex(status string) int {
	for i, s := range kloterStatusOrder {
		if s == status {
			return i
		}
	}
	return -1
}

// kloterToJourneyStatus is the cascade map from section 4 of
// SPEC_GROUP_KLOTER_MUTTAWWIF.md — only kloter transitions with a direct
// 1:1 journey-status equivalent cascade; DRAFT/CONFIRMED don't.
var kloterToJourneyStatus = map[string]string{
	"DEPARTED":       "DEPARTED_INDONESIA",
	"IN_SAUDI":       "ARRIVED_SAUDI",
	"DEPARTED_SAUDI": "DEPARTED_SAUDI",
	"COMPLETED":      "ARRIVED_INDONESIA",
}

func (s *KloterService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "kloter", entityID, message)
}

func (s *KloterService) ListKloters(ctx context.Context, orgID string, req *hajjv1.ListKlotersRequest) (*hajjv1.ListKlotersResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("KloterService.ListKloters", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.ListKloters", err)
	}
	kloters, err := s.kloterRepository.ListForOperator(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("KloterService.ListKloters", err)
	}
	result := &hajjv1.ListKlotersResponse{Kloters: make([]*hajjv1.Kloter, 0, len(kloters))}
	for _, k := range kloters {
		result.Kloters = append(result.Kloters, kloterMessage(k))
	}
	return result, nil
}

func (s *KloterService) CreateKloter(ctx context.Context, orgID string, req *hajjv1.CreateKloterRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("KloterService.CreateKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.CreateKloter", err)
	}
	kloter, err := s.kloterRepository.Create(ctx, op.ID, req.SeasonId, req.Code, req.Embarkation, req.FlightNumber, timestampPtr(req.DepartureDate), req.Capacity)
	if err != nil {
		return nil, serviceError("KloterService.CreateKloter", err)
	}
	s.logActivity(ctx, op.ID, "kloter_created", kloter.ID, fmt.Sprintf("Kloter %s dibuat", kloter.Code))
	return kloterMessage(kloter), nil
}

func (s *KloterService) UpdateKloter(ctx context.Context, orgID string, req *hajjv1.UpdateKloterRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("KloterService.UpdateKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloter", err)
	}
	kloter, err := s.kloterRepository.Update(ctx, op.ID, req.KloterId, req.Code, req.Embarkation, req.FlightNumber, timestampPtr(req.DepartureDate), req.Capacity)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloter", err)
	}
	s.logActivity(ctx, op.ID, "kloter_updated", kloter.ID, fmt.Sprintf("Kloter %s diperbarui", kloter.Code))
	return kloterMessage(kloter), nil
}

func (s *KloterService) UpdateKloterStatus(ctx context.Context, orgID string, req *hajjv1.UpdateKloterStatusRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" || strings.TrimSpace(req.Status) == "" {
		return nil, serviceError("KloterService.UpdateKloterStatus", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloterStatus", err)
	}
	kloter, err := s.updateStatus(ctx, op.ID, req.KloterId, req.Status)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloterStatus", err)
	}
	return kloterMessage(kloter), nil
}

// updateStatus is the operator-independent core, reused by
// TripService.UpdateTripKloterStatus (a Tour Leader assigned to the kloter
// via kloter_staff — already resolved its own operatorID via
// authorizeKloter, no need to re-derive it here).
func (s *KloterService) updateStatus(ctx context.Context, operatorID, kloterID, status string) (*domain.Kloter, error) {
	targetIdx := kloterStatusIndex(status)
	if targetIdx == -1 {
		return nil, apperror.ErrValidation
	}
	current, err := s.kloterRepository.GetForOperator(ctx, operatorID, kloterID)
	if err != nil {
		return nil, err
	}
	currentIdx := kloterStatusIndex(current.Status)
	isRollback := current.Status == "CONFIRMED" && status == "DRAFT"
	if !isRollback && targetIdx <= currentIdx {
		return nil, errors.New("status kloter hanya bisa maju")
	}
	kloter, err := s.kloterRepository.UpdateStatus(ctx, operatorID, kloterID, status)
	if err != nil {
		return nil, err
	}
	s.logActivity(ctx, operatorID, "kloter_status_changed", kloter.ID, fmt.Sprintf("Status kloter %s: %s -> %s", kloter.Code, current.Status, status))
	// Cascade: best-effort, never rolls back the status change itself —
	// full async event-bus/SSE/push delivery is a later phase, but the
	// core data effect (bulk journey status + push) happens synchronously.
	if journeyStatus, ok := kloterToJourneyStatus[status]; ok && s.journeyService != nil {
		if _, err := s.journeyService.bulkUpdateStatus(ctx, operatorID, kloter.ID, journeyStatus, fmt.Sprintf("Cascade dari kloter %s -> %s", kloter.Code, status)); err != nil {
			sentry.CaptureException(fmt.Errorf("KloterService.updateStatus: journey cascade: %w", err))
		}
	}
	if body, ok := kloterStatusPushText[status]; ok && s.pushNotifier != nil {
		s.pushNotifier.NotifyKloterPilgrims(ctx, operatorID, kloter.ID, "Tawafiq Hub", body)
	}
	return kloter, nil
}

func (s *KloterService) DeleteKloter(ctx context.Context, orgID string, req *hajjv1.DeleteKloterRequest) (*emptypb.Empty, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.DeleteKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.DeleteKloter", err)
	}
	if err := s.kloterRepository.Delete(ctx, op.ID, req.KloterId); err != nil {
		return nil, serviceError("KloterService.DeleteKloter", err)
	}
	return &emptypb.Empty{}, nil
}

func kloterMessage(k *domain.Kloter) *hajjv1.Kloter {
	msg := &hajjv1.Kloter{Id: k.ID, SeasonId: k.SeasonID, Code: k.Code, Embarkation: k.Embarkation, FlightNumber: k.FlightNumber, Capacity: k.Capacity, PilgrimCount: k.PilgrimCount, Status: k.Status, Notes: k.Notes}
	if k.DepartureDate != nil {
		msg.DepartureDate = timestamppb.New(*k.DepartureDate)
	}
	return msg
}
