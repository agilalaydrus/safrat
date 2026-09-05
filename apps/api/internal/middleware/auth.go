package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const (
	ctxKeyUserID     contextKey = "user_id"
	ctxKeyOperatorID contextKey = "operator_id"
	ctxKeyUserName   contextKey = "user_name"
	ctxKeyUserEmail  contextKey = "user_email"
	ctxKeyOrgRole    contextKey = "org_role"
	ctxKeySessionID  contextKey = "session_id"
	ctxKeyClientIP   contextKey = "client_ip"
)

// publicProcedures lists RPCs that must be reachable without a Better Auth session —
// e.g. a prospective agent applying via a public link. Keep this list minimal and
// treat every addition as a security decision: identity fields these handlers need
// (like operator_id) must come from the request body instead of ctx, and the service
// layer must validate them explicitly since there's no session to trust.
var publicProcedures = map[string]bool{
	"/hajj.v1.AgentService/ApplyAsAgent":           true,
	"/hajj.v1.PilgrimAppService/GetMyInfo":         true,
	"/hajj.v1.PilgrimAppService/ListMySchedule":    true,
	"/hajj.v1.PilgrimAppService/ListMyProducts":    true,
	"/hajj.v1.PilgrimAppService/UpdateMyLocation":  true,
	"/hajj.v1.PilgrimAppService/RequestWheelchair": true,
	"/hajj.v1.SOSService/CreateSOSAlert":           true,
	"/hajj.v1.SOSService/ListMyPilgrimSOSAlerts":   true,
	"/hajj.v1.ChatService/ListMyMessages":          true,
	"/hajj.v1.ChatService/SendMyMessage":           true,
	// CreateOrder touches money — see internal/service/order.go. Public for
	// the same reason as the rest of PilgrimAppService (a pilgrim checking
	// out has no Better Auth session, only app_access_code), rate-limited
	// like every other entry here (see ratelimit.go), and never trusts a
	// client-supplied operator/season/price — every value is re-derived
	// server-side from the pilgrim record and the product row.
	"/hajj.v1.OrderService/CreateOrder":           true,
	"/hajj.v1.PilgrimAppService/ListMyBroadcasts": true,
	// SubmitRegistration/GetRegistrationForm: a prospective pilgrim filling
	// out the public registration form (see registration.proto) has no
	// pilgrim record yet, so no app_access_code either — authenticated only
	// by knowing a real operator_id+season_id, re-validated server-side on
	// every call (see RegistrationService.Submit/GetForm).
	"/hajj.v1.RegistrationService/SubmitRegistration":  true,
	"/hajj.v1.RegistrationService/GetRegistrationForm": true,
	// WaitlistService — a prospective jamaah joining/leaving/confirming a
	// waitlist slot has no session either; identity is email+operator_id+
	// season_id from the body, re-validated server-side (see waitlist.go).
	"/hajj.v1.WaitlistService/JoinWaitlist":        true,
	"/hajj.v1.WaitlistService/LeaveWaitlist":       true,
	"/hajj.v1.WaitlistService/ConfirmWaitlistSlot": true,
	// FamilyTrackerService — authenticated by app_access_code only, no
	// session of any kind. See family_tracker.proto for the exposed-field
	// allowlist that keeps this from leaking PII.
	"/hajj.v1.FamilyTrackerService/GetFamilyStatus":   true,
	"/hajj.v1.FamilyTrackerService/ListFamilyMoments": true,
	// ChecklistService pilgrim-app RPCs — app_access_code authenticated,
	// same pattern as the rest of PilgrimAppService.
	"/hajj.v1.ChecklistService/GetMyChecklist":          true,
	"/hajj.v1.ChecklistService/CompleteMyChecklistItem": true,
	// LostReportService/ReportLost — a pilgrim tapping "Saya Tersesat" has
	// no session, only app_access_code; pilgrim_id/operator_id/group_id are
	// always derived server-side from it (see LostReportService.ReportLost).
	"/hajj.v1.LostReportService/ReportLost": true,
	// PilgrimAppService/GetMyCertificate — same public, code-authenticated
	// pattern as the rest of this service.
	"/hajj.v1.PilgrimAppService/GetMyCertificate": true,
	// PilgrimAppService/SubmitMyPilgrimKyc — same public, code-authenticated
	// pattern as the rest of this service; pilgrim_id/operator_id are always
	// derived server-side from app_access_code, never trusted from the body.
	"/hajj.v1.PilgrimAppService/SubmitMyPilgrimKyc": true,
	// PilgrimAppService/ListMyRituals, RegisterMyPushToken — same
	// public, code-authenticated pattern as the rest of this service.
	"/hajj.v1.PilgrimAppService/ListMyRituals":       true,
	"/hajj.v1.PilgrimAppService/RegisterMyPushToken": true,
	// OperatorService/ResolveOperatorSlug — apps/web/middleware.ts calls this
	// on every subdomain request (before any session exists) to map a slug
	// like "vacana" to an operator ID. Returns only id + name.
	"/hajj.v1.OperatorService/ResolveOperatorSlug": true,
	// Same reason: apps/web/middleware.ts resolves a client's own domain before
	// any session exists. Only verified domains resolve.
	"/hajj.v1.OperatorService/ResolveOperatorDomain": true,
	// FunnelService records a visitor step. Public by necessity: the whole
	// point is the visitor who has not identified themselves. It writes only,
	// reads nothing back, and stores no address — the reporting side lives on
	// the authenticated services, scoped to one operator.
	"/hajj.v1.FunnelService/RecordEvent": true,
	// Onboarding availability check contains no tenant data and runs before
	// the new operator record exists.
	"/hajj.v1.OperatorService/CheckOperatorSlug": true,
	// OperatorService/GetPublicProfile — the shareable operator landing page
	// at {slug}.tawafiqhub.id, fetched server-side with no session. Returns only
	// non-sensitive profile fields plus available seasons.
	"/hajj.v1.OperatorService/GetPublicProfile": true,
	// SeasonService/ResolveSeasonSlug — same pattern, for an explicit-season
	// subdomain link (vacana.tawafiqhub.id/register/musim-haji-2026).
	"/hajj.v1.SeasonService/ResolveSeasonSlug": true,
	// InquiryService/SubmitInquiry — the storefront's "Hubungi Kami" form.
	// Same trust boundary as SubmitRegistration: no session, authenticated
	// only by a real operator_id, re-validated server-side on every call
	// (see InquiryService.Submit).
	"/hajj.v1.InquiryService/SubmitInquiry": true,
}

