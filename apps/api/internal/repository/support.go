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
