package repository

import (
	"context"
	"errors"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
)

type IdentityRepository struct{ queries *db.Queries }

func NewIdentityRepository(queries *db.Queries) *IdentityRepository {
	return &IdentityRepository{queries: queries}
}

// GetMyAccess resolves every role system this user id participates in —
// organization staff, group leadership, and a linked pilgrim record — so
// the frontend's single shared login can land the caller on the right
// surface. Each lookup is independent and "not found" is expected for most
// identities in most role systems (a leader is rarely also staff), so a
// missing row from any one of them is not an error for the whole call.
func (r *IdentityRepository) GetMyAccess(ctx context.Context, userID string) (*domain.MyAccess, error) {
	result := &domain.MyAccess{}

	membership, err := r.queries.GetOrgMembershipForUser(ctx, userID)
	switch {
	case err == nil:
		result.IsOrgMember = true
		result.OrgRole = membership.Role
		result.OperatorID = uuidString(membership.OperatorID)
		result.OperatorName = membership.OperatorName
	case errors.Is(err, pgx.ErrNoRows):
		// not staff — expected for leaders/pilgrims
	default:
		return nil, err
	}

	groups, err := r.queries.ListLeaderGroupsForUser(ctx, pgText(userID))
	if err != nil {
		return nil, err
	}
	result.LeaderGroups = make([]domain.LeaderGroupSummary, 0, len(groups))
	for _, g := range groups {
		result.LeaderGroups = append(result.LeaderGroups, domain.LeaderGroupSummary{ID: uuidString(g.ID), Name: g.Name})
	}

	pilgrim, err := r.queries.GetLinkedPilgrimForUser(ctx, pgText(userID))
	switch {
	case err == nil:
		result.LinkedPilgrim = &domain.PilgrimSummary{ID: uuidString(pilgrim.ID), AppAccessCode: pilgrim.AppAccessCode, FullName: pilgrim.FullName}
	case errors.Is(err, pgx.ErrNoRows):
		// not linked to a pilgrim — expected for staff/leaders
	default:
		return nil, err
	}

	return result, nil
}
