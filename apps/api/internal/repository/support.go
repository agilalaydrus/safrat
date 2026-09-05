package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
)

type SupportRepository struct {
	queries *db.Queries
}

func NewSupportRepository(queries *db.Queries) *SupportRepository {
	return &SupportRepository{queries: queries}
}

func toSupportTicket(row db.SupportTicket) *domain.SupportTicket {
	t := &domain.SupportTicket{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), Subject: row.Subject,
		Priority: row.Priority, Status: row.Status, CreatedByID: row.CreatedByUserID, CreatedAt: row.CreatedAt.Time,
	}
	if row.ResolvedAt.Valid {
		resolved := row.ResolvedAt.Time
		t.ResolvedAt = &resolved
	}
	return t
}

func toSupportTicketMessage(row db.SupportTicketMessage) *domain.SupportTicketMessage {
	return &domain.SupportTicketMessage{
		ID: uuidString(row.ID), TicketID: uuidString(row.TicketID), Body: row.Body,
		AuthorUserID: row.AuthorUserID, AuthorName: row.AuthorName, AuthorIsPlatform: row.AuthorIsPlatform,
		CreatedAt: row.CreatedAt.Time,
	}
}

func (r *SupportRepository) Create(ctx context.Context, operatorID, subject, priority, createdByUserID, firstMessageBody, firstMessageAuthorName string) (*domain.SupportTicket, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateSupportTicket(ctx, db.CreateSupportTicketParams{
		OperatorID: op, Subject: subject, Priority: priority, CreatedByUserID: createdByUserID,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	if _, err := r.queries.CreateSupportTicketMessage(ctx, db.CreateSupportTicketMessageParams{
		TicketID: row.ID, Body: firstMessageBody, AuthorUserID: createdByUserID, AuthorName: firstMessageAuthorName,
	}); err != nil {
		return nil, databaseError(err)
	}
	return toSupportTicket(row), nil
}

func (r *SupportRepository) List(ctx context.Context, operatorID string) ([]*domain.SupportTicket, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListSupportTickets(ctx, op)
	if err != nil {
		return nil, databaseError(err)
	}
	out := make([]*domain.SupportTicket, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSupportTicket(row))
	}
	return out, nil
}

func (r *SupportRepository) Get(ctx context.Context, operatorID, ticketID string) (*domain.SupportTicket, []*domain.SupportTicketMessage, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, nil, apperror.ErrValidation
	}
	id, err := pgUUID(ticketID)
	if err != nil {
		return nil, nil, apperror.ErrValidation
	}
	ticketRow, err := r.queries.GetSupportTicket(ctx, db.GetSupportTicketParams{ID: id, OperatorID: op})
	if err != nil {
		return nil, nil, databaseError(err)
	}
	messageRows, err := r.queries.ListSupportTicketMessages(ctx, ticketRow.ID)
	if err != nil {
		return nil, nil, databaseError(err)
	}
	messages := make([]*domain.SupportTicketMessage, 0, len(messageRows))
	for _, row := range messageRows {
		messages = append(messages, toSupportTicketMessage(row))
	}
	return toSupportTicket(ticketRow), messages, nil
}

// AddMessage requires the ticket to already belong to this operator —
// callers pass it in confirmed, not re-checked here, so this stays a plain
// insert; GetSupportTicket / the service layer is where that check happens.
func (r *SupportRepository) AddMessage(ctx context.Context, ticketID, body, authorUserID, authorName string) (*domain.SupportTicketMessage, error) {
	id, err := pgUUID(ticketID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateSupportTicketMessage(ctx, db.CreateSupportTicketMessageParams{
		TicketID: id, Body: body, AuthorUserID: authorUserID, AuthorName: authorName,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toSupportTicketMessage(row), nil
}

func (r *SupportRepository) Close(ctx context.Context, operatorID, ticketID string) (*domain.SupportTicket, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	id, err := pgUUID(ticketID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CloseSupportTicket(ctx, db.CloseSupportTicketParams{ID: id, OperatorID: op})
	if err != nil {
		return nil, databaseError(err)
	}
	return toSupportTicket(row), nil
}

// ListAll, GetAsPlatform, ReplyAsPlatform and SetStatus are the platform-side
// half (C5, TUGAS-PANEL-SAAS.md) — deliberately unscoped by operator_id,
// which is what makes this the admin inbox rather than a tenant's own.

func (r *SupportRepository) ListAll(ctx context.Context) ([]*domain.SupportTicket, error) {
	rows, err := r.queries.ListAllSupportTickets(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	out := make([]*domain.SupportTicket, 0, len(rows))
	for _, row := range rows {
		t := &domain.SupportTicket{
			ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), OperatorName: row.OperatorName,
			Subject: row.Subject, Priority: row.Priority, Status: row.Status,
			CreatedByID: row.CreatedByUserID, CreatedAt: row.CreatedAt.Time,
		}
		if row.ResolvedAt.Valid {
			resolved := row.ResolvedAt.Time
			t.ResolvedAt = &resolved
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *SupportRepository) GetAsPlatform(ctx context.Context, ticketID string) (*domain.SupportTicket, []*domain.SupportTicketMessage, error) {
	id, err := pgUUID(ticketID)
	if err != nil {
		return nil, nil, apperror.ErrValidation
	}
	row, err := r.queries.GetSupportTicketAsPlatform(ctx, id)
	if err != nil {
		return nil, nil, databaseError(err)
	}
	messageRows, err := r.queries.ListSupportTicketMessages(ctx, row.ID)
	if err != nil {
		return nil, nil, databaseError(err)
	}
	messages := make([]*domain.SupportTicketMessage, 0, len(messageRows))
	for _, m := range messageRows {
		messages = append(messages, toSupportTicketMessage(m))
	}
	ticket := &domain.SupportTicket{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), OperatorName: row.OperatorName,
		Subject: row.Subject, Priority: row.Priority, Status: row.Status,
		CreatedByID: row.CreatedByUserID, CreatedAt: row.CreatedAt.Time,
	}
	if row.ResolvedAt.Valid {
		resolved := row.ResolvedAt.Time
		ticket.ResolvedAt = &resolved
	}
	return ticket, messages, nil
}

// ReplyAsPlatform requires the ticket to exist — checked by the caller via
// GetAsPlatform first, same reasoning as AddMessage above — and writes with
// author_is_platform true.
func (r *SupportRepository) ReplyAsPlatform(ctx context.Context, ticketID, body, authorUserID, authorName string) (*domain.SupportTicketMessage, error) {
	id, err := pgUUID(ticketID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateSupportTicketMessageAsPlatform(ctx, db.CreateSupportTicketMessageAsPlatformParams{
		TicketID: id, Body: body, AuthorUserID: authorUserID, AuthorName: authorName,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toSupportTicketMessage(row), nil
}

// SetStatus refuses to touch a CLOSED ticket (see the query's WHERE clause)
// — CLOSED is exclusively the operator's own CloseSupportTicket action.
func (r *SupportRepository) SetStatus(ctx context.Context, ticketID, status string) (*domain.SupportTicket, error) {
	id, err := pgUUID(ticketID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.SetSupportTicketStatus(ctx, db.SetSupportTicketStatusParams{ID: id, Status: status})
	if err != nil {
		return nil, databaseError(err)
	}
	return toSupportTicket(row), nil
}