// sessionOnlyProcedures lists RPCs that require a real, server-validated
// Better Auth session (so identity can't be spoofed via the request body)
// but must NOT require organization membership like every other
// authenticated RPC does — a pilgrim's Google identity is never an org
// member. Keep this list minimal for the same reason as publicProcedures:
// every entry is a security decision, and its service method must derive
// operator/tenant scoping from elsewhere (e.g. the pilgrim record looked up
// by app_access_code), never from ctx's operator id, which is empty here.
var sessionOnlyProcedures = map[string]bool{
	"/hajj.v1.PilgrimAppService/LinkGoogleAccount": true,
	"/hajj.v1.IdentityService/GetMyAccess":         true,
	"/hajj.v1.IdentityService/InvalidateMyAccess":  true,
	// ListMyTransactions is deliberately here rather than in
	// publicProcedures, unlike the rest of PilgrimAppService. Payment history
	// is a step up from a schedule: app_access_code alone should not open it,
	// because that code also travels through links and caches. The service
	// additionally requires the presented code to be the one belonging to this
	// session's own pilgrim, and takes the pilgrim id from the session rather
	// than the request.
	"/hajj.v1.PilgrimAppService/ListMyTransactions": true,
	// Refund wallet reads and payout requests are money-sensitive and must
	// never open from an app_access_code alone. Their service additionally
	// binds that code to this session's linked pilgrim; payout creation also
	// requires the Better Auth account to have 2FA enabled.
	"/hajj.v1.RefundPayoutService/GetMyRefundWallet":        true,
	"/hajj.v1.RefundPayoutService/RequestRefundPayout":      true,
	"/hajj.v1.RefundPayoutService/GetMyAgentRefundWallet":   true,
	"/hajj.v1.RefundPayoutService/RequestAgentRefundPayout": true,
	// PlatformService — TawafiqHub's own admin surface. Session-only because a
	// platform admin is a Better Auth user who need not belong to any operator;
	// requiring org membership would force platform staff into somebody's
	// tenant to do platform work.
	//
	// This grants *reachability*, not access. Every method except
	// AmIPlatformAdmin calls requirePlatformAdmin, which checks the
	// platform_admins table on each request. AmIPlatformAdmin answers only
	// about the caller, so any signed-in user may ask.
	"/hajj.v1.PlatformService/AmIPlatformAdmin":         true,
	"/hajj.v1.PlatformService/ListOperators":            true,
	"/hajj.v1.PlatformService/ListPlanLimits":           true,
	"/hajj.v1.PlatformService/PreviewPlanLimitChange":   true,
	"/hajj.v1.PlatformService/SetPlanLimit":             true,
	"/hajj.v1.PlatformService/ListPlanOverrides":        true,
	"/hajj.v1.PlatformService/SetPlanOverride":          true,
	"/hajj.v1.PlatformService/DeletePlanOverride":       true,
	"/hajj.v1.PlatformService/ListUsage":                true,
	"/hajj.v1.PlatformService/SetTrialDays":             true,
	"/hajj.v1.PlatformService/ListSubscriptionInvoices": true,
	"/hajj.v1.PlatformService/VoidSubscriptionInvoice":  true,
	"/hajj.v1.PlatformService/ListProductsNeedingCost":  true,
	"/hajj.v1.PlatformService/SetProductSupplierCost":   true,
	// The supplier catalogue — same gate, same reason: every one of these
	// checks requirePlatformAdmin before touching anything.
	"/hajj.v1.PlatformService/ListSuppliers":         true,
	"/hajj.v1.PlatformService/SaveSupplier":          true,
	"/hajj.v1.PlatformService/ListProductRoutes":     true,
	"/hajj.v1.PlatformService/SaveProductRoute":      true,
	"/hajj.v1.PlatformService/ListResponseRules":     true,
	"/hajj.v1.PlatformService/CreateResponseRule":    true,
	"/hajj.v1.PlatformService/SetResponseRuleActive": true,
	"/hajj.v1.PlatformService/TestResponseRules":     true,
	"/hajj.v1.PlatformService/ListSupplierLogs":      true,
	"/hajj.v1.PlatformService/ListTransactions":      true,
	// Accounts, platform access and identity records. Same gate: every one
	// checks requirePlatformAdmin, and GetKycRecord audit-logs each read.
	"/hajj.v1.PlatformService/ListAccounts":        true,
	"/hajj.v1.PlatformService/GrantPlatformAdmin":  true,
	"/hajj.v1.PlatformService/RevokePlatformAdmin": true,
	"/hajj.v1.PlatformService/RevokeSessions":      true,
	"/hajj.v1.PlatformService/ListKycRecords":      true,
	"/hajj.v1.PlatformService/GetKycRecord":        true,
	"/hajj.v1.PlatformService/SetKycStatus":        true,
}

