package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SOSService struct {
	operatorRepository *repository.OperatorRepository
	pilgrimRepository  *repository.PilgrimRepository
	sosRepository      *repository.SOSRepository
	notifier           SOSNotifier
}

// SOSNotifier decouples the service from the Firebase-specific push
// implementation (internal/notification) — nil is a valid no-op notifier for
// local dev without FIREBASE_SERVICE_ACCOUNT_JSON set.
type SOSNotifier interface {
	NotifySOSAlert(ctx context.Context, operatorID string, alert *domain.SOSAlert)
}

func NewSOSService(operators *repository.OperatorRepository, pilgrims *repository.PilgrimRepository, sos *repository.SOSRepository, notifier SOSNotifier) *SOSService {
	return &SOSService{operatorRepository: operators, pilgrimRepository: pilgrims, sosRepository: sos, notifier: notifier}
}

// CreateSOSAlert is the public, unauthenticated entry point — see
// publicProcedures in internal/middleware/auth.go. Deliberately not
// rate-limited (see ratelimit.go): a real emergency must never be throttled.
func (s *SOSService) CreateSOSAlert(ctx context.Context, req *hajjv1.CreateSOSAlertRequest) (*hajjv1.SOSAlert, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("SOSService.CreateSOSAlert", apperror.ErrValidation)
	}
	pilgrim, err := s.pilgrimRepository.GetByAppAccessCode(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("SOSService.CreateSOSAlert", apperror.ErrNotFound)
	}
	alert, err := s.sosRepository.Create(ctx, pilgrim.OperatorID, pilgrim.ID)
	if err != nil {
		return nil, serviceError("SOSService.CreateSOSAlert", err)
	}
	alert.PilgrimName = pilgrim.FullName
	if s.notifier != nil {
		s.notifier.NotifySOSAlert(ctx, pilgrim.OperatorID, alert)
	}
	return sosAlertMessage(alert), nil
}

// EscalateStaleAlerts is invoked by the worker's periodic sweep (cmd/worker),
// not by any RPC — matches the business rule "unacknowledged after 10min ->
// ESCALATED -> push to ALL coordinators."
func (s *SOSService) EscalateStaleAlerts(ctx context.Context) error {
	alerts, err := s.sosRepository.EscalateStale(ctx)
	if err != nil {
		return err
	}
	if s.notifier == nil {
		return nil
	}
	for _, alert := range alerts {
		s.notifier.NotifySOSAlert(ctx, alert.OperatorID, alert)
	}
	return nil
}

func (s *SOSService) ListActiveSOSAlerts(ctx context.Context, orgID string) (*hajjv1.ListSOSAlertsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SOSService.ListActiveSOSAlerts", err)
	}
	alerts, err := s.sosRepository.ListActive(ctx, op.ID)
	if err != nil {
		return nil, serviceError("SOSService.ListActiveSOSAlerts", err)
	}
	result := &hajjv1.ListSOSAlertsResponse{Alerts: make([]*hajjv1.SOSAlert, 0, len(alerts))}
	for _, alert := range alerts {
		result.Alerts = append(result.Alerts, sosAlertMessage(alert))
	}
	return result, nil
}

func (s *SOSService) AcknowledgeSOSAlert(ctx context.Context, orgID, userID string, req *hajjv1.AcknowledgeSOSAlertRequest) (*hajjv1.SOSAlert, error) {
	if req == nil || strings.TrimSpace(req.SosAlertId) == "" {
		return nil, serviceError("SOSService.AcknowledgeSOSAlert", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SOSService.AcknowledgeSOSAlert", err)
	}
	alert, err := s.sosRepository.Acknowledge(ctx, op.ID, req.SosAlertId, userID)
	if err != nil {
		return nil, serviceError("SOSService.AcknowledgeSOSAlert", apperror.ErrFailedPrecondition)
	}
	return sosAlertMessage(alert), nil
}

func (s *SOSService) ResolveSOSAlert(ctx context.Context, orgID, userID string, req *hajjv1.ResolveSOSAlertRequest) (*hajjv1.SOSAlert, error) {
	if req == nil || strings.TrimSpace(req.SosAlertId) == "" {
		return nil, serviceError("SOSService.ResolveSOSAlert", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SOSService.ResolveSOSAlert", err)
	}
	alert, err := s.sosRepository.Resolve(ctx, op.ID, req.SosAlertId, userID, req.Notes)
	if err != nil {
		return nil, serviceError("SOSService.ResolveSOSAlert", apperror.ErrFailedPrecondition)
	}
	return sosAlertMessage(alert), nil
}

func sosAlertMessage(alert *domain.SOSAlert) *hajjv1.SOSAlert {
	message := &hajjv1.SOSAlert{Id: alert.ID, PilgrimId: alert.PilgrimID, PilgrimName: alert.PilgrimName, Status: alert.Status, Notes: alert.Notes, CreatedAt: timestamppb.New(alert.CreatedAt)}
	if alert.AcknowledgedAt != nil {
		message.AcknowledgedAt = timestamppb.New(*alert.AcknowledgedAt)
	}
	if alert.ResolvedAt != nil {
		message.ResolvedAt = timestamppb.New(*alert.ResolvedAt)
	}
	return message
}
