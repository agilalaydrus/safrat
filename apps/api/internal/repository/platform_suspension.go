package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
)

// SuspensionChange is a deliberate lock-out, or the lifting of one.
type SuspensionChange struct {
	OperatorID     string
	ActorUserID    string
	Reason         string
	Confirmation   string
	IdempotencyKey string
	AdminCount     int32
}

type SuspensionResult struct {
	OperatorID   string
	OperatorName string
	SuspendedAt  *time.Time
	// Untouched by either operation, and returned so the screen can say what
	// the tenant gets back when the suspension is lifted.
	AccessUntil *time.Time
}

// Suspend locks a tenant out on purpose.
//
// It sets suspended_at and nothing else. access_until is deliberately left
// alone: access is decided by `suspended_at IS NULL AND effective_access_until
// > NOW()`, so the flag closes the door immediately while the time they have
// already paid for keeps running underneath. Lifting the suspension therefore
// returns exactly what they bought, with no arithmetic to get wrong.
//
// The confirmation must be the tenant's own name, typed by hand. It is checked
// here rather than in the service because this is where the real name is known
// — a check against a name the caller also supplied would confirm nothing.
func (r *PlatformRepository) Suspend(ctx context.Context, change SuspensionChange) (SuspensionResult, error) {
	return r.setSuspension(ctx, change, true)
}

// Reinstate lifts a suspension. Safe to call on a tenant who is locked out for
// non-payment instead: clearing the flag cannot grant access on its own,
// because their access_until is already in the past.
func (r *PlatformRepository) Reinstate(ctx context.Context, change SuspensionChange) (SuspensionResult, error) {
	return r.setSuspension(ctx, change, false)
}

func (r *PlatformRepository) setSuspension(ctx context.Context, change SuspensionChange, suspend bool) (SuspensionResult, error) {
	result := SuspensionResult{}
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

	// One tenant at a time. Two admins acting on the same tenant at once would
	// otherwise interleave the read of the current state with the write.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"platform-suspension:"+change.OperatorID); err != nil {
		return result, databaseError(err)
	}

	var name string
	var suspendedAt, accessUntil *time.Time
	var previousReason string
	err = tx.QueryRow(ctx, `
		SELECT o.name, s.suspended_at, s.access_until, COALESCE(s.suspended_reason, '')
		FROM operators o
		LEFT JOIN subscriptions s ON s.operator_id = o.id
		WHERE o.id = $1`, operator).Scan(&name, &suspendedAt, &accessUntil, &previousReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrTenantNotFound
	}
	if err != nil {
		return result, databaseError(err)
	}

	// Typed by hand, compared to the real name. Case and surrounding spaces are
	// forgiven; the words are not. Someone who cannot produce the name of the
	// agency they are about to lock out is on the wrong row.
	if !strings.EqualFold(strings.TrimSpace(change.Confirmation), strings.TrimSpace(name)) {
		return result, apperror.ErrValidation
	}

	kind := "REINSTATE"
	if suspend {
		kind = "SUSPEND"
	}

	// A retry settles the first action rather than performing a second one.
	// Checked inside the advisory lock, and backed by the unique constraint on
	// (requested_by, kind, idempotency_key) underneath — the lock makes the
	// retry deterministic, the constraint is what makes it safe.
	var existing string
	err = tx.QueryRow(ctx, `
		SELECT payload->>'operator_id' FROM privileged_actions
		WHERE requested_by = $1 AND kind = $2 AND idempotency_key = $3`,
		change.ActorUserID, kind, change.IdempotencyKey).Scan(&existing)
	switch {
	case err == nil:
		if existing != change.OperatorID {
			// The same key used for a different tenant is a bug in the caller,
			// and answering it with the first tenant's result would hide it.
			return result, apperror.ErrConflict
		}
		return SuspensionResult{OperatorID: change.OperatorID, OperatorName: name,
			SuspendedAt: suspendedAt, AccessUntil: accessUntil}, nil
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return result, databaseError(err)
	}

	if suspend {
		if _, err := tx.Exec(ctx, `
			UPDATE subscriptions
			SET suspended_at = NOW(), suspended_reason = $2, updated_at = NOW()
			WHERE operator_id = $1`, operator, strings.TrimSpace(change.Reason)); err != nil {
			return result, databaseError(err)
		}
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE subscriptions
			SET suspended_at = NULL, suspended_reason = '', updated_at = NOW()
			WHERE operator_id = $1`, operator); err != nil {
			return result, databaseError(err)
		}
	}

	payload := map[string]any{
		"operator_id": change.OperatorID, "operator_name": name,
		"confirmation": strings.TrimSpace(change.Confirmation),
		// What the tenant was locked out for before this, so lifting a
		// suspension does not erase the reason it existed.
		"previous_reason":        previousReason,
		"admin_count_at_request": change.AdminCount,
	}
	payloadJSON, _ := json.Marshal(payload)
	if _, err := tx.Exec(ctx, `
		INSERT INTO privileged_actions (kind, payload, reason, requested_by, approved_by, executed_at, idempotency_key)
		VALUES ($1,$2,$3,$4,$4,NOW(),$5)`,
		kind, payloadJSON, strings.TrimSpace(change.Reason), change.ActorUserID, change.IdempotencyKey); err != nil {
		if IsUniqueViolation(err, "privileged_actions_requested_by_kind_idempotency_key_key") {
			return result, apperror.ErrConflict
		}
		return result, databaseError(err)
	}

	action := "tenant_reinstated"
	if suspend {
		action = "tenant_suspended"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (operator_id,user_id,action,entity_type,entity_id,metadata)
		VALUES ($1,$2,$3,'operator',$4,$5)`, operator, change.ActorUserID, action, change.OperatorID,
		map[string]any{"message": strings.TrimSpace(change.Reason), "idempotency_key": change.IdempotencyKey,
			"admin_count_at_request": change.AdminCount}); err != nil {
		return result, databaseError(err)
	}

	if err := tx.QueryRow(ctx, `
		SELECT s.suspended_at, s.access_until FROM subscriptions s WHERE s.operator_id = $1`, operator).
		Scan(&result.SuspendedAt, &result.AccessUntil); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, databaseError(err)
	}
	result.OperatorID = change.OperatorID
	result.OperatorName = name
	if err := tx.Commit(ctx); err != nil {
		return result, databaseError(err)
	}
	return result, nil
}