// restrictedMemberProcedures lists RPCs a "restricted member" — an org
// member who is a Muttawwif (leads a group) and/or a Tour Leader (linked
// agent), and not owner/admin — may call. Everything else on the
// authenticated (non-public) surface is refused with PermissionDenied,
// even though such an identity does hold a valid org-member session that
// would otherwise satisfy every dashboard RPC's org-scoping check. This is
// the actual enforcement of "a Muttawwif/Tour Leader only gets their own
// portal, not the operator dashboard" — RequireAccess on the frontend is
// UX, this is the real boundary. A plain "member" who is neither a leader
// nor an agent is NOT restricted (see resolveLandingPath's rule 3 — that
// identity's only surface IS the dashboard).
//
// Kept deliberately small and reviewed as a security decision, same as
// publicProcedures. A few reads with no pilgrim-level or financial detail
// (season/kloter/movement listings) are allowed operator-wide rather than
// building dedicated scoped RPCs for every one of them — documented next
// to each entry below.
var restrictedMemberProcedures = map[string]bool{
	// GroupLeaderService — every method is already self-scoped to groups
	// this identity leads (EnsureLeaderOwnsGroup / EnsureLeaderOwnsPilgrim).
	"/hajj.v1.GroupLeaderService/ListMyGroups":             true,
	"/hajj.v1.GroupLeaderService/GetGroupRoster":           true,
	"/hajj.v1.GroupLeaderService/ListCheckIns":             true,
	"/hajj.v1.GroupLeaderService/CreateCheckIn":            true,
	"/hajj.v1.GroupLeaderService/ListMySOSAlerts":          true,
	"/hajj.v1.GroupLeaderService/CheckInGroupPilgrimHotel": true,
	"/hajj.v1.GroupLeaderService/AcknowledgeMySOSAlert":    true,
	"/hajj.v1.GroupLeaderService/ResolveMySOSAlert":        true,
	// TripService — every method is already self-scoped to kloters this
	// identity is assigned staff on (EnsureStaffAssignedToKloter).
	"/hajj.v1.TripService/GetTripRoster":           true,
	"/hajj.v1.TripService/SetTripHotelCheckIn":     true,
	"/hajj.v1.TripService/ListTripMovements":       true,
	"/hajj.v1.TripService/ListTripCheckIns":        true,
	"/hajj.v1.TripService/CreateTripCheckIn":       true,
	"/hajj.v1.TripService/ListTripSOSAlerts":       true,
	"/hajj.v1.TripService/AcknowledgeTripSOSAlert": true,
	"/hajj.v1.TripService/ResolveTripSOSAlert":     true,
	// AgentService — self-scoped wallet/referral methods only. Deliberately
	// NOT ListPayoutRequests/RecordAgentPayout/RejectPayoutRequest/
	// ListAgents/etc — those are the operator's payout inbox and agent
	// roster, not a Tour Leader's own data.
	"/hajj.v1.AgentService/GetMyWallet":                true,
	"/hajj.v1.AgentService/RequestAgentPayout":         true,
	"/hajj.v1.AgentService/ListMyPilgrims":             true,
	"/hajj.v1.AgentService/ListMyReferredTransactions": true,
	// OrderService/CreateOrderForPilgrim — an agent or Muttawwif selling to any
	// jamaah of their operator. Selling is deliberately open, but the
	// commission still follows the jamaah's referral rather than the seller,
	// so this cannot be used to earn from somebody else's referral. The
	// operator boundary still holds: the caller's agent record is resolved
	// from their own identity, and every id in the request is re-checked
	// against that operator.
	//
	// Deliberately NOT CreateManualOrder, whose CASH/BANK_TRANSFER paths mark
	// an order PAID on the caller's word alone, nor RefundOrder, which moves
	// money back out.
	"/hajj.v1.OrderService/CreateOrderForPilgrim":   true,
	"/hajj.v1.OrderService/ListMyPurchaseCatalogue": true,
	"/hajj.v1.OrderService/CreateOrderForSelf":      true,
	"/hajj.v1.OrderService/ListMyOrders":            true,
	// KYC self-service — same "resolve own agent from identity" scoping as
	// GetMyWallet. Works for a Muttawwif too (EnsureAgentForLeader).
	"/hajj.v1.AgentService/SubmitMyAgentKyc":     true,
	"/hajj.v1.AgentService/GetMyAgentKyc":        true,
	"/hajj.v1.AgentService/ListMyAgentDocuments": true,
	// StaffScheduleService — read-only, own assignments only.
	"/hajj.v1.StaffScheduleService/ListMyAssignments": true,
	// ChatService — group_id-scoped; ChatService itself additionally
	// enforces EnsureLeaderOwnsGroup for any non-owner/admin caller (see
	// service/chat.go), so listing this here does not by itself grant
	// cross-group access.
	"/hajj.v1.ChatService/ListGroupMessages": true,
	"/hajj.v1.ChatService/SendGroupMessage":  true,
	// LostReportService — already scoped via EnsureLeaderOwnsGroup.
	"/hajj.v1.LostReportService/ListGroupLostReports":   true,
	"/hajj.v1.LostReportService/ResolveGroupLostReport": true,
	// NotificationService — registers only the caller's own push token.
	"/hajj.v1.NotificationService/RegisterPushSubscription": true,
	// Low-sensitivity, read-only, operator-wide listings with no
	// pilgrim-level or financial detail — pragmatically allowed rather than
	// building a dedicated scoped RPC for each. Revisit if that changes
	// (e.g. movements gain per-pilgrim manifest data in the response).
	"/hajj.v1.SeasonService/ListSeasons":      true,
	"/hajj.v1.KloterService/ListKloters":      true,
	"/hajj.v1.TransportService/ListMovements": true,
	// GroupService — ListOperatorMembers is the picker GroupFormDialog uses
	// (dashboard-only in practice, but read-only and non-sensitive — just
	// org member names/emails already visible via the invite flow).
	"/hajj.v1.GroupService/ListOperatorMembers": true,
}

