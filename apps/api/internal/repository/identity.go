package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
)

// accessCacheTTL is deliberately short — long enough that a page guard
// calling GetMyAccess on every dashboard/leader layout mount doesn't hammer
// the DB with 3 queries per navigation, short enough that a role change
// (e.g. an admin removing someone's group leadership, or a pilgrim linking
// their Google account) takes effect for that user within one refresh
// cycle rather than requiring a sign-out.
const accessCacheTTL = 20 * time.Second

type accessCacheEntry struct {
	value     *domain.MyAccess
	expiresAt time.Time
}

type IdentityRepository struct {
	queries         *db.Queries
	agentRepository *AgentRepository

	mu    sync.Mutex
	cache map[string]accessCacheEntry // userID -> cached MyAccess
}

func NewIdentityRepository(queries *db.Queries, agents *AgentRepository) *IdentityRepository {
	return &IdentityRepository{queries: queries, agentRepository: agents, cache: make(map[string]accessCacheEntry)}
}

// GetMyAccess resolves every role system this user id participates in —
// organization staff, group leadership, and a linked pilgrim record — so
// the frontend's single shared login can land the caller on the right
// surface, and every protected page can verify the caller actually belongs
// there instead of trusting session-cookie presence alone. In-memory,
// per-process cache — fine for the single-API-instance deployment (same
// tradeoff as the rate limiter in middleware/ratelimit.go); move to Redis
// if the API ever runs more than one replica.
func (r *IdentityRepository) GetMyAccess(ctx context.Context, userID string) (*domain.MyAccess, error) {
	if cached, ok := r.cachedAccess(userID); ok {
		return cached, nil
	}
	access, err := r.fetchMyAccess(ctx, userID)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[userID] = accessCacheEntry{value: access, expiresAt: time.Now().Add(accessCacheTTL)}
	r.mu.Unlock()
	return access, nil
}

// InvalidateAccessCache drops any cached MyAccess for a user id immediately
// — called right after an action that changes what this call would return
// (e.g. LinkGoogleAccount) so the next check reflects it without waiting
// out the TTL.
func (r *IdentityRepository) InvalidateAccessCache(userID string) {
	r.mu.Lock()
	delete(r.cache, userID)
	r.mu.Unlock()
}

func (r *IdentityRepository) cachedAccess(userID string) (*domain.MyAccess, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.cache[userID]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

// fetchMyAccess is the real, uncached lookup — each lookup is independent
// and "not found" is expected for most identities in most role systems (a
// leader is rarely also staff), so a missing row from any one of them is
// not an error for the whole call.
func (r *IdentityRepository) fetchMyAccess(ctx context.Context, userID string) (*domain.MyAccess, error) {
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

	// Tour Leader (Agent) — only possible for an org member, since the
	// lookup is scoped to this identity's own operator. Non-fatal if not
	// found (pgx.ErrNoRows) — most org members are not also an agent.
	if result.IsOrgMember {
		agent, err := r.agentRepository.GetByLinkedUser(ctx, result.OperatorID, userID)
		switch {
		case err == nil && agent.IsActive:
			result.LinkedAgent = &domain.Agent{ID: agent.ID, Name: agent.Name, ReferralCode: agent.ReferralCode, IsActive: agent.IsActive}
		case err == nil, errors.Is(err, pgx.ErrNoRows):
			// inactive agent, or not an agent at all — expected
		default:
			return nil, err
		}
	}

	return result, nil
}
