package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type ChatHandler struct{ chatService *service.ChatService }

func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}
func (h *ChatHandler) ListMyMessages(ctx context.Context, req *connect.Request[hajjv1.ListMyMessagesRequest]) (*connect.Response[hajjv1.ListChatMessagesResponse], error) {
	result, err := h.chatService.ListMyMessages(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *ChatHandler) SendMyMessage(ctx context.Context, req *connect.Request[hajjv1.SendMyMessageRequest]) (*connect.Response[hajjv1.ChatMessage], error) {
	result, err := h.chatService.SendMyMessage(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *ChatHandler) ListGroupMessages(ctx context.Context, req *connect.Request[hajjv1.ListGroupMessagesRequest]) (*connect.Response[hajjv1.ListChatMessagesResponse], error) {
	result, err := h.chatService.ListGroupMessages(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *ChatHandler) SendGroupMessage(ctx context.Context, req *connect.Request[hajjv1.SendGroupMessageRequest]) (*connect.Response[hajjv1.ChatMessage], error) {
	result, err := h.chatService.SendGroupMessage(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