// billingProcedures stay reachable while a subscription has lapsed. Locking the
// dashboard is meant to prompt payment, not to trap the operator: without these
// they could not see what they owe, or pay it.
var billingProcedures = map[string]bool{
	"/hajj.v1.SubscriptionService/GetMySubscription": true,
	"/hajj.v1.SubscriptionService/ListMyInvoices":    true,
	"/hajj.v1.SubscriptionService/CreateInvoice":     true,
}

// securitySettingsProcedures stay reachable regardless of the IP allowlist
// check below, for the same reason billingProcedures stay reachable
// regardless of the subscription gate: an operator locked out by their own
// misconfigured allowlist (a changed home IP, a typo'd CIDR) must still be
// able to reach the one screen that can fix it, or the feature that exists
// to add security becomes a way to lose access to the account entirely.
var securitySettingsProcedures = map[string]bool{
	"/hajj.v1.SecuritySettingsService/GetSecurityPosture": true,
	"/hajj.v1.SecuritySettingsService/SetIpAllowlist":     true,
	"/hajj.v1.SecuritySettingsService/ListActiveSessions": true,
	"/hajj.v1.SecuritySettingsService/RevokeSession":      true,
}

// NewAuthInterceptor validates Better Auth's opaque database session token.
// Better Auth does not issue JWTs for its default session strategy.
//
// Implemented as a full connect.Interceptor (WrapUnary + WrapStreamingClient
// + WrapStreamingHandler), not connect.UnaryInterceptorFunc — that helper
// only wraps unary RPCs and silently no-ops on streaming ones, which would
// mean a server-streaming RPC like MonitoringService.StreamEvents reaches
// its handler with zero authentication and an empty operator ID on ctx.
// Both WrapUnary and WrapStreamingHandler below call the same authenticate
// helper so the two paths can never drift.
func NewAuthInterceptor(pool *pgxpool.Pool, identityRepository *repository.IdentityRepository, subscriptions *repository.SubscriptionRepository) connect.Interceptor {
	return &authInterceptor{pool: pool, identity: identityRepository, subscriptions: subscriptions}
}

