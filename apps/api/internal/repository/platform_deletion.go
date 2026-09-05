package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
)

// TenantDeletionGraceDays is how long a tenant's data survives after their
// access actually ends before deletion becomes possible at all — D6
// (TUGAS-PANEL-SAAS.md, §7.3 DESAIN): "penghapusan menunggu 90 hari sejak
// akses berakhir." A pure constant, not a setting: the day this needs to be
// configurable is a policy decision for the owner, not a default to expose.
const TenantDeletionGraceDays = 90

// DeletionChange is the request to permanently delete one tenant's data.
type DeletionChange struct {
	OperatorID     string
	ActorUserID    string
	Reason         string
	Confirmation   string
	IdempotencyKey string
	AdminCount     int32
}

// DeletionResult is what is worth showing after a tenant no longer exists —
// mostly a receipt, since there is no tenant left to look up afterward.
type DeletionResult struct {
	OperatorID   string
	OperatorName string
	DeletedAt    time.Time
}

// DeleteTenant permanently removes a tenant and everything that belongs to
// it, except audit_logs — see migration 165 for why that survives.
//
// Everything in this schema that is a tenant's own data already cascades
// from operators(id) ON DELETE CASCADE (pilgrims, seasons, orders, products,
// CRM leads, support tickets — all of it). That is what makes a plain
// DELETE FROM operators a complete deletion with nothing left orphaned: the
// alternative, hand-picking which tables count as "personal data" and
// deleting each one, is exactly the kind of thing that quietly misses a
// table and leaves data behind that UU PDP says should be gone.
//
// Three things must all be true before this runs, checked inside the same
// advisory-locked transaction as Suspend/Reinstate so a concurrent request
// cannot race past any of them:
//  1. Access must have been over for TenantDeletionGraceDays already —
//     access_until plus grace period, not "cancelled," because a lapsed
//     trial that was never explicitly cancelled must count too.
//  2. A READY data export must exist — the portability right actually
//     honoured, not merely offered in a UI that nobody enforces.
//  3. Four-eyes, the same shape as Suspend/Reinstate: privileged_actions,
//     confirmation typed as the tenant's own name, admin_count_at_request
//     recorded honestly rather than assumed.
func (r *PlatformRepository) DeleteTenant(ctx context.Context, change DeletionChange, hasReadyExport bool) (DeletionResult, error) {
	result := DeletionResult{}
	operator, err := pgUUID(change.OperatorID)
	if err != nil {
		return result, apperror.ErrValidation
	}
	if len(strings.TrimSpace(change.Reason)) < 10 ||
		strings.TrimSpace(change.IdempotencyKey) == "" ||
		strings.TrimSpace(change.ActorUserID) == "" {
		return result, apperror.ErrValidation
	}
	if change.AdminCount < 1 {
		change.AdminCount = 1
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return result, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"platform-deletion:"+change.OperatorID); err != nil {
		return result, databaseError(err)
	}

	var existing string
	err = tx.QueryRow(ctx, `
		SELECT payload->>'operator_id' FROM privileged_actions
		WHERE requested_by = $1 AND kind = 'DELETE_TENANT' AND idempotency_key = $2`,
		change.ActorUserID, change.IdempotencyKey).Scan(&existing)
	switch {
	case err == nil:
		if existing != change.OperatorID {
			return result, apperror.ErrConflict
		}
		// The tenant is already gone by definition of this being a replay —
		// nothing left to read back except what the ledger itself recorded.
		var name string
		var executedAt *time.Time
		if err := tx.QueryRow(ctx, `
			SELECT payload->>'operator_name', executed_at FROM privileged_actions
			WHERE requested_by = $1 AND kind = 'DELETE_TENANT' AND idempotency_key = $2`,
			change.ActorUserID, change.IdempotencyKey).Scan(&name, &executedAt); err != nil {
			return result, databaseError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return result, databaseError(err)
		}
		result.OperatorID, result.OperatorName = change.OperatorID, name
		if executedAt != nil {
			result.DeletedAt = *executedAt
		}
		return result, nil
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return result, databaseError(err)
	}

	var name string
	var effectiveAccessUntil *time.Time
	var pilgrimCount, seasonCount int
	err = tx.QueryRow(ctx, `
		SELECT o.name,
		       CASE WHEN s.operator_id IS NULL THEN NULL
		            ELSE subscription_effective_access_until(s.access_until, s.grace_period_days) END,
		       (SELECT COUNT(*) FROM pilgrims WHERE operator_id = o.id),
		       (SELECT COUNT(*) FROM seasons WHERE operator_id = o.id)
		FROM operators o
		LEFT JOIN subscriptions s ON s.operator_id = o.id
		WHERE o.id = $1 FOR UPDATE OF o`, operator).
		Scan(&name, &effectiveAccessUntil, &pilgrimCount, &seasonCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrTenantNotFound
	}
	if err != nil {
		return result, databaseError(err)
	}
	if !strings.EqualFold(strings.TrimSpace(change.Confirmation), strings.TrimSpace(name)) {
		return result, apperror.ErrValidation
	}

	if effectiveAccessUntil == nil {
		return result, fmt.Errorf("%w: tenant belum pernah punya langganan, tidak ada akses yang berakhir untuk dihitung", apperror.ErrFailedPrecondition)
	}
	eligibleAt := effectiveAccessUntil.AddDate(0, 0, TenantDeletionGraceDays)
	if time.Now().Before(eligibleAt) {
		return result, fmt.Errorf("%w: baru bisa dihapus mulai %s (90 hari sejak akses berakhir)",
			apperror.ErrFailedPrecondition, eligibleAt.Format("2 January 2006"))
	}
	if !hasReadyExport {
		return result, fmt.Errorf("%w: ekspor data tenant ini belum pernah dibuat — minta ekspornya lebih dulu",
			apperror.ErrFailedPrecondition)
	}

	// Written while operator_id is still a live reference — after the
	// DELETE below, migration 165 turns it into NULL, and this row is the
	// last place the connection between this event and that tenant's name
	// is spelled out in one readable sentence rather than reconstructed
	// from a payload.
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (operator_id,user_id,action,entity_type,entity_id,metadata)
		VALUES ($1,$2,'tenant_deleted','operator',$3,$4)`,
		operator, change.ActorUserID, change.OperatorID, map[string]any{
			"message": strings.TrimSpace(change.Reason), "operator_name": name,
			"idempotency_key": change.IdempotencyKey, "admin_count_at_request": change.AdminCount,
			"pilgrim_count": pilgrimCount, "season_count": seasonCount,
		}); err != nil {
		return result, databaseError(err)
	}

	payload := map[string]any{
		"operator_id": change.OperatorID, "operator_name": name,
		"confirmation": strings.TrimSpace(change.Confirmation),
		"admin_count_at_request": change.AdminCount,
		"pilgrim_count": pilgrimCount, "season_count": seasonCount,
	}
	payloadJSON, _ := json.Marshal(payload)
	if _, err := tx.Exec(ctx, `
		INSERT INTO privileged_actions (kind, payload, reason, requested_by, approved_by, executed_at, idempotency_key)
		VALUES ('DELETE_TENANT',$1,$2,$3,$3,NOW(),$4)`,
		payloadJSON, strings.TrimSpace(change.Reason), change.ActorUserID, change.IdempotencyKey); err != nil {
		if IsUniqueViolation(err, "privileged_actions_requested_by_kind_idempotency_key_key") {
			return result, apperror.ErrConflict
		}
		return result, databaseError(err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM operators WHERE id = $1`, operator); err != nil {
		return result, databaseError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return result, databaseError(err)
	}
	result.OperatorID, result.OperatorName, result.DeletedAt = change.OperatorID, name, time.Now()
	return result, nil
}
