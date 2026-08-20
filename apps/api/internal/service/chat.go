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
	groupRepository       *repository.GroupRepository
	groupLeaderRepository *repository.GroupLeaderRepository
}

func NewChatService(operators *repository.OperatorRepository, pilgrims *repository.PilgrimRepository, chat *repository.ChatRepository, groups *repository.GroupRepository, groupLeaders *repository.GroupLeaderRepository) *ChatService {
	return &ChatService{operatorRepository: operators, pilgrimRepository: pilgrims, chatRepository: chat, groupRepository: groups, groupLeaderRepository: groupLeaders}
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

// ListGroupMessages / SendGroupMessage are authenticated — any member of the
// operator's org can read and post in any of the operator's groups (not just
// a group's assigned leader). That's deliberate: coordinators need to be able
// to monitor every group's chat, not only groups they personally lead.
// group_id is still checked against EnsureGroupBelongsToOperator, since
// without that a caller could pass another operator's group_id.
func (s *ChatService) ListGroupMessages(ctx context.Context, orgID string, req *hajjv1.ListGroupMessagesRequest) (*hajjv1.ListChatMessagesResponse, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" {
		return nil, serviceError("ChatService.ListGroupMessages", apperror.ErrValidation)
	}
	op, err := s.resolveOperatorGroup(ctx, orgID, req.GroupId)
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
	op, err := s.resolveOperatorGroup(ctx, orgID, req.GroupId)
	if err != nil {
		return nil, err
	}
	senderID := middleware.UserIDFromCtx(ctx)
	senderName := middleware.UserNameFromCtx(ctx)
	message, err := s.chatRepository.CreateFromUser(ctx, op.ID, req.GroupId, senderID, senderName, req.Body)
	if err != nil {
		return nil, serviceError("ChatService.SendGroupMessage", err)
	}
	return chatMessage(message), nil
}

// resolveOperatorGroup confirms groupID belongs to this operator, and
// additionally that the caller owns it (leads it) unless they're a real
// staff member (owner/admin) — the operator dashboard's group chat panel
// needs to reach any group, but the leader app's identical RPC call must
// not let one Muttawwif read or post into another Muttawwif's group chat.
func (s *ChatService) resolveOperatorGroup(ctx context.Context, orgID, groupID string) (*domain.Operator, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ChatService", err)
	}
	if err := s.groupRepository.EnsureGroupBelongsToOperator(ctx, op.ID, groupID); err != nil {
		return nil, serviceError("ChatService", apperror.ErrNotFound)
	}
	role := middleware.OrgRoleFromCtx(ctx)
	if role != "owner" && role != "admin" {
		if err := s.groupLeaderRepository.EnsureLeaderOwnsGroup(ctx, op.ID, groupID, middleware.UserIDFromCtx(ctx)); err != nil {
			return nil, serviceError("ChatService", apperror.ErrForbidden)
		}
	}
	return op, nil
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