// NewAuthInterceptorWithImpersonation is the same interceptor with platform
// impersonation enabled. Kept as a separate constructor so that a server built
// without it cannot accept an impersonation header at all — the feature is off
// unless somebody wired it on purpose.
func NewAuthInterceptorWithImpersonation(pool *pgxpool.Pool, identityRepository *repository.IdentityRepository, subscriptions *repository.SubscriptionRepository, impersonation *repository.ImpersonationRepository, personalDataReads *repository.PersonalDataReadRepository) connect.Interceptor {
	return &authInterceptor{pool: pool, identity: identityRepository, subscriptions: subscriptions,
		impersonation: impersonation, personalDataReads: personalDataReads}
}

type authInterceptor struct {
	pool              *pgxpool.Pool
	identity          *repository.IdentityRepository
	subscriptions     *repository.SubscriptionRepository
	impersonation     *repository.ImpersonationRepository
	personalDataReads *repository.PersonalDataReadRepository
}

func (a *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := a.authenticate(ctx, request.Spec().Procedure, request.Header(), request.Peer().Addr)
		if err != nil {
			return nil, err
		}
		return next(ctx, request)
	}
}

// WrapStreamingClient is a passthrough — this server never originates
// outbound streaming Connect calls of its own, only handles inbound ones.
func (a *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (a *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := a.authenticate(ctx, conn.Spec().Procedure, conn.RequestHeader(), conn.Peer().Addr)
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

// authenticate holds the exact rule set previously inlined in the unary-only
// interceptor — same publicProcedures/sessionOnlyProcedures/
// restrictedMemberProcedures checks, same three context values on success.
func (a *authInterceptor) authenticate(ctx context.Context, procedure string, header http.Header, peerAddr string) (context.Context, error) {
	// An impersonation header on a procedure impersonation may not reach is
	// refused before anything else is considered.
	//
	// The check lives here rather than only inside impersonate() because not
	// every procedure reaches that code: PlatformService is session-only and
	// returns earlier, so the header would simply be ignored there and the call
	// would run as the admin's own identity. Ignoring it is not harmful — the
	// caller is the same person either way — but "ignored" and "refused" are
	// different promises, and the one worth making is the second.
	if strings.TrimSpace(header.Get(ImpersonationHeader)) != "" && !ImpersonationAllows(procedure) {
		return nil, connect.NewError(connect.CodePermissionDenied,
			errors.New("sesi impersonasi hanya boleh membaca; lakukan perubahan lewat panel platform"))
	}
	if publicProcedures[procedure] {
		return ctx, nil
	}
	token, err := bearerToken(header.Get("Authorization"))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if sessionOnlyProcedures[procedure] {
		var userID, userEmail string
		const sessionQuery = `
			SELECT s."userId", u.email
			FROM session s
			JOIN "user" u ON u.id = s."userId"
			WHERE s.token = $1 AND s."expiresAt" > NOW()`
		err = a.pool.QueryRow(ctx, sessionQuery, token).Scan(&userID, &userEmail)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired Better Auth session"))
		}
		if err != nil {
			slog.Error("validate Better Auth session", "procedure", procedure, "error", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("validate Better Auth session"))
		}
		ctx = context.WithValue(ctx, ctxKeyUserID, userID)
		ctx = context.WithValue(ctx, ctxKeyUserEmail, userEmail)
		return ctx, nil
	}
	userID, organizationID, userName, orgRole, sessionID, err := resolveStaffSessionWithRole(ctx, a.pool, token)
	if err != nil {
		return nil, err
	}
	callerIP := clientIP(header, peerAddr)

	// IP allowlist. Checked against the operator this session belongs to,
	// skipped entirely for securitySettingsProcedures (see its own comment)
	// and for a request with no configured allowlist at all — the common
	// case, and the only one that must cost nothing.
	if !securitySettingsProcedures[procedure] {
		var enabled bool
		var cidrs []string
		const allowlistQuery = `
			SELECT s.ip_allowlist_enabled, s.ip_allowlist_cidrs
			FROM operator_security_settings s
			JOIN operators o ON o.id = s.operator_id
			WHERE o.better_auth_org_id = $1`
		switch scanErr := a.pool.QueryRow(ctx, allowlistQuery, organizationID).Scan(&enabled, &cidrs); {
		case errors.Is(scanErr, pgx.ErrNoRows):
			// No row: this operator never configured an allowlist. Fails
			// open, deliberately — the feature is opt-in and must change
			// nothing for the operators who never touch it.
		case scanErr != nil:
			slog.Error("resolve IP allowlist", "procedure", procedure, "error", scanErr)
			return nil, connect.NewError(connect.CodeInternal, errors.New("resolve IP allowlist"))
		case enabled && !ipAllowed(callerIP, cidrs):
			return nil, connect.NewError(connect.CodePermissionDenied,
				errors.New("alamat IP ini tidak ada dalam daftar yang diizinkan operator"))
		}
	}

	// Impersonation replaces the tenant this request belongs to, before any of
	// the checks below run against it. Placed here rather than at the top so an
	// impersonation header is worthless without a real session of its own: the
	// admin is always identifiable, whatever screen they are looking at.
	if impersonated, active, impErr := a.impersonate(ctx, procedure, header, userID); impErr != nil {
		return nil, impErr
	} else if active {
		impersonated = context.WithValue(impersonated, ctxKeyUserID, userID)
		impersonated = context.WithValue(impersonated, ctxKeyUserName, userName)
		// Deliberately not a staff actor: an impersonated request must never be
		// able to author a row that names the customer's own staff as the one
		// who did it.
		//
		// Returning here also skips the subscription gate, and that is the
		// intended behaviour: a locked account is exactly the one support needs
		// to look at. It is safe only because the session cannot write — the
		// gate exists to stop an unpaid tenant using the product, and reading
		// their own screen on our behalf is not that.
		return impersonated, nil
	}

	if orgRole != "owner" && orgRole != "admin" {
		access, err := a.identity.GetMyAccess(ctx, userID)
		if err != nil {
			slog.Error("resolve access", "procedure", procedure, "error", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("resolve access"))
		}
		restricted := len(access.LeaderGroups) > 0 || (access.LinkedAgent != nil && access.LinkedAgent.IsActive)
		if restricted && !restrictedMemberProcedures[procedure] {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("this account's role does not permit this action"))
		}
	}

	// Subscription gate. Placed here deliberately: only an operator staff
	// session reaches this far, because the pilgrim, leader and agent surfaces
	// return earlier via publicProcedures or sessionOnlyProcedures. A lapsed
	// subscription therefore locks the dashboard while leaving the storefront
	// and every portal a jamaah depends on untouched.
	//
	// Fails closed — a procedure added later is gated unless it is deliberately
	// listed as reachable while locked. Billing must stay open, or an operator
	// is trapped with no way to pay their way back in.
	if a.subscriptions != nil && !billingProcedures[procedure] {
		access, accessErr := a.subscriptions.GetAccessByOrgID(ctx, organizationID)
		switch {
		case errors.Is(accessErr, apperror.ErrNotFound):
			// No subscription row yet: an operator created before billing
			// existed, or one still mid-signup. Allowing it is the safe
			// direction; the alternative locks a paying customer out over a
			// missing row.
		case accessErr != nil:
			slog.Error("resolve subscription", "procedure", procedure, "error", accessErr)
			return nil, connect.NewError(connect.CodeInternal, errors.New("resolve subscription"))
		case !access.Allowed:
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("langganan tidak aktif; selesaikan pembayaran untuk membuka kembali dashboard"))
		}
	}
	ctx = context.WithValue(ctx, ctxKeyUserID, userID)
	ctx = context.WithValue(ctx, ctxKeyOperatorID, organizationID)
	ctx = context.WithValue(ctx, ctxKeyUserName, userName)
	ctx = context.WithValue(ctx, ctxKeyOrgRole, orgRole)
	ctx = context.WithValue(ctx, ctxKeySessionID, sessionID)
	ctx = context.WithValue(ctx, ctxKeyClientIP, callerIP)
	ctx = repository.ContextWithStaffActor(ctx, userID)
	return ctx, nil
}

