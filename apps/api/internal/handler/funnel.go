package handler

import (
	"context"
	"net/http"
	"strings"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/service"
)

type FunnelHandler struct {
	funnelService *service.FunnelService
}

func NewFunnelHandler(funnelService *service.FunnelService) *FunnelHandler {
	return &FunnelHandler{funnelService: funnelService}
}

func (h *FunnelHandler) RecordEvent(ctx context.Context, req *connect.Request[hajjv1.RecordFunnelEventRequest]) (*connect.Response[hajjv1.RecordFunnelEventResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		// Still a success to the caller. A malformed beacon must not turn into
		// an error a page might surface to a visitor.
		return connect.NewResponse(&hajjv1.RecordFunnelEventResponse{}), nil
	}
	result, err := h.funnelService.RecordEvent(ctx, req.Msg,
		funnelClientIP(req.Header(), req.Peer().Addr), req.Header().Get("User-Agent"))
	if err != nil {
		return connect.NewResponse(&hajjv1.RecordFunnelEventResponse{}), nil
	}
	return connect.NewResponse(result), nil
}

// funnelClientIP trusts the proxy headers for the same reason the rate limiter
// does: nginx sets them and the app is not reachable except through it.
//
// The address is used only to derive a daily token and is never stored, so a
// forged header costs an inflated count and nothing else.
func funnelClientIP(header http.Header, peerAddr string) string {
	if value := strings.TrimSpace(header.Get("X-Real-IP")); value != "" {
		return value
	}
	if value := strings.TrimSpace(header.Get("X-Forwarded-For")); value != "" {
		if first, _, found := strings.Cut(value, ","); found {
			return strings.TrimSpace(first)
		}
		return value
	}
	return peerAddr
}
