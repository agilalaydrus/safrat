package service

import (
	"context"
	"fmt"
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
	auditRepository    *repository.AuditRepository
	notifier           SOSNotifier
}

// SOSNotifier decouples the service from the Firebase-specific push
// implementation (internal/notification) — nil is a valid no-op notifier for
// local dev without FIREBASE_SERVICE_ACCOUNT_JSON set.
type SOSNotifier interface {
	NotifySOSAlert(ctx context.Context, operatorID string, alert *domain.SOSAlert)
}

func NewSOSService(operators *repository.OperatorRepository, pilgrims *repository.PilgrimRepository, sos *repository.SOSRepository, audit *repository.AuditRepository, notifier SOSNotifier) *SOSService {
	return &SOSService{operatorRepository: operators, pilgrimRepository: pilgrims, sosRepository: sos, auditRepository: audit, notifier: notifier}
}

func (s *SOSService) logActivity(ctx context.Context, operatorID, userID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, userID, action, "sos_alert", entityID, message)
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
	// Prefer a fresh one-shot GPS fix from the SOS request itself; fall back
	// to the pilgrim's last periodic location ping (see UpdateMyLocation)
	// when the device couldn't get one in time.
	lat, lng := req.Lat, req.Lng
	if lat == nil || lng == nil {
		lat, lng = pilgrim.LastLat, pilgrim.LastLng
	}
	alert, err := s.sosRepository.Create(ctx, pilgrim.OperatorID, pilgrim.ID, lat, lng)
	if err != nil {
		return nil, serviceError("SOSService.CreateSOSAlert", err)
	}
	alert.PilgrimName = pilgrim.FullName
	s.logActivity(ctx, pilgrim.OperatorID, "", "sos_created", alert.ID, fmt.Sprintf("SOS dari %s", pilgrim.FullName))
	if s.notifier != nil {
		s.notifier.NotifySOSAlert(ctx, pilgrim.OperatorID, alert)
	}
	return sosAlertMessage(alert), nil
}

// ListMyPilgrimSOSAlerts is the public, unauthenticated entry point the
// pilgrim app polls to notice a leader has acknowledged/resolved their
// alert — see publicProcedures in internal/middleware/auth.go.
func (s *SOSService) ListMyPilgrimSOSAlerts(ctx context.Context, req *hajjv1.ListMyPilgrimSOSAlertsRequest) (*hajjv1.ListSOSAlertsResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("SOSService.ListMyPilgrimSOSAlerts", apperror.ErrValidation)
	}
	pilgrim, err := s.pilgrimRepository.GetByAppAccessCode(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("SOSService.ListMyPilgrimSOSAlerts", apperror.ErrNotFound)
	}
	alerts, err := s.sosRepository.ListPilgrimHistory(ctx, pilgrim.OperatorID, pilgrim.ID, pilgrim.FullName)
	if err != nil {
		return nil, serviceError("SOSService.ListMyPilgrimSOSAlerts", err)
	}
	result := &hajjv1.ListSOSAlertsResponse{Alerts: make([]*hajjv1.SOSAlert, 0, len(alerts))}
	for _, alert := range alerts {
		result.Alerts = append(result.Alerts, sosAlertMessage(alert))
	}
	return result, nil
}

// EscalateStaleAlerts is invoked by the worker's periodic sweep (cmd/worker),
// not by any RPC — matches the business rule "unacknowledged after 10min ->
// ESCALATED -> push to ALL coordinators."
func (s *SOSService) EscalateStaleAlerts(ctx context.Context) error {
	alerts, err := s.sosRepository.EscalateStale(ctx)
	if err != nil {
		return err
	}
	for _, alert := range alerts {
		s.logActivity(ctx, alert.OperatorID, "", "sos_escalated", alert.ID, fmt.Sprintf("SOS %s dieskalasi — belum dikonfirmasi 10 menit", alert.PilgrimName))
		if s.notifier != nil {
			s.notifier.NotifySOSAlert(ctx, alert.OperatorID, alert)
		}
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
	s.logActivity(ctx, op.ID, userID, "sos_acknowledged", alert.ID, "SOS dikonfirmasi")
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
	s.logActivity(ctx, op.ID, userID, "sos_resolved", alert.ID, "SOS ditandai selesai")
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
	if alert.Lat != nil && alert.Lng != nil {
		message.Lat = *alert.Lat
		message.Lng = *alert.Lng
		message.HasLocation = true
		message.LocationAt = timestamppb.New(alert.CreatedAt)
	}
	return message
}