// ipAllowed reports whether ip falls inside at least one of cidrs. A CIDR
// that fails to parse is skipped rather than treated as a match — a typo in
// one range must narrow the allowlist, never silently widen it to "anything".
func ipAllowed(ip string, cidrs []string) bool {
	parsed, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue
		}
		if prefix.Contains(parsed) {
			return true
		}
	}
	return false
}

// ResolveStaffSession validates a Better Auth bearer token the same way
// NewAuthInterceptor does for every authenticated Connect RPC, and returns
// the same three values the interceptor puts on ctx. Exported so plain
// net/http handlers outside Connect (e.g. the multipart upload endpoint in
// main.go) can authenticate with the identical rule instead of
// reimplementing this query.
func ResolveStaffSession(ctx context.Context, pool *pgxpool.Pool, token string) (userID, organizationID, userName string, err error) {
	userID, organizationID, userName, _, _, err = resolveStaffSessionWithRole(ctx, pool, token)
	return userID, organizationID, userName, err
}

// ResolveStaffSessionRole is the authorization-oriented counterpart used by
// plain HTTP handlers that need the caller's organization role. Keeping this
// separate prevents a user's display name from ever being mistaken for an
// authorization value.
func ResolveStaffSessionRole(ctx context.Context, pool *pgxpool.Pool, token string) (userID, organizationID, orgRole string, err error) {
	userID, organizationID, _, orgRole, _, err = resolveStaffSessionWithRole(ctx, pool, token)
	return userID, organizationID, orgRole, err
}

