package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

type ChatRepository struct{ queries *db.Queries }

func NewChatRepository(queries *db.Queries) *ChatRepository {
	return &ChatRepository{queries: queries}
}

func (r *ChatRepository) CreateFromPilgrim(ctx context.Context, operatorID, groupID, pilgrimID, senderName, body string) (*domain.ChatMessage, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	message, err := r.queries.CreateChatMessageFromPilgrim(ctx, db.CreateChatMessageFromPilgrimParams{OperatorID: opUUID, GroupID: groupUUID, SenderPilgrimID: pilgrimUUID, Body: body})
	if err != nil {
		return nil, err
	}
	return &domain.ChatMessage{ID: uuid.UUID(message.ID.Bytes).String(), OperatorID: operatorID, GroupID: groupID, SenderName: senderName, FromPilgrim: true, Body: message.Body, CreatedAt: message.CreatedAt.Time}, nil
}

func (r *ChatRepository) CreateFromUser(ctx context.Context, operatorID, groupID, userID, senderName, body string) (*domain.ChatMessage, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, err
	}
	message, err := r.queries.CreateChatMessageFromUser(ctx, db.CreateChatMessageFromUserParams{OperatorID: opUUID, GroupID: groupUUID, SenderUserID: pgText(userID), Body: body})
	if err != nil {
		return nil, err
	}
	return &domain.ChatMessage{ID: uuid.UUID(message.ID.Bytes).String(), OperatorID: operatorID, GroupID: groupID, SenderName: senderName, FromPilgrim: false, Body: message.Body, CreatedAt: message.CreatedAt.Time}, nil
}

func (r *ChatRepository) ListByGroup(ctx context.Context, operatorID, groupID string) ([]*domain.ChatMessage, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListChatMessagesByGroup(ctx, db.ListChatMessagesByGroupParams{GroupID: groupUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.ChatMessage, 0, len(rows))
	for _, row := range rows {
		fromPilgrim := row.SenderPilgrimID.Valid
		name := row.SenderUserName.String
		if fromPilgrim {
			name = row.SenderPilgrimName.String
		}
		result = append(result, &domain.ChatMessage{ID: uuid.UUID(row.ID.Bytes).String(), OperatorID: operatorID, GroupID: groupID, SenderName: name, FromPilgrim: fromPilgrim, Body: row.Body, CreatedAt: row.CreatedAt.Time})
	}
	return result, nil
}
