package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"
)

// ImpersonationHeader carries the token issued by StartImpersonation. It is
// deliberately separate from Authorization: the platform admin's own session
// stays in place underneath, so the request is always attributable to a real
// person as well as to the tenant being viewed.
const ImpersonationHeader = "X-Impersonation-Token"

const (
	ctxKeyImpersonating   contextKey = "impersonating"
	ctxKeyImpersonatedBy  contextKey = "impersonated_by"
	ctxKeyImpersonationID contextKey = "impersonation_id"
)

// readOnlyPrefixes is the whole of what an impersonated session may call.
//
// Default deny, by prefix rather than by a list of procedures. A list would be
// silently wrong the day somebody adds an RPC and forgets it — and the failure
// direction of a forgotten entry matters: with a denylist, a new write RPC is
// reachable; with this, it is refused until somebody decides otherwise.
//
// The prefixes match the naming this codebase already keeps to (verified by
// TestImpersonationAllowsOnlyReads, which walks every registered procedure).
// CheckOperatorSlug and similar reads are absent on purpose: being denied
// costs a support person nothing, being wrongly allowed costs a customer.
var readOnlyPrefixes = []string{"List", "Get", "Preview", "Count", "Am"}

// servicesClosedToImpersonation are surfaces that must never be reachable
// while wearing a customer's face, whatever their method is called.
// PlatformService is the platform's own controls: reading it as a tenant would
// hand the tenant's screen every other tenant's data, and the admin already
// has it through their own session.
var servicesClosedToImpersonation = []string{
	"/hajj.v1.PlatformService/",
	"/hajj.v1.FunnelService/",
}

// ImpersonationAllows reports whether an impersonated session may call this
// procedure. Exported so the test that walks the generated service descriptors
// can check every one of them.
func ImpersonationAllows(procedure string) bool {
	for _, closed := range servicesClosedToImpersonation {
		if strings.HasPrefix(procedure, closed) {
			return false
		}
	}
	slash := strings.LastIndex(procedure, "/")
	if slash < 0 || slash+1 >= len(procedure) {
		return false
	}
	method := procedure[slash+1:]
	for _, prefix := range readOnlyPrefixes {
		if !strings.HasPrefix(method, prefix) {
			continue
		}
		// "Get" must not also admit "GetOrCreateX". The character after the
		// prefix has to start a new word that is not another verb glued on —
		// checked properly by the descriptor test; this is the cheap guard.
		rest := method[len(prefix):]
		if rest == "" || (rest[0] >= 'A' && rest[0] <= 'Z') {
			return !strings.Contains(method, "OrCreate")
		}
	}
	return false
}

// impersonate resolves the header, if present, into a tenant identity.
//
// Three things must all hold: the caller holds platform access with a second
// factor, the token is live, and it belongs to that same caller. The last is
// what stops a token leaking from one admin's browser and being replayed by
// somebody else who merely has access of their own.
func (a *authInterceptor) impersonate(ctx context.Context, procedure string, header http.Header, adminUserID string) (context.Context, bool, error) {
	token := strings.TrimSpace(header.Get(ImpersonationHeader))
	if token == "" {
		return ctx, false, nil
	}
	if a.impersonation == nil {
		return nil, false, connect.NewError(connect.CodePermissionDenied, errors.New("impersonasi tidak aktif di server ini"))
	}

	var granted, twoFactor bool
	if err := a.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM platform_admins WHERE user_id = $1),
		       COALESCE((SELECT "twoFactorEnabled" FROM "user" WHERE id = $1), false)`,
		adminUserID).Scan(&granted, &twoFactor); err != nil {
		slog.Error("resolve platform access for impersonation", "procedure", procedure, "error", err)
		return nil, false, connect.NewError(connect.CodeInternal, errors.New("resolve platform access"))
	}
	if !granted || !twoFactor {
		return nil, false, connect.NewError(connect.CodePermissionDenied, errors.New("akses admin platform diperlukan"))
	}

	session, err := a.impersonation.Resolve(ctx, token)
	if err != nil {
		// Expired, ended and never-existed all answer the same, so a token
		// cannot be probed for which of the three it is.
		return nil, false, connect.NewError(connect.CodeUnauthenticated, errors.New("sesi impersonasi tidak berlaku"))
	}
	if session.AdminUserID != adminUserID {
		return nil, false, connect.NewError(connect.CodePermissionDenied, errors.New("sesi impersonasi milik admin lain"))
	}
	if !ImpersonationAllows(procedure) {
		// The refusal names itself: somebody looking at a customer's screen who
		// clicks Save should be told why nothing happened, not left guessing.
		return nil, false, connect.NewError(connect.CodePermissionDenied,
			errors.New("sesi impersonasi hanya boleh membaca; lakukan perubahan lewat panel platform"))
	}

	// Recorded before the handler runs, so a read that is served is a read that
	// is written down. Doing it afterwards would mean a crash mid-response
	// leaves the data seen and the record missing.
	//
	// The consequence, stated rather than hidden: a request that the handler
	// then rejects still counts. The ledger therefore counts attempts, not
	// rows returned — which is the more useful of the two for a privacy review,
	// and the screen says so.
	//
	// Only impersonated reads are recorded here. A travel agency reading its own
	// jamaah is not a privacy event and would bury the ones that are.
	a.recordPersonalDataRead(ctx, procedure, adminUserID, session.ID, session.OperatorID)

	ctx = context.WithValue(ctx, ctxKeyOperatorID, session.BetterAuthOrgID)
	ctx = context.WithValue(ctx, ctxKeyImpersonating, true)
	ctx = context.WithValue(ctx, ctxKeyImpersonatedBy, adminUserID)
	ctx = context.WithValue(ctx, ctxKeyImpersonationID, session.ID)
	return ctx, true, nil
}

// IsImpersonating reports whether this request is being made through an
// impersonation session. Anything that writes, sends, or charges should refuse
// when it is true — the interceptor already refuses first, and this is the
// second line for code that runs outside it.
func IsImpersonating(ctx context.Context) bool {
	value, _ := ctx.Value(ctxKeyImpersonating).(bool)
	return value
}

// ImpersonatedByFromCtx is the real person behind the request.
func ImpersonatedByFromCtx(ctx context.Context) string {
	value, _ := ctx.Value(ctxKeyImpersonatedBy).(string)
	return value
}

func ImpersonationIDFromCtx(ctx context.Context) string {
	value, _ := ctx.Value(ctxKeyImpersonationID).(string)
	return value
}