func resolveStaffSessionWithRole(ctx context.Context, pool *pgxpool.Pool, token string) (userID, organizationID, userName, orgRole, sessionID string, err error) {
	const query = `
		SELECT s.id,
		       s."userId",
		       COALESCE(s."activeOrganizationId", m."organizationId") AS "orgId",
		       u.name,
		       m.role
		FROM session s
		JOIN member m ON m."userId" = s."userId"
		JOIN "user" u ON u.id = s."userId"
		WHERE s.token = $1
		  AND s."expiresAt" > NOW()
		  AND (
		    s."activeOrganizationId" IS NULL
		    OR s."activeOrganizationId" = m."organizationId"
		  )
		ORDER BY
		  CASE WHEN s."activeOrganizationId" = m."organizationId" THEN 0 ELSE 1 END
		LIMIT 1`
	err = pool.QueryRow(ctx, query, token).Scan(&sessionID, &userID, &organizationID, &userName, &orgRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", "", "", connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired Better Auth session"))
	}
	if err != nil {
		return "", "", "", "", "", connect.NewError(connect.CodeInternal, errors.New("validate Better Auth session"))
	}
	if userID == "" || organizationID == "" {
		return "", "", "", "", "", connect.NewError(connect.CodeUnauthenticated, errors.New("Better Auth session has no active organization"))
	}
	return userID, organizationID, userName, orgRole, sessionID, nil
}

