package repository

import (
	"context"
	"errors"

	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type staffActorContextKey struct{}

// ContextWithStaffActor marks a request as an authenticated operator-staff
// request. Middleware owns the identity check; repositories own the branch
// lookup and every data filter that follows from it.
func ContextWithStaffActor(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, staffActorContextKey{}, userID)
}

func staffActorFromContext(ctx context.Context) string {
	userID, _ := ctx.Value(staffActorContextKey{}).(string)
	return userID
}

// branchScope returns a valid UUID only when the authenticated staff actor is
// assigned to a branch of this operator. A missing actor represents an
// explicitly non-staff/system path; a staff member without branch_members is
// head-office staff and remains operator-wide.
func branchScope(ctx context.Context, queries *db.Queries, operatorID pgtype.UUID) (pgtype.UUID, error) {
	userID := staffActorFromContext(ctx)
	if userID == "" {
		return pgtype.UUID{}, nil
	}
	branchID, err := queries.GetStaffBranchScope(ctx, db.GetStaffBranchScopeParams{
		BetterAuthUserID: userID,
		OperatorID:       operatorID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, nil
	}
	if err != nil {
		return pgtype.UUID{}, databaseError(err)
	}
	return branchID, nil
}
