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
)

// publicProcedures lists RPCs that must be reachable without a Better Auth session —
// e.g. a prospective agent applying via a public link. Keep this list minimal and
// treat every addition as a security decision: identity fields these handlers need
// (like operator_id) must come from the request body instead of ctx, and the service
// layer must validate them explicitly since there's no session to trust.
var publicProcedures = map[string]bool{
	"/hajj.v1.AgentService/ApplyAsAgent":          true,
	"/hajj.v1.PilgrimAppService/GetMyInfo":        true,
	"/hajj.v1.PilgrimAppService/ListMySchedule":   true,
	"/hajj.v1.PilgrimAppService/ListMyProducts":   true,
	"/hajj.v1.PilgrimAppService/UpdateMyLocation": true,
	"/hajj.v1.SOSService/CreateSOSAlert":          true,
	"/hajj.v1.ChatService/ListMyMessages":         true,
	"/hajj.v1.ChatService/SendMyMessage":          true,
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
			var userID, organizationID, userName string
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
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired Better Auth session"))
			}
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.New("validate Better Auth session"))
			}
			if userID == "" || organizationID == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("Better Auth session has no active organization"))
			}
			ctx = context.WithValue(ctx, ctxKeyUserID, userID)
			ctx = context.WithValue(ctx, ctxKeyOperatorID, organizationID)
			ctx = context.WithValue(ctx, ctxKeyUserName, userName)
			return next(ctx, request)
		}
	})
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