func UserIDFromCtx(ctx context.Context) string {
	userID, _ := ctx.Value(ctxKeyUserID).(string)
	return userID
}

func OperatorIDFromCtx(ctx context.Context) string {
	operatorID, _ := ctx.Value(ctxKeyOperatorID).(string)
	return operatorID
}

func UserNameFromCtx(ctx context.Context) string {
	userName, _ := ctx.Value(ctxKeyUserName).(string)
	return userName
}

func UserEmailFromCtx(ctx context.Context) string {
	userEmail, _ := ctx.Value(ctxKeyUserEmail).(string)
	return userEmail
}

func SessionIDFromCtx(ctx context.Context) string {
	sessionID, _ := ctx.Value(ctxKeySessionID).(string)
	return sessionID
}

func ClientIPFromCtx(ctx context.Context) string {
	ip, _ := ctx.Value(ctxKeyClientIP).(string)
	return ip
}

func OrgRoleFromCtx(ctx context.Context) string {
	orgRole, _ := ctx.Value(ctxKeyOrgRole).(string)
	return orgRole
}

// ContextWithIdentity attaches authenticated values for in-process callers and tests.
// Production requests receive the same values from NewAuthInterceptor.
func ContextWithIdentity(ctx context.Context, userID, operatorID string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID, userID)
	ctx = context.WithValue(ctx, ctxKeyOperatorID, operatorID)
	return repository.ContextWithStaffActor(ctx, userID)
}

// ContextWithStaffIdentity is the test/in-process counterpart for operations
// whose service layer also enforces an owner/admin role. Production requests
// receive these exact values from authenticate.
func ContextWithStaffIdentity(ctx context.Context, userID, operatorID, role string) context.Context {
	ctx = ContextWithIdentity(ctx, userID, operatorID)
	return context.WithValue(ctx, ctxKeyOrgRole, role)
}

func bearerToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("missing bearer token")
	}
	return parts[1], nil
}
