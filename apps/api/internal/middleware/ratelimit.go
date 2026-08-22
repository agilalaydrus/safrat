package middleware

import (
	"context"
	"errors"
	"net"
	"net/http"
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
	"/hajj.v1.AgentService/ApplyAsAgent":           rate.Every(time.Hour / rateLimitBurst), // 5 per hour per IP
	"/hajj.v1.PilgrimAppService/GetMyInfo":         rate.Every(time.Minute / 4),            // 4 per minute per IP
	"/hajj.v1.PilgrimAppService/ListMySchedule":    rate.Every(time.Minute / 4),
	"/hajj.v1.PilgrimAppService/ListMyProducts":    rate.Every(time.Minute / 4),
	"/hajj.v1.PilgrimAppService/UpdateMyLocation":  rate.Every(time.Minute / 4), // pings every 5min; allows retries
	"/hajj.v1.PilgrimAppService/RequestWheelchair": rate.Every(time.Minute / 4),
	"/hajj.v1.SOSService/ListMyPilgrimSOSAlerts":   rate.Every(time.Second * 3), // supports ~3s polling
	"/hajj.v1.ChatService/ListMyMessages":          rate.Every(time.Second * 3), // supports ~3s polling
	"/hajj.v1.ChatService/SendMyMessage":           rate.Every(time.Minute / 10),
	// A real checkout is a one-shot action, not a poll — tight ceiling,
	// closer to ApplyAsAgent than to the read endpoints above.
	"/hajj.v1.OrderService/CreateOrder":           rate.Every(time.Hour / rateLimitBurst), // 5 per hour per IP
	"/hajj.v1.PilgrimAppService/ListMyBroadcasts": rate.Every(time.Minute / 4),
	// A public registration submission is a one-shot form, same tier as
	// ApplyAsAgent/CreateOrder. GetRegistrationForm is read-only (loading
	// the form itself) so it gets the looser read-endpoint ceiling instead.
	"/hajj.v1.RegistrationService/SubmitRegistration":  rate.Every(time.Minute / rateLimitBurst), // 5 per minute per IP
	"/hajj.v1.RegistrationService/GetRegistrationForm": rate.Every(time.Minute / 4),
	"/hajj.v1.WaitlistService/JoinWaitlist":            rate.Every(time.Hour / rateLimitBurst), // 5 per hour per IP
	"/hajj.v1.WaitlistService/LeaveWaitlist":           rate.Every(time.Hour / rateLimitBurst),
	"/hajj.v1.WaitlistService/ConfirmWaitlistSlot":     rate.Every(time.Hour / rateLimitBurst),
	// Tight ceiling — this is the one endpoint where the identity token
	// itself (app_access_code) is guessable-by-brute-force in principle;
	// a legitimate family member checks at most a few times per hour.
	"/hajj.v1.FamilyTrackerService/GetFamilyStatus":     rate.Every(time.Minute),
	"/hajj.v1.ChecklistService/GetMyChecklist":          rate.Every(time.Minute / 4),
	"/hajj.v1.ChecklistService/CompleteMyChecklistItem": rate.Every(time.Minute / 4),
	// Once lost, a pilgrim doesn't spam this — but allow a few retries if
	// the first push silently fails.
	"/hajj.v1.LostReportService/ReportLost": rate.Every(time.Hour / 3), // 3 per hour per IP
	// Loose — a jamaah might share and view their certificate multiple times.
	"/hajj.v1.PilgrimAppService/GetMyCertificate": rate.Every(time.Minute / 4),
	// A pilgrim filling in KYC fields might retry a typo a few times.
	"/hajj.v1.PilgrimAppService/SubmitMyPilgrimKyc":  rate.Every(time.Minute / 6),
	"/hajj.v1.PilgrimAppService/ListMyRituals":       rate.Every(time.Minute / 4),
	"/hajj.v1.PilgrimAppService/RegisterMyPushToken": rate.Every(time.Minute / 4),
	// Called from apps/web/middleware.ts (server-side, not the visitor's own
	// browser) on every subdomain page load, so this needs a much looser
	// ceiling than a real per-visitor endpoint — middleware.ts also caches
	// the slug->operatorId mapping in-memory to keep actual call volume low.
	"/hajj.v1.OperatorService/ResolveOperatorSlug": rate.Every(time.Second / 5), // 5 per second per IP
	"/hajj.v1.SeasonService/ResolveSeasonSlug":     rate.Every(time.Second / 5),
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
// Implemented as a full connect.Interceptor, not connect.UnaryInterceptorFunc
// — see the identical rationale on authInterceptor in auth.go. No procedure
// in rateLimitedProcedures is a streaming RPC today, but a
// connect.UnaryInterceptorFunc would silently skip rate limiting entirely
// for one if it ever were, rather than erroring loudly.
func NewRateLimitInterceptor() connect.Interceptor {
	limiter := &rateLimiter{limiters: make(map[string]map[string]*ipLimiter)}
	go limiter.cleanupLoop()
	return &rateLimitInterceptor{limiter: limiter}
}

type rateLimitInterceptor struct{ limiter *rateLimiter }

func (r *rateLimitInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if err := r.check(request.Spec().Procedure, clientIP(request.Header(), request.Peer().Addr)); err != nil {
			return nil, err
		}
		return next(ctx, request)
	}
}

func (r *rateLimitInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (r *rateLimitInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := r.check(conn.Spec().Procedure, clientIP(conn.RequestHeader(), conn.Peer().Addr)); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (r *rateLimitInterceptor) check(procedure, ip string) error {
	limit, limited := rateLimitedProcedures[procedure]
	if !limited {
		return nil
	}
	if !r.limiter.allow(procedure, ip, limit) {
		return connect.NewError(connect.CodeResourceExhausted, errors.New("too many requests — try again later"))
	}
	return nil
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
func clientIP(header http.Header, peerAddr string) string {
	if real := strings.TrimSpace(header.Get("X-Real-IP")); real != "" {
		return real
	}
	if forwarded := header.Get("X-Forwarded-For"); forwarded != "" {
		if first := strings.TrimSpace(strings.Split(forwarded, ",")[0]); first != "" {
			return first
		}
	}
	if host, _, err := net.SplitHostPort(peerAddr); err == nil {
		return host
	}
	return peerAddr
}
