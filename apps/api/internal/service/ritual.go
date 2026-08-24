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
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RitualService struct {
	operatorRepository *repository.OperatorRepository
	ritualRepository   *repository.RitualRepository
	journeyRepository  *repository.JourneyRepository
	auditRepository    *repository.AuditRepository
	outboxRepository   *repository.OutboxRepository
	db                 *pgxpool.Pool
	eventBus           *events.Bus
}

func NewRitualService(operators *repository.OperatorRepository, rituals *repository.RitualRepository, journeys *repository.JourneyRepository, audit *repository.AuditRepository, outbox *repository.OutboxRepository, db *pgxpool.Pool, bus *events.Bus) *RitualService {
	return &RitualService{operatorRepository: operators, ritualRepository: rituals, journeyRepository: journeys, auditRepository: audit, outboxRepository: outbox, db: db, eventBus: bus}
}

func (s *RitualService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "ritual", entityID, message)
}

func (s *RitualService) ListRitualTemplates(ctx context.Context, orgID string, req *hajjv1.ListRitualTemplatesRequest) (*hajjv1.ListRitualTemplatesResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonType) == "" {
		return nil, serviceError("RitualService.ListRitualTemplates", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("RitualService.ListRitualTemplates", err)
	}
	templates, err := s.ritualRepository.ListTemplates(ctx, op.ID, req.SeasonType)
	if err != nil {
		return nil, serviceError("RitualService.ListRitualTemplates", err)
	}
	result := &hajjv1.ListRitualTemplatesResponse{Templates: make([]*hajjv1.RitualTemplate, 0, len(templates))}
	for _, t := range templates {
		result.Templates = append(result.Templates, ritualTemplateMessage(t))
	}
	return result, nil
}

