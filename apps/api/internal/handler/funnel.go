package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/service"
)

type FunnelHandler struct {
	funnelService *service.FunnelService
	ingestSecret  []byte
	now           func() time.Time
}

func NewFunnelHandler(funnelService *service.FunnelService, ingestSecret string) *FunnelHandler {
	return &FunnelHandler{
		funnelService: funnelService,
		ingestSecret:  []byte(strings.TrimSpace(ingestSecret)),
		now:           time.Now,
	}
}

func (h *FunnelHandler) RecordEvent(ctx context.Context, req *connect.Request[hajjv1.RecordEventRequest]) (*connect.Response[hajjv1.RecordEventResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		// Still a success to the caller. A malformed beacon must not turn into
		// an error a page might surface to a visitor.
		return connect.NewResponse(&hajjv1.RecordEventResponse{}), nil
	}
	result, err := h.funnelService.RecordEvent(ctx, req.Msg,
		h.clientIP(req.Header(), req.Peer().Addr), req.Header().Get("User-Agent"))
	if err != nil {
		return connect.NewResponse(&hajjv1.RecordEventResponse{}), nil
	}
	return connect.NewResponse(result), nil
}

const funnelSignatureMaxAge = 2 * time.Minute

// clientIP accepts the original visitor address forwarded by the web
// middleware only when it carries a fresh HMAC. Public requests still use the
// edge-controlled proxy headers below, so an internet client cannot choose
// the identity used for counting.
func (h *FunnelHandler) clientIP(header http.Header, peerAddr string) string {
	forwarded := strings.TrimSpace(header.Get("X-Funnel-Client-IP"))
	timestamp := strings.TrimSpace(header.Get("X-Funnel-Timestamp"))
	signature := strings.TrimSpace(header.Get("X-Funnel-Signature"))
	if len(h.ingestSecret) >= 32 && forwarded != "" && timestamp != "" && signature != "" {
		unixSeconds, err := strconv.ParseInt(timestamp, 10, 64)
		if err == nil {
			age := h.now().UTC().Sub(time.Unix(unixSeconds, 0).UTC())
			if age >= -funnelSignatureMaxAge && age <= funnelSignatureMaxAge {
				provided, decodeErr := hex.DecodeString(signature)
				mac := hmac.New(sha256.New, h.ingestSecret)
				_, _ = mac.Write([]byte(timestamp + "\n" + forwarded + "\n" + header.Get("User-Agent")))
				if decodeErr == nil && hmac.Equal(provided, mac.Sum(nil)) {
					return forwarded
				}
			}
		}
	}
	return funnelClientIP(header, peerAddr)
}

// funnelClientIP trusts the proxy headers for the same reason the rate limiter
// does: nginx sets them and the app is not reachable except through it.
//
// The address is used only to derive a daily token and is never stored, so a
// forged header costs an inflated count and nothing else. In production Caddy
// overwrites both headers at the edge; the signed path above is only for the
// internal web-to-API request.
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
