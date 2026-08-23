package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/events"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type HealthReportService struct {
	operatorRepository *repository.OperatorRepository
	healthRepository   *repository.HealthReportRepository
	pilgrimRepository  *repository.PilgrimRepository
	auditRepository    *repository.AuditRepository
	outboxRepository   *repository.OutboxRepository
	db                 *pgxpool.Pool
	eventBus           *events.Bus
}

func NewHealthReportService(operators *repository.OperatorRepository, health *repository.HealthReportRepository, pilgrims *repository.PilgrimRepository, audit *repository.AuditRepository, outbox *repository.OutboxRepository, db *pgxpool.Pool, bus *events.Bus) *HealthReportService {
	return &HealthReportService{operatorRepository: operators, healthRepository: health, pilgrimRepository: pilgrims, auditRepository: audit, outboxRepository: outbox, db: db, eventBus: bus}
}

func (s *HealthReportService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "health_report", entityID, message)
}

// CreateHealthReport: BERAT severity automatically pushes to the operator —
// see PushNotifier.NotifyOperatorStaff. Never deletable, only resolvable —
// see HealthReportService.ResolveHealthReport.
func (s *HealthReportService) CreateHealthReport(ctx context.Context, orgID string, req *hajjv1.CreateHealthReportRequest) (*hajjv1.HealthReport, error) {
	if req == nil || strings.TrimSpace(req.PilgrimId) == "" || strings.TrimSpace(req.Symptoms) == "" {
		return nil, serviceError("HealthReportService.CreateHealthReport", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("HealthReportService.CreateHealthReport", err)
	}
	// group_id is never taken from the request — resolved server-side from
	// the pilgrim's own record, same boundary discipline as elsewhere in
	// this codebase (never trust a related-entity id from the client when
	// it can be derived).
	pilgrim, err := s.pilgrimRepository.Get(ctx, op.ID, req.PilgrimId)
	if err != nil {
		return nil, serviceError("HealthReportService.CreateHealthReport", apperror.ErrNotFound)
	}
	// The report insert and the outbox event that fans out its BERAT push
	// commit atomically — a crash after commit can't lose the "someone needs
	// to be alerted" signal (the old code pushed fire-and-forget, in-request,
	// which vanished on any failure). The worker relay drains it with retry.
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, serviceError("HealthReportService.CreateHealthReport", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	report, err := s.healthRepository.CreateTx(ctx, tx, op.ID, req.PilgrimId, pilgrim.GroupID, middleware.UserIDFromCtx(ctx), req.Severity, req.Symptoms, req.ActionTaken)
	if err != nil {
		return nil, serviceError("HealthReportService.CreateHealthReport", err)
	}
	if req.Severity == "BERAT" {
		if err := s.outboxRepository.EnqueueTx(ctx, tx, op.ID, domain.EventHealthReportCreated, report.ID, domain.HealthReportCreatedPayload{Severity: req.Severity, PilgrimName: pilgrim.FullName}); err != nil {
			return nil, serviceError("HealthReportService.CreateHealthReport", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError("HealthReportService.CreateHealthReport", err)
	}
	s.logActivity(ctx, op.ID, "health_report_created", report.ID, fmt.Sprintf("Laporan kesehatan (%s) untuk %s dibuat", req.Severity, pilgrim.FullName))
	s.eventBus.Publish(op.ID, "health", report.ID)
	return healthReportMessage(report, pilgrim.FullName), nil
}

func (s *HealthReportService) ListHealthReports(ctx context.Context, orgID string, req *hajjv1.ListHealthReportsRequest) (*hajjv1.ListHealthReportsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("HealthReportService.ListHealthReports", err)
	}
	var resolved *bool
	if req != nil && req.Resolved != nil {
		resolved = req.Resolved
	}
	reports, err := s.healthRepository.List(ctx, op.ID, resolved)
	if err != nil {
		return nil, serviceError("HealthReportService.ListHealthReports", err)
	}
	result := &hajjv1.ListHealthReportsResponse{Reports: make([]*hajjv1.HealthReport, 0, len(reports))}
	for _, r := range reports {
		result.Reports = append(result.Reports, healthReportListMessage(r))
	}
	return result, nil
}

func (s *HealthReportService) ResolveHealthReport(ctx context.Context, orgID string, req *hajjv1.ResolveHealthReportRequest) (*hajjv1.HealthReport, error) {
	if req == nil || strings.TrimSpace(req.ReportId) == "" {
		return nil, serviceError("HealthReportService.ResolveHealthReport", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("HealthReportService.ResolveHealthReport", err)
	}
	report, err := s.healthRepository.Resolve(ctx, op.ID, req.ReportId)
	if err != nil {
		return nil, serviceError("HealthReportService.ResolveHealthReport", err)
	}
	s.logActivity(ctx, op.ID, "health_report_resolved", report.ID, "Laporan kesehatan diselesaikan")
	s.eventBus.Publish(op.ID, "health", report.ID)
	return healthReportMessage(report, ""), nil
}

func healthReportMessage(r *domain.HealthReport, pilgrimName string) *hajjv1.HealthReport {
	msg := &hajjv1.HealthReport{Id: r.ID, PilgrimId: r.PilgrimID, PilgrimName: pilgrimName, GroupId: r.GroupID, Severity: r.Severity, Symptoms: r.Symptoms, ActionTaken: r.ActionTaken, Resolved: r.Resolved, CreatedAt: timestamppb.New(r.CreatedAt)}
	if r.ResolvedAt != nil {
		msg.ResolvedAt = timestamppb.New(*r.ResolvedAt)
	}
	return msg
}

func healthReportListMessage(r domain.HealthReport) *hajjv1.HealthReport {
	msg := &hajjv1.HealthReport{Id: r.ID, PilgrimId: r.PilgrimID, PilgrimName: r.PilgrimName, GroupId: r.GroupID, GroupName: r.GroupName, Severity: r.Severity, Symptoms: r.Symptoms, ActionTaken: r.ActionTaken, Resolved: r.Resolved, CreatedAt: timestamppb.New(r.CreatedAt)}
	if r.ResolvedAt != nil {
		msg.ResolvedAt = timestamppb.New(*r.ResolvedAt)
	}
	return msg
}
