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
)

// NewAuthInterceptor validates Better Auth's opaque database session token.
// Better Auth does not issue JWTs for its default session strategy.
func NewAuthInterceptor(pool *pgxpool.Pool) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			token, err := bearerToken(request.Header().Get("Authorization"))
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			var userID, organizationID string
			const query = `
				SELECT s."userId",
				       COALESCE(s."activeOrganizationId", m."organizationId") AS "orgId"
				FROM session s
				JOIN member m ON m."userId" = s."userId"
				WHERE s.token = $1
				  AND s."expiresAt" > NOW()
				  AND (
				    s."activeOrganizationId" IS NULL
				    OR s."activeOrganizationId" = m."organizationId"
				  )
				ORDER BY
				  CASE WHEN s."activeOrganizationId" = m."organizationId" THEN 0 ELSE 1 END
				LIMIT 1`
			err = pool.QueryRow(ctx, query, token).Scan(&userID, &organizationID)
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
