package service

import (
	"context"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AgendaService struct {
	operatorRepository *repository.OperatorRepository
	agendaRepository   *repository.AgendaRepository
}

func NewAgendaService(operators *repository.OperatorRepository, agenda *repository.AgendaRepository) *AgendaService {
	return &AgendaService{operatorRepository: operators, agendaRepository: agenda}
}

func agendaEventMessage(e *domain.AgendaEvent) *hajjv1.AgendaEvent {
	msg := &hajjv1.AgendaEvent{
		Id: e.ID, BranchId: e.BranchID, BranchName: e.BranchName, SeasonId: e.SeasonID,
		Title: e.Title, Location: e.Location, StartsAt: timestamppb.New(e.StartsAt), Notes: e.Notes,
	}
	if !e.EndsAt.IsZero() {
		msg.EndsAt = timestamppb.New(e.EndsAt)
	}
	return msg
}

// endsAtOrZero keeps ends_at open-ended (the zero time) when the client sent
// none, rather than defaulting it to the Unix epoch.
func endsAtOrZero(ts *timestamppb.Timestamp) (t time.Time) {
	if ts != nil {
		t = ts.AsTime()
	}
	return t
}

func (s *AgendaService) CreateAgendaEvent(ctx context.Context, orgID string, req *hajjv1.CreateAgendaEventRequest) (*hajjv1.AgendaEvent, error) {
	if req == nil || strings.TrimSpace(req.Title) == "" || req.StartsAt == nil {
		return nil, serviceError("AgendaService.CreateAgendaEvent", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgendaService.CreateAgendaEvent", err)
	}
	e, err := s.agendaRepository.CreateEvent(ctx, op.ID, strings.TrimSpace(req.BranchId), strings.TrimSpace(req.SeasonId),
		strings.TrimSpace(req.Title), strings.TrimSpace(req.Location), req.StartsAt.AsTime(), endsAtOrZero(req.EndsAt), strings.TrimSpace(req.Notes))
	if err != nil {
		return nil, serviceError("AgendaService.CreateAgendaEvent", err)
	}
	return agendaEventMessage(e), nil
}

func (s *AgendaService) UpdateAgendaEvent(ctx context.Context, orgID string, req *hajjv1.UpdateAgendaEventRequest) (*hajjv1.AgendaEvent, error) {
	if req == nil || strings.TrimSpace(req.EventId) == "" || strings.TrimSpace(req.Title) == "" || req.StartsAt == nil {
		return nil, serviceError("AgendaService.UpdateAgendaEvent", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgendaService.UpdateAgendaEvent", err)
	}
	e, err := s.agendaRepository.UpdateEvent(ctx, op.ID, req.EventId, strings.TrimSpace(req.BranchId), strings.TrimSpace(req.SeasonId),
		strings.TrimSpace(req.Title), strings.TrimSpace(req.Location), req.StartsAt.AsTime(), endsAtOrZero(req.EndsAt), strings.TrimSpace(req.Notes))
	if err != nil {
		return nil, serviceError("AgendaService.UpdateAgendaEvent", err)
	}
	return agendaEventMessage(e), nil
}

func (s *AgendaService) DeleteAgendaEvent(ctx context.Context, orgID string, req *hajjv1.DeleteAgendaEventRequest) error {
	if req == nil || strings.TrimSpace(req.EventId) == "" {
		return serviceError("AgendaService.DeleteAgendaEvent", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("AgendaService.DeleteAgendaEvent", err)
	}
	if err := s.agendaRepository.DeleteEvent(ctx, op.ID, req.EventId); err != nil {
		return serviceError("AgendaService.DeleteAgendaEvent", err)
	}
	return nil
}

func (s *AgendaService) ListAgenda(ctx context.Context, orgID string, req *hajjv1.ListAgendaRequest) (*hajjv1.ListAgendaResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("AgendaService.ListAgenda", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgendaService.ListAgenda", err)
	}
	items, err := s.agendaRepository.ListAgenda(ctx, op.ID, req.SeasonId, strings.TrimSpace(req.BranchId))
	if err != nil {
		return nil, serviceError("AgendaService.ListAgenda", err)
	}
	response := &hajjv1.ListAgendaResponse{}
	for _, item := range items {
		msg := &hajjv1.AgendaItem{
			Id: item.ID, Kind: item.Kind, Title: item.Title, Location: item.Location,
			StartsAt: timestamppb.New(item.StartsAt), KloterId: item.KloterID, KloterCode: item.KloterCode,
			BranchId: item.BranchID, BranchName: item.BranchName, Notes: item.Notes,
		}
		if !item.EndsAt.IsZero() {
			msg.EndsAt = timestamppb.New(item.EndsAt)
		}
		response.Items = append(response.Items, msg)
	}
	return response, nil
}