func (s *RitualService) CreateRitualTemplate(ctx context.Context, orgID string, req *hajjv1.CreateRitualTemplateRequest) (*hajjv1.RitualTemplate, error) {
	if req == nil || strings.TrimSpace(req.SeasonType) == "" || strings.TrimSpace(req.Name) == "" {
		return nil, serviceError("RitualService.CreateRitualTemplate", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("RitualService.CreateRitualTemplate", err)
	}
	t, err := s.ritualRepository.CreateTemplate(ctx, op.ID, req.SeasonType, req.Name, req.Description, req.OrderNum, req.IsRequired)
	if err != nil {
		return nil, serviceError("RitualService.CreateRitualTemplate", err)
	}
	s.logActivity(ctx, op.ID, "ritual_template_created", t.ID, fmt.Sprintf("Ritual %s ditambahkan", t.Name))
	return ritualTemplateMessage(*t), nil
}

// SeedDefaultTemplates is a no-op if templates already exist for this
// season_type — safe to call more than once.
func (s *RitualService) SeedDefaultTemplates(ctx context.Context, orgID string, req *hajjv1.SeedDefaultTemplatesRequest) (*emptypb.Empty, error) {
	if req == nil || strings.TrimSpace(req.SeasonType) == "" {
		return nil, serviceError("RitualService.SeedDefaultTemplates", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("RitualService.SeedDefaultTemplates", err)
	}
	count, err := s.ritualRepository.CountTemplates(ctx, op.ID, req.SeasonType)
	if err != nil {
		return nil, serviceError("RitualService.SeedDefaultTemplates", err)
	}
	if count > 0 {
		return &emptypb.Empty{}, nil
	}
	defaults := domain.DefaultUmrahRituals
	if req.SeasonType == "HAJJ" {
		defaults = domain.DefaultHajjRituals
	}
	for i, d := range defaults {
		if _, err := s.ritualRepository.CreateTemplate(ctx, op.ID, req.SeasonType, d.Name, d.Description, int32(i), true); err != nil {
			return nil, serviceError("RitualService.SeedDefaultTemplates", err)
		}
	}
	s.logActivity(ctx, op.ID, "ritual_templates_seeded", "", fmt.Sprintf("%d template ritual %s dibuat otomatis", len(defaults), req.SeasonType))
	return &emptypb.Empty{}, nil
}

func (s *RitualService) GetGroupRitualProgress(ctx context.Context, orgID string, req *hajjv1.GetGroupRitualProgressRequest) (*hajjv1.GroupRitualProgress, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" {
		return nil, serviceError("RitualService.GetGroupRitualProgress", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("RitualService.GetGroupRitualProgress", err)
	}
	items, err := s.ritualRepository.GetGroupProgress(ctx, op.ID, req.GroupId)
	if err != nil {
		return nil, serviceError("RitualService.GetGroupRitualProgress", err)
	}
	result := &hajjv1.GroupRitualProgress{GroupId: req.GroupId, Items: make([]*hajjv1.RitualProgressItem, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, &hajjv1.RitualProgressItem{
			RitualId: item.RitualID, Name: item.Name, OrderNum: item.OrderNum, TotalPilgrims: item.TotalPilgrims,
			CompletedCount: item.CompletedCount, IncompletePilgrimNames: item.IncompletePilgrimNames,
		})
	}
	return result, nil
}

// BulkCompleteRitual is the Muttawwif's one-tap "everyone's done" action —
// backend resolves the pilgrim list from group_id, client never sends one.
// Ritual completion is immutable (never unmarked) per the business rule in
// SPEC_GROUP_KLOTER_MUTTAWWIF.md section 7 — CompletePilgrimRitual is a
// one-way upsert, there's no uncomplete path anywhere in this service.
func (s *RitualService) BulkCompleteRitual(ctx context.Context, orgID string, req *hajjv1.BulkCompleteRitualRequest) (*hajjv1.BulkCompleteRitualResponse, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" || strings.TrimSpace(req.RitualId) == "" {
		return nil, serviceError("RitualService.BulkCompleteRitual", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("RitualService.BulkCompleteRitual", err)
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, serviceError("RitualService.BulkCompleteRitual", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	pilgrimIDs, err := s.journeyRepository.ListPilgrimIDsByGroupTx(ctx, tx, op.ID, req.GroupId)
	if err != nil {
		return nil, serviceError("RitualService.BulkCompleteRitual", err)
	}
	excluded := make(map[string]bool, len(req.ExcludedPilgrimIds))
	for _, id := range req.ExcludedPilgrimIds {
		excluded[id] = true
	}
	userID := middleware.UserIDFromCtx(ctx)
	var count int32
	for _, pilgrimID := range pilgrimIDs {
		if excluded[pilgrimID] {
			continue
		}
		if err := s.ritualRepository.CompletePilgrimRitualTx(ctx, tx, op.ID, pilgrimID, req.RitualId, userID, req.Notes); err != nil {
			return nil, serviceError("RitualService.BulkCompleteRitual", err)
		}
		count++
	}
	if err := s.outboxRepository.EnqueueTx(ctx, tx, op.ID, domain.EventRitualBulkCompleted, req.GroupId, domain.RitualBulkCompletedPayload{
		GroupID: req.GroupId, RitualID: req.RitualId, CompletedCount: count, NotificationBody: "Sebuah ritual ibadah telah diselesaikan ✓",
	}); err != nil {
		return nil, serviceError("RitualService.BulkCompleteRitual", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError("RitualService.BulkCompleteRitual", err)
	}
	s.logActivity(ctx, op.ID, "ritual_bulk_completed", req.GroupId, fmt.Sprintf("%d jamaah menyelesaikan ritual", count))
	s.eventBus.Publish(op.ID, "ritual", req.GroupId)
	return &hajjv1.BulkCompleteRitualResponse{CompletedCount: count}, nil
}

func (s *RitualService) CompleteRitualForPilgrim(ctx context.Context, orgID string, req *hajjv1.CompleteRitualForPilgrimRequest) (*emptypb.Empty, error) {
	if req == nil || strings.TrimSpace(req.PilgrimId) == "" || strings.TrimSpace(req.RitualId) == "" {
		return nil, serviceError("RitualService.CompleteRitualForPilgrim", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("RitualService.CompleteRitualForPilgrim", err)
	}
	if err := s.ritualRepository.CompletePilgrimRitual(ctx, op.ID, req.PilgrimId, req.RitualId, middleware.UserIDFromCtx(ctx), req.Notes); err != nil {
		return nil, serviceError("RitualService.CompleteRitualForPilgrim", err)
	}
	return &emptypb.Empty{}, nil
}

func ritualTemplateMessage(t domain.RitualTemplate) *hajjv1.RitualTemplate {
	return &hajjv1.RitualTemplate{Id: t.ID, SeasonType: t.SeasonType, Name: t.Name, Description: t.Description, OrderNum: t.OrderNum, IsRequired: t.IsRequired}
}

func pilgrimRitualStatusMessage(s domain.PilgrimRitualStatus) *hajjv1.PilgrimRitualStatus {
	msg := &hajjv1.PilgrimRitualStatus{RitualId: s.RitualID, Name: s.Name, Description: s.Description, OrderNum: s.OrderNum, IsRequired: s.IsRequired, Completed: s.Completed, CompletedByName: s.CompletedByName}
	if s.CompletedAt != nil {
		msg.CompletedAt = timestamppb.New(*s.CompletedAt)
	}
	return msg
}
