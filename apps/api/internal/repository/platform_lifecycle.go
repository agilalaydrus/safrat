package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
)

// D2 (TUGAS-PANEL-SAAS.md, §7.1 DESAIN): extending one tenant's trial is a
// mandatory-reason settings action, not a four-eyes one — nothing about it is
// hard to reverse the way SUSPEND or DELETE_TENANT are, so it follows
// SetGracePeriod's shape (advisory lock + idempotency dedup + plain audit_logs)
// rather than Suspend/Reinstate's privileged_actions ledger.

type ExtendTrialChange struct {
	OperatorID     string
	AdditionalDays int32
	Reason         string
	// Confirmation must be the tenant's own name, typed by hand — same
	// mistake-proofing as Suspend/SetGracePeriod: catches an admin who
	// clicked the wrong row in a list of many tenants.
	Confirmation   string
	ActorUserID    string
	IdempotencyKey string
}

// ExtendTrial only ever adds to access_until — it never shortens one, and it
// refuses outright on anything but a TRIALING subscription: extending a paid
// tenant's access is a different action (a credit or a discount), not this
// one.
func (r *SubscriptionRepository) ExtendTrial(ctx context.Context, change ExtendTrialChange) (*time.Time, error) {
	if change.AdditionalDays <= 0 || change.AdditionalDays > 90 ||
		strings.TrimSpace(change.Reason) == "" || strings.TrimSpace(change.ActorUserID) == "" ||
		strings.TrimSpace(change.IdempotencyKey) == "" {
		return nil, apperror.ErrValidation
	}
	operator, err := pgUUID(change.OperatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "subscription-trial:"+change.OperatorID); err != nil {
		return nil, databaseError(err)
	}

	fingerprint, _, err := requestFingerprint(change)
	if err != nil {
		return nil, err
	}
	duplicate, err := checkPlatformMutationKey(ctx, tx, change.ActorUserID, "trial_extended", change.IdempotencyKey, fingerprint)
	if err != nil {
		return nil, err
	}
	if duplicate {
		var accessUntil time.Time
		if err := tx.QueryRow(ctx, `SELECT access_until FROM subscriptions WHERE operator_id = $1`, operator).Scan(&accessUntil); err != nil {
			return nil, databaseError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, databaseError(err)
		}
		return &accessUntil, nil
	}

	var name, status string
	if err := tx.QueryRow(ctx, `
		SELECT o.name, s.status::text FROM operators o
		JOIN subscriptions s ON s.operator_id = o.id
		WHERE o.id = $1 FOR UPDATE`, operator).Scan(&name, &status); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTenantNotFound
	} else if err != nil {
		return nil, databaseError(err)
	}
	if !strings.EqualFold(strings.TrimSpace(change.Confirmation), strings.TrimSpace(name)) {
		return nil, apperror.ErrValidation
	}
	if status != "TRIALING" {
		return nil, fmt.Errorf("%w: hanya berlaku untuk langganan yang masih trial (status saat ini: %s)", apperror.ErrFailedPrecondition, status)
	}

	var newAccessUntil time.Time
	if err := tx.QueryRow(ctx, `
		UPDATE subscriptions SET access_until = access_until + make_interval(days => $2), updated_at = NOW()
		WHERE operator_id = $1
		RETURNING access_until`, operator, change.AdditionalDays).Scan(&newAccessUntil); err != nil {
		return nil, databaseError(err)
	}

	if err := insertPlatformMutationAudit(ctx, tx, change.OperatorID, change.ActorUserID, "trial_extended",
		"operator", change.OperatorID, change.Reason, change.IdempotencyKey, fingerprint); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, databaseError(err)
	}
	return &newAccessUntil, nil
}

// D5 (TUGAS-PANEL-SAAS.md, §7.3 DESAIN): cancellation is not deletion.
// cancelled_at is set and access_until is left exactly as it is — the
// customer keeps what they already paid for, which is their right, not a
// courtesy. Access continues to be governed entirely by access_until/
// suspended_at, the same as always; nothing here needs to touch that.

type CancelSubscriptionChange struct {
	OperatorID     string
	Reason         string
	Confirmation   string
	ActorUserID    string
	IdempotencyKey string
}

type CancelSubscriptionResult struct {
	CancelledAt time.Time
	AccessUntil time.Time
}

func (r *SubscriptionRepository) Cancel(ctx context.Context, change CancelSubscriptionChange) (*CancelSubscriptionResult, error) {
	if strings.TrimSpace(change.Reason) == "" || strings.TrimSpace(change.ActorUserID) == "" ||
		strings.TrimSpace(change.IdempotencyKey) == "" {
		return nil, apperror.ErrValidation
	}
	operator, err := pgUUID(change.OperatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "subscription-cancel:"+change.OperatorID); err != nil {
		return nil, databaseError(err)
	}

	fingerprint, _, err := requestFingerprint(change)
	if err != nil {
		return nil, err
	}
	duplicate, err := checkPlatformMutationKey(ctx, tx, change.ActorUserID, "subscription_cancelled", change.IdempotencyKey, fingerprint)
	if err != nil {
		return nil, err
	}
	if duplicate {
		var result CancelSubscriptionResult
		if err := tx.QueryRow(ctx, `SELECT cancelled_at, access_until FROM subscriptions WHERE operator_id = $1`, operator).
			Scan(&result.CancelledAt, &result.AccessUntil); err != nil {
			return nil, databaseError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, databaseError(err)
		}
		return &result, nil
	}

	var name string
	var existingCancelledAt *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT o.name, s.cancelled_at FROM operators o
		JOIN subscriptions s ON s.operator_id = o.id
		WHERE o.id = $1 FOR UPDATE`, operator).Scan(&name, &existingCancelledAt); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTenantNotFound
	} else if err != nil {
		return nil, databaseError(err)
	}
	if !strings.EqualFold(strings.TrimSpace(change.Confirmation), strings.TrimSpace(name)) {
		return nil, apperror.ErrValidation
	}
	if existingCancelledAt != nil {
		return nil, fmt.Errorf("%w: langganan sudah dibatalkan sebelumnya", apperror.ErrFailedPrecondition)
	}

	var result CancelSubscriptionResult
	if err := tx.QueryRow(ctx, `
		UPDATE subscriptions SET cancelled_at = NOW(), updated_at = NOW()
		WHERE operator_id = $1
		RETURNING cancelled_at, access_until`, operator).Scan(&result.CancelledAt, &result.AccessUntil); err != nil {
		return nil, databaseError(err)
	}

	if err := insertPlatformMutationAudit(ctx, tx, change.OperatorID, change.ActorUserID, "subscription_cancelled",
		"operator", change.OperatorID, change.Reason, change.IdempotencyKey, fingerprint); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, databaseError(err)
	}
	return &result, nil
}
