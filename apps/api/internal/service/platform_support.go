package service

import (
	"context"
	"sort"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
)

// ListAllSupportTickets is C5's inbox (TUGAS-PANEL-SAAS.md): every tenant's
// tickets, unscoped by design — that is the entire point of this being on
// PlatformService rather than SupportService.
func (s *PlatformService) ListAllSupportTickets(ctx context.Context, req *hajjv1.ListAllSupportTicketsRequest) (*hajjv1.ListAllSupportTicketsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	tickets, err := s.supportRepository.ListAll(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.ListAllSupportTickets", err)
	}
	status := strings.ToUpper(strings.TrimSpace(req.GetStatus()))
	priority := strings.ToUpper(strings.TrimSpace(req.GetPriority()))
	filtered := tickets[:0]
	for _, t := range tickets {
		if status != "" && t.Status != status {
			continue
		}
		if priority != "" && t.Priority != priority {
			continue
		}
		filtered = append(filtered, t)
	}
	// Whichever ticket has blown past its first-response target the longest
	// belongs at the top — that is the one question this inbox exists to
	// answer at a glance. Everything else keeps the newest-first order the
	// repository already returned.
	sort.SliceStable(filtered, func(i, j int) bool {
		oi, oj := filtered[i].ResponseOverdue(), filtered[j].ResponseOverdue()
		if oi != oj {
			return oi
		}
		if oi && oj {
			return filtered[i].ResponseDueAt().Before(filtered[j].ResponseDueAt())
		}
		return false
	})
	response := &hajjv1.ListAllSupportTicketsResponse{}
	for _, t := range filtered {
		response.Tickets = append(response.Tickets, supportTicketMessage(t))
	}
	return response, nil
}

func (s *PlatformService) GetSupportTicketAsPlatform(ctx context.Context, req *hajjv1.GetSupportTicketAsPlatformRequest) (*hajjv1.SupportTicketDetail, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.TicketId) == "" {
		return nil, serviceError("PlatformService.GetSupportTicketAsPlatform", apperror.ErrValidation)
	}
	ticket, messages, err := s.supportRepository.GetAsPlatform(ctx, req.TicketId)
	if err != nil {
		return nil, serviceError("PlatformService.GetSupportTicketAsPlatform", err)
	}
	detail := &hajjv1.SupportTicketDetail{Ticket: supportTicketMessage(ticket)}
	for _, m := range messages {
		detail.Messages = append(detail.Messages, supportMessageMessage(m))
	}
	return detail, nil
}

// ReplyToSupportTicketAsPlatform confirms the ticket exists before writing to
// it, same reasoning as SupportService.AddSupportTicketMessage — a plain
// insert keyed only by ticket_id would happily write into a ticket id that
// does not exist at all.
func (s *PlatformService) ReplyToSupportTicketAsPlatform(ctx context.Context, req *hajjv1.ReplyToSupportTicketAsPlatformRequest) (*hajjv1.SupportTicketMessage, error) {
	adminUserID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.TicketId) == "" || strings.TrimSpace(req.Body) == "" {
		return nil, serviceError("PlatformService.ReplyToSupportTicketAsPlatform", apperror.ErrValidation)
	}
	ticket, _, err := s.supportRepository.GetAsPlatform(ctx, req.TicketId)
	if err != nil {
		return nil, serviceError("PlatformService.ReplyToSupportTicketAsPlatform", err)
	}
	// Same source SupportService.AddSupportTicketMessage uses for the
	// operator's own side — the staff session's own display name, resolved
	// by the auth interceptor, never a value the caller supplies.
	authorName := middleware.UserNameFromCtx(ctx)
	if authorName == "" {
		authorName = "Tim TawafiqHub"
	}
	message, err := s.supportRepository.ReplyAsPlatform(ctx, req.TicketId, strings.TrimSpace(req.Body), adminUserID, authorName)
	if err != nil {
		return nil, serviceError("PlatformService.ReplyToSupportTicketAsPlatform", err)
	}
	// F5 (TUGAS-PANEL-SAAS.md): a platform admin writing into a tenant's own
	// support thread is exactly the kind of cross-tenant action the audit
	// trail exists to catch — scoped to that tenant, not blank, so it shows
	// up on their own "Jejak audit" the same as any other platform action
	// taken on their account.
	_ = s.auditRepository.Write(ctx, ticket.OperatorID, adminUserID, "support_ticket_replied_as_platform",
		"support_ticket", req.TicketId, "Dibalas oleh "+authorName)
	return supportMessageMessage(message), nil
}

// SetSupportTicketStatus moves a ticket between OPEN, IN_PROGRESS and
// RESOLVED — buf.validate on the request already rejects anything else, and
// the repository's own WHERE clause refuses a CLOSED ticket regardless, so
// this is defence in depth, not the only thing standing between this RPC and
// touching a ticket that belongs to the operator's own CloseSupportTicket.
func (s *PlatformService) SetSupportTicketStatus(ctx context.Context, req *hajjv1.SetSupportTicketStatusRequest) (*hajjv1.SupportTicket, error) {
	adminUserID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.TicketId) == "" {
		return nil, serviceError("PlatformService.SetSupportTicketStatus", apperror.ErrValidation)
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	ticket, err := s.supportRepository.SetStatus(ctx, req.TicketId, status)
	if err != nil {
		return nil, serviceError("PlatformService.SetSupportTicketStatus", err)
	}
	// F5 (TUGAS-PANEL-SAAS.md): see ReplyToSupportTicketAsPlatform — same
	// reasoning, scoped to the ticket's own tenant.
	_ = s.auditRepository.Write(ctx, ticket.OperatorID, adminUserID, "support_ticket_status_set_as_platform",
		"support_ticket", req.TicketId, "Status diubah menjadi "+status)
	return supportTicketMessage(ticket), nil
}
