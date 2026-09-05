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

type SupportService struct {
	operatorRepository *repository.OperatorRepository
	repository         *repository.SupportRepository
}

func NewSupportService(operators *repository.OperatorRepository, repo *repository.SupportRepository) *SupportService {
	return &SupportService{operatorRepository: operators, repository: repo}
}

func supportTicketMessage(t *domain.SupportTicket) *hajjv1.SupportTicket {
	msg := &hajjv1.SupportTicket{
		Id: t.ID, Subject: t.Subject, Priority: t.Priority, Status: t.Status,
		CreatedAt: timestamppb.New(t.CreatedAt), ResponseDueAt: timestamppb.New(t.ResponseDueAt()), ResponseOverdue: t.ResponseOverdue(),
	}
	if t.ResolvedAt != nil {
		msg.ResolvedAt = timestamppb.New(*t.ResolvedAt)
	}
	return msg
}

func supportMessageMessage(m *domain.SupportTicketMessage) *hajjv1.SupportTicketMessage {
	return &hajjv1.SupportTicketMessage{
		Id: m.ID, Body: m.Body, AuthorName: m.AuthorName, AuthorIsPlatform: m.AuthorIsPlatform,
		CreatedAt: timestamppb.New(m.CreatedAt),
	}
}

func (s *SupportService) CreateSupportTicket(ctx context.Context, orgID, userID, userName string, req *hajjv1.CreateSupportTicketRequest) (*hajjv1.SupportTicket, error) {
	if req == nil || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Body) == "" {
		return nil, serviceError("SupportService.CreateSupportTicket", apperror.ErrValidation)
	}
	priority := strings.ToUpper(strings.TrimSpace(req.Priority))
	if priority == "" {
		priority = "MEDIUM"
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SupportService.CreateSupportTicket", err)
	}
	ticket, err := s.repository.Create(ctx, op.ID, strings.TrimSpace(req.Subject), priority, userID, strings.TrimSpace(req.Body), userName)
	if err != nil {
		return nil, serviceError("SupportService.CreateSupportTicket", err)
	}
	return supportTicketMessage(ticket), nil
}

func (s *SupportService) ListMySupportTickets(ctx context.Context, orgID string, req *hajjv1.ListMySupportTicketsRequest) (*hajjv1.ListMySupportTicketsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SupportService.ListMySupportTickets", err)
	}
	tickets, err := s.repository.List(ctx, op.ID)
	if err != nil {
		return nil, serviceError("SupportService.ListMySupportTickets", err)
	}
	response := &hajjv1.ListMySupportTicketsResponse{}
	for _, t := range tickets {
		response.Tickets = append(response.Tickets, supportTicketMessage(t))
	}
	return response, nil
}

func (s *SupportService) GetSupportTicket(ctx context.Context, orgID string, req *hajjv1.GetSupportTicketRequest) (*hajjv1.SupportTicketDetail, error) {
	if req == nil || strings.TrimSpace(req.TicketId) == "" {
		return nil, serviceError("SupportService.GetSupportTicket", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SupportService.GetSupportTicket", err)
	}
	ticket, messages, err := s.repository.Get(ctx, op.ID, req.TicketId)
	if err != nil {
		return nil, serviceError("SupportService.GetSupportTicket", err)
	}
	detail := &hajjv1.SupportTicketDetail{Ticket: supportTicketMessage(ticket)}
	for _, m := range messages {
		detail.Messages = append(detail.Messages, supportMessageMessage(m))
	}
	return detail, nil
}

// AddSupportTicketMessage confirms the ticket belongs to this operator
// before writing to it — a plain insert keyed only by ticket_id would let an
// operator that merely guesses another tenant's ticket id post into their
// thread.
func (s *SupportService) AddSupportTicketMessage(ctx context.Context, orgID, userID, userName string, req *hajjv1.AddSupportTicketMessageRequest) (*hajjv1.SupportTicketMessage, error) {
	if req == nil || strings.TrimSpace(req.TicketId) == "" || strings.TrimSpace(req.Body) == "" {
		return nil, serviceError("SupportService.AddSupportTicketMessage", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SupportService.AddSupportTicketMessage", err)
	}
	if _, _, err := s.repository.Get(ctx, op.ID, req.TicketId); err != nil {
		return nil, serviceError("SupportService.AddSupportTicketMessage", err)
	}
	message, err := s.repository.AddMessage(ctx, req.TicketId, strings.TrimSpace(req.Body), userID, userName)
	if err != nil {
		return nil, serviceError("SupportService.AddSupportTicketMessage", err)
	}
	return supportMessageMessage(message), nil
}

func (s *SupportService) CloseSupportTicket(ctx context.Context, orgID string, req *hajjv1.CloseSupportTicketRequest) (*hajjv1.SupportTicket, error) {
	if req == nil || strings.TrimSpace(req.TicketId) == "" {
		return nil, serviceError("SupportService.CloseSupportTicket", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SupportService.CloseSupportTicket", err)
	}
	ticket, err := s.repository.Close(ctx, op.ID, req.TicketId)
	if err != nil {
		return nil, serviceError("SupportService.CloseSupportTicket", err)
	}
	return supportTicketMessage(ticket), nil
}
