package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChatService struct {
	operatorRepository    *repository.OperatorRepository
	pilgrimRepository     *repository.PilgrimRepository
	chatRepository        *repository.ChatRepository
	groupLeaderRepository *repository.GroupLeaderRepository
}

func NewChatService(operators *repository.OperatorRepository, pilgrims *repository.PilgrimRepository, chat *repository.ChatRepository, groupLeaders *repository.GroupLeaderRepository) *ChatService {
	return &ChatService{operatorRepository: operators, pilgrimRepository: pilgrims, chatRepository: chat, groupLeaderRepository: groupLeaders}
}

// ListMyMessages / SendMyMessage are public (app_access_code) — the pilgrim
// side of a group's chat board.
func (s *ChatService) ListMyMessages(ctx context.Context, req *hajjv1.ChatAppRequest) (*hajjv1.ListChatMessagesResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("ChatService.ListMyMessages", apperror.ErrValidation)
	}
	pilgrim, err := s.pilgrimRepository.GetByAppAccessCode(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("ChatService.ListMyMessages", apperror.ErrNotFound)
	}
	if pilgrim.GroupID == "" {
		return &hajjv1.ListChatMessagesResponse{}, nil
	}
	messages, err := s.chatRepository.ListByGroup(ctx, pilgrim.OperatorID, pilgrim.GroupID)
	if err != nil {
		return nil, serviceError("ChatService.ListMyMessages", err)
	}
	return chatMessagesResponse(messages), nil
}

func (s *ChatService) SendMyMessage(ctx context.Context, req *hajjv1.SendMyMessageRequest) (*hajjv1.ChatMessage, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" || strings.TrimSpace(req.Body) == "" {
		return nil, serviceError("ChatService.SendMyMessage", apperror.ErrValidation)
	}
	pilgrim, err := s.pilgrimRepository.GetByAppAccessCode(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("ChatService.SendMyMessage", apperror.ErrNotFound)
	}
	if pilgrim.GroupID == "" {
		return nil, serviceError("ChatService.SendMyMessage", apperror.ErrFailedPrecondition)
	}
	message, err := s.chatRepository.CreateFromPilgrim(ctx, pilgrim.OperatorID, pilgrim.GroupID, pilgrim.ID, pilgrim.FullName, req.Body)
	if err != nil {
		return nil, serviceError("ChatService.SendMyMessage", err)
	}
	return chatMessage(message), nil
}

// ListGroupMessages / SendGroupMessage are authenticated — the leader resolves
// their own identity from the session, the group_id in the request is only
// checked against groups they actually lead.
func (s *ChatService) ListGroupMessages(ctx context.Context, orgID string, req *hajjv1.ListGroupMessagesRequest) (*hajjv1.ListChatMessagesResponse, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" {
		return nil, serviceError("ChatService.ListGroupMessages", apperror.ErrValidation)
	}
	op, _, err := s.resolveLeader(ctx, orgID, req.GroupId)
	if err != nil {
		return nil, err
	}
	messages, err := s.chatRepository.ListByGroup(ctx, op.ID, req.GroupId)
	if err != nil {
		return nil, serviceError("ChatService.ListGroupMessages", err)
	}
	return chatMessagesResponse(messages), nil
}

func (s *ChatService) SendGroupMessage(ctx context.Context, orgID string, req *hajjv1.SendGroupMessageRequest) (*hajjv1.ChatMessage, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" || strings.TrimSpace(req.Body) == "" {
		return nil, serviceError("ChatService.SendGroupMessage", apperror.ErrValidation)
	}
	op, leaderID, err := s.resolveLeader(ctx, orgID, req.GroupId)
	if err != nil {
		return nil, err
	}
	leaderName := middleware.UserNameFromCtx(ctx)
	message, err := s.chatRepository.CreateFromUser(ctx, op.ID, req.GroupId, leaderID, leaderName, req.Body)
	if err != nil {
		return nil, serviceError("ChatService.SendGroupMessage", err)
	}
	return chatMessage(message), nil
}

func (s *ChatService) resolveLeader(ctx context.Context, orgID, groupID string) (*domain.Operator, string, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, "", serviceError("ChatService", err)
	}
	leaderID := middleware.UserIDFromCtx(ctx)
	if err := s.groupLeaderRepository.EnsureLeaderOwnsGroup(ctx, op.ID, groupID, leaderID); err != nil {
		return nil, "", serviceError("ChatService", apperror.ErrForbidden)
	}
	return op, leaderID, nil
}

func chatMessage(message *domain.ChatMessage) *hajjv1.ChatMessage {
	return &hajjv1.ChatMessage{Id: message.ID, GroupId: message.GroupID, SenderName: message.SenderName, FromPilgrim: message.FromPilgrim, Body: message.Body, CreatedAt: timestamppb.New(message.CreatedAt)}
}

func chatMessagesResponse(messages []*domain.ChatMessage) *hajjv1.ListChatMessagesResponse {
	result := &hajjv1.ListChatMessagesResponse{Messages: make([]*hajjv1.ChatMessage, 0, len(messages))}
	for _, message := range messages {
		result.Messages = append(result.Messages, chatMessage(message))
	}
	return result
}
