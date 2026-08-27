package handler

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// WebhookSourceGuard restricts who may deliver a webhook, by source address.
//
// It is the third of three independent controls, and deliberately not the one
// anything depends on: the callback token authenticates the delivery, and
// settling from Xendit's own API means even a forged delivery cannot move
// money. This narrows the surface so a forged delivery cannot be attempted at
// all — and, just as usefully, keeps random internet scanners off a money
// endpoint.
//
// Addresses are the gateway's published egress ranges, given as
// XENDIT_WEBHOOK_ALLOWED_IPS (comma-separated IPs or CIDRs).
type WebhookSourceGuard struct {
	networks []*net.IPNet
	logger   *slog.Logger
}

// NewWebhookSourceGuard parses the allowlist.
//
// An empty allowlist permits every source, and says so loudly at startup. That
// is a deliberate choice: failing closed here would stop every payment from
// settling the moment the variable is missing or a gateway silently changes
// its egress ranges, which is a worse outcome than a wider surface behind two
// other controls. The warning exists so "wider surface" never becomes
// invisible.
func NewWebhookSourceGuard(logger *slog.Logger, allowed string) *WebhookSourceGuard {
	guard := &WebhookSourceGuard{logger: logger}
	for _, entry := range strings.Split(allowed, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			// A bare address is a /32 or /128.
			if ip := net.ParseIP(entry); ip != nil {
				bits := 32
				if ip.To4() == nil {
					bits = 128
				}
				guard.networks = append(guard.networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
				continue
			}
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			logger.Error("webhook allowlist: ignoring unparseable entry", "entry", entry, "error", err)
			continue
		}
		guard.networks = append(guard.networks, network)
	}
	if len(guard.networks) == 0 {
		logger.Warn("webhook source allowlist is empty; deliveries are accepted from any address",
			"remedy", "set XENDIT_WEBHOOK_ALLOWED_IPS to the gateway's published egress ranges")
	}
	return guard
}

// Allows reports whether a request may deliver a webhook.
func (g *WebhookSourceGuard) Allows(r *http.Request) bool {
	if g == nil || len(g.networks) == 0 {
		return true
	}
	ip := net.ParseIP(clientAddress(r))
	if ip == nil {
		return false
	}
	for _, network := range g.networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// clientAddress is the caller's address as seen by the edge.
//
// X-Real-IP is trusted for the same reason the rate limiter trusts it: nginx
// always sets it itself (DEPLOY.md §7), overwriting anything a client sent. On
// a deployment without that proxy the header is absent, and the connection's
// own address is used — never a client-supplied one.
func clientAddress(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Real-IP")); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
