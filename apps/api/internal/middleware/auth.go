package middleware

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const (
	ctxKeyUserID     contextKey = "user_id"
	ctxKeyOperatorID contextKey = "operator_id"
	ctxKeyUserName   contextKey = "user_name"
	ctxKeyUserEmail  contextKey = "user_email"
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
	"/hajj.v1.FamilyTrackerService/GetFamilyStatus": true,
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
}

// NewAuthInterceptor validates Better Auth's opaque database session token.
// Better Auth does not issue JWTs for its default session strategy.
func NewAuthInterceptor(pool *pgxpool.Pool) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			if publicProcedures[request.Spec().Procedure] {
				return next(ctx, request)
			}
			token, err := bearerToken(request.Header().Get("Authorization"))
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			if sessionOnlyProcedures[request.Spec().Procedure] {
				var userID, userEmail string
				const sessionQuery = `
					SELECT s."userId", u.email
					FROM session s
					JOIN "user" u ON u.id = s."userId"
					WHERE s.token = $1 AND s."expiresAt" > NOW()`
				err = pool.QueryRow(ctx, sessionQuery, token).Scan(&userID, &userEmail)
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired Better Auth session"))
				}
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, errors.New("validate Better Auth session"))
				}
				ctx = context.WithValue(ctx, ctxKeyUserID, userID)
				ctx = context.WithValue(ctx, ctxKeyUserEmail, userEmail)
				return next(ctx, request)
			}
			userID, organizationID, userName, err := ResolveStaffSession(ctx, pool, token)
			if err != nil {
				return nil, err
			}
			ctx = context.WithValue(ctx, ctxKeyUserID, userID)
			ctx = context.WithValue(ctx, ctxKeyOperatorID, organizationID)
			ctx = context.WithValue(ctx, ctxKeyUserName, userName)
			return next(ctx, request)
		}
	})
}

// ResolveStaffSession validates a Better Auth bearer token the same way
// NewAuthInterceptor does for every authenticated Connect RPC, and returns
// the same three values the interceptor puts on ctx. Exported so plain
// net/http handlers outside Connect (e.g. the multipart upload endpoint in
// main.go) can authenticate with the identical rule instead of
// reimplementing this query.
func ResolveStaffSession(ctx context.Context, pool *pgxpool.Pool, token string) (userID, organizationID, userName string, err error) {
	const query = `
		SELECT s."userId",
		       COALESCE(s."activeOrganizationId", m."organizationId") AS "orgId",
		       u.name
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
	err = pool.QueryRow(ctx, query, token).Scan(&userID, &organizationID, &userName)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired Better Auth session"))
	}
	if err != nil {
		return "", "", "", connect.NewError(connect.CodeInternal, errors.New("validate Better Auth session"))
	}
	if userID == "" || organizationID == "" {
		return "", "", "", connect.NewError(connect.CodeUnauthenticated, errors.New("Better Auth session has no active organization"))
	}
	return userID, organizationID, userName, nil
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

// ContextWithIdentity attaches authenticated values for in-process callers and tests.
// Production requests receive the same values from NewAuthInterceptor.
func ContextWithIdentity(ctx context.Context, userID, operatorID string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID, userID)
	return context.WithValue(ctx, ctxKeyOperatorID, operatorID)
}

func bearerToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("missing bearer token")
	}
	return parts[1], nil
}