// PrivilegedAction is one row of the ledger, for reading back.
type PrivilegedAction struct {
	ID           string
	Kind         string
	Reason       string
	RequestedBy  string
	ApprovedBy   string
	RequestedAt  time.Time
	ExecutedAt   *time.Time
	OperatorName string
	AdminCount   int32
}

// ListPrivilegedActionsForOperator is what makes the ledger worth keeping. A
// table nobody reads is a table nobody notices is empty.
func (r *PlatformRepository) ListPrivilegedActionsForOperator(ctx context.Context, operatorID string, limit int32) ([]PrivilegedAction, error) {
	if _, err := pgUUID(operatorID); err != nil {
		return nil, apperror.ErrValidation
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT a.id::text, a.kind, a.reason,
		       COALESCE(NULLIF(requester.email, ''), a.requested_by),
		       COALESCE(NULLIF(approver.email, ''), a.approved_by),
		       a.requested_at, a.executed_at,
		       COALESCE(a.payload->>'operator_name', ''),
		       COALESCE((a.payload->>'admin_count_at_request')::int, 1)
		FROM privileged_actions a
		LEFT JOIN "user" requester ON requester.id = a.requested_by
		LEFT JOIN "user" approver ON approver.id = a.approved_by
		WHERE a.payload->>'operator_id' = $1
		ORDER BY a.requested_at DESC
		LIMIT $2`, operatorID, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	actions := make([]PrivilegedAction, 0)
	for rows.Next() {
		var action PrivilegedAction
		if err := rows.Scan(&action.ID, &action.Kind, &action.Reason, &action.RequestedBy,
			&action.ApprovedBy, &action.RequestedAt, &action.ExecutedAt,
			&action.OperatorName, &action.AdminCount); err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}
