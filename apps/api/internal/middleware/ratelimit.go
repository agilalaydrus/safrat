package middleware

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/time/rate"
)

// rateLimitedProcedures caps request rate per client IP for RPCs reachable
// without authentication (see publicProcedures in auth.go) — a session can't
// already provide a cheap identity to throttle by, so this is the one place
// that needs its own abuse guard.
const rateLimitBurst = 5 // requests allowed immediately, refilling at the rate below

// SOSService/CreateSOSAlert is deliberately NOT in this map — see sos.go.
// The PilgrimAppService/ChatService entries are read/poll endpoints a
// legitimate pilgrim's own device calls repeatedly (app open, periodic
// refresh, chat polling), so they get a much looser per-IP ceiling than a
// one-shot form like ApplyAsAgent.
var rateLimitedProcedures = map[string]rate.Limit{
	"/hajj.v1.AgentService/ApplyAsAgent":          rate.Every(time.Hour / rateLimitBurst), // 5 per hour per IP
	"/hajj.v1.PilgrimAppService/GetMyInfo":        rate.Every(time.Minute / 4),            // 4 per minute per IP
	"/hajj.v1.PilgrimAppService/ListMySchedule":   rate.Every(time.Minute / 4),
	"/hajj.v1.PilgrimAppService/ListMyProducts":   rate.Every(time.Minute / 4),
	"/hajj.v1.PilgrimAppService/UpdateMyLocation": rate.Every(time.Minute / 4), // pings every 5min; allows retries
	"/hajj.v1.ChatService/ListMyMessages":         rate.Every(time.Second * 3), // supports ~3s polling
	"/hajj.v1.ChatService/SendMyMessage":          rate.Every(time.Minute / 10),
}

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	limiters map[string]map[string]*ipLimiter // procedure -> client IP -> limiter
}

// NewRateLimitInterceptor throttles the procedures listed in
// rateLimitedProcedures by client IP. State is in-memory and per-process —
// fine for the single-API-instance deployment in DEPLOY.md; move to Redis if
// the API ever runs more than one replica.
func NewRateLimitInterceptor() connect.Interceptor {
	limiter := &rateLimiter{limiters: make(map[string]map[string]*ipLimiter)}
	go limiter.cleanupLoop()
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			limit, limited := rateLimitedProcedures[request.Spec().Procedure]
			if !limited {
				return next(ctx, request)
			}
			if !limiter.allow(request.Spec().Procedure, clientIP(request), limit) {
				return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("too many requests — try again later"))
			}
			return next(ctx, request)
		}
	})
}

func (rl *rateLimiter) allow(procedure, ip string, limit rate.Limit) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	perIP, ok := rl.limiters[procedure]
	if !ok {
		perIP = make(map[string]*ipLimiter)
		rl.limiters[procedure] = perIP
	}
	entry, ok := perIP[ip]
	if !ok {
		entry = &ipLimiter{limiter: rate.NewLimiter(limit, rateLimitBurst)}
		perIP[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter.Allow()
}

// cleanupLoop evicts idle entries so a long-running process doesn't
// accumulate one limiter per distinct IP forever.
func (rl *rateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-time.Hour)
		rl.mu.Lock()
		for procedure, perIP := range rl.limiters {
			for ip, entry := range perIP {
				if entry.lastSeen.Before(cutoff) {
					delete(perIP, ip)
				}
			}
			if len(perIP) == 0 {
				delete(rl.limiters, procedure)
			}
		}
		rl.mu.Unlock()
	}
}

// clientIP trusts X-Real-IP/X-Forwarded-For because DEPLOY.md's nginx config
// always sets X-Real-IP to $remote_addr itself — a client can't override what
// nginx forwards. Falls back to the raw peer address for local dev without a
// proxy in front.
func clientIP(request connect.AnyRequest) string {
	if real := strings.TrimSpace(request.Header().Get("X-Real-IP")); real != "" {
		return real
	}
	if forwarded := request.Header().Get("X-Forwarded-For"); forwarded != "" {
		if first := strings.TrimSpace(strings.Split(forwarded, ",")[0]); first != "" {
			return first
		}
	}
	addr := request.Peer().Addr
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
