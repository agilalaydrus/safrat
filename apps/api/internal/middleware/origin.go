package middleware

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"
)

// VerifiedHostnameSource supplies the client-owned hostnames that are allowed
// to call the API. It is backed by operator_domains, so routing, CORS, and TLS
// issuance all trust exactly the same table.
type VerifiedHostnameSource interface {
	ListVerifiedHostnames(context.Context) ([]string, error)
}

// TenantOriginAllowlist answers "may this Origin call us?" for client-owned
// domains, on top of whatever the statically configured origin already allows.
//
// The set is cached because it is consulted on every request. A refresh failure
// deliberately keeps serving the last known good set rather than locking every
// tenant out of the API over a transient database blip; the window is bounded
// by the TTL, and a hostname only ever enters the set by being verified.
type TenantOriginAllowlist struct {
	source      VerifiedHostnameSource
	logger      *slog.Logger
	ttl         time.Duration
	allowPlain  bool
	mutex       sync.RWMutex
	hostnames   map[string]struct{}
	refreshedAt time.Time
}

// NewTenantOriginAllowlist builds the allowlist. allowPlainHTTP must only be
// true for local development: in production an https page must never be able to
// fall back to an http origin we would then echo into
// Access-Control-Allow-Origin.
func NewTenantOriginAllowlist(source VerifiedHostnameSource, logger *slog.Logger, ttl time.Duration, allowPlainHTTP bool) *TenantOriginAllowlist {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &TenantOriginAllowlist{source: source, logger: logger, ttl: ttl, allowPlain: allowPlainHTTP, hostnames: map[string]struct{}{}}
}

// Allows reports whether the origin is a verified client domain. The match is
// on the exact hostname — a verified umrohvacana.com must not also admit
// anything.umrohvacana.com, which the operator may not control.
func (a *TenantOriginAllowlist) Allows(ctx context.Context, origin string) bool {
	if a == nil || origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if parsed.Scheme != "https" && !(a.allowPlain && parsed.Scheme == "http") {
		return false
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return false
	}
	a.refresh(ctx)
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	_, ok := a.hostnames[hostname]
	return ok
}

func (a *TenantOriginAllowlist) refresh(ctx context.Context) {
	a.mutex.RLock()
	fresh := time.Since(a.refreshedAt) < a.ttl
	a.mutex.RUnlock()
	if fresh {
		return
	}
	hostnames, err := a.source.ListVerifiedHostnames(ctx)
	if err != nil {
		// Keep the previous set; see the type comment.
		if a.logger != nil {
			a.logger.Warn("refresh tenant origin allowlist", "error", err)
		}
		a.mutex.Lock()
		a.refreshedAt = time.Now().Add(-a.ttl / 2) // retry sooner than a full TTL
		a.mutex.Unlock()
		return
	}
	next := make(map[string]struct{}, len(hostnames))
	for _, hostname := range hostnames {
		next[strings.ToLower(hostname)] = struct{}{}
	}
	a.mutex.Lock()
	a.hostnames = next
	a.refreshedAt = time.Now()
	a.mutex.Unlock()
}
