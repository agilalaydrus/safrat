package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ImpersonationRepository struct {
	pool *pgxpool.Pool
}

func NewImpersonationRepository(pool *pgxpool.Pool) *ImpersonationRepository {
	return &ImpersonationRepository{pool: pool}
}

// MaxImpersonationMinutes bounds a session no matter what the caller asks for.
// A window long enough to look at a screen, short enough that forgetting to
// close it is not the same as leaving a key in the door.
const MaxImpersonationMinutes = 60

type ImpersonationSession struct {
	ID              string
	AdminUserID     string
	OperatorID      string
	OperatorName    string
	BetterAuthOrgID string
	Reason          string
	IP              string
	UserAgent       string
	StartedAt       time.Time
	ExpiresAt       time.Time
	EndedAt         *time.Time
	EndedReason     string
}

type StartImpersonation struct {
	AdminUserID    string
	OperatorID     string
	Reason         string
	Minutes        int32
	IP             string
	UserAgent      string
	IdempotencyKey string
}

// hashToken is the only way a token ever reaches the database.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Start opens a session and returns the token exactly once. It is not stored
// and cannot be recovered: losing it means starting a new session, which is the
// correct trade for a credential that stands in for a customer.
func (r *ImpersonationRepository) Start(ctx context.Context, start StartImpersonation) (ImpersonationSession, string, error) {
	session := ImpersonationSession{}
	if start.Minutes <= 0 || start.Minutes > MaxImpersonationMinutes {
		start.Minutes = MaxImpersonationMinutes
	}
	if len(strings.TrimSpace(start.Reason)) < 10 {
		return session, "", apperror.ErrValidation
	}
	operator, err := pgUUID(start.OperatorID)
	if err != nil {
		return session, "", apperror.ErrValidation
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return session, "", err
	}
	token := hex.EncodeToString(raw)

	err = r.pool.QueryRow(ctx, `
		INSERT INTO impersonation_sessions
		  (admin_user_id, operator_id, token_hash, reason, ip, user_agent, expires_at, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6, NOW() + make_interval(mins => $7::int), $8)
		RETURNING id::text, started_at, expires_at`,
		start.AdminUserID, operator, hashToken(token), strings.TrimSpace(start.Reason),
		start.IP, start.UserAgent, start.Minutes, strings.TrimSpace(start.IdempotencyKey)).
		Scan(&session.ID, &session.StartedAt, &session.ExpiresAt)
	if err != nil {
		// A retried request must settle the first session rather than open a
		// second one — but the token of the first is gone, so the honest answer
		// is a conflict the caller can see rather than a silent no-op.
		if IsUniqueViolation(err, "impersonation_sessions_idempotency_key_key") {
			return session, "", apperror.ErrConflict
		}
		return session, "", databaseError(err)
	}
	session.AdminUserID = start.AdminUserID
	session.OperatorID = start.OperatorID
	session.Reason = strings.TrimSpace(start.Reason)
	session.IP = start.IP
	session.UserAgent = start.UserAgent
	return session, token, nil
}

// Resolve validates a token on every impersonated request. It returns the
// tenant being impersonated, or ErrNotFound — an expired, ended or unknown
// token are deliberately indistinguishable to the caller.
func (r *ImpersonationRepository) Resolve(ctx context.Context, token string) (ImpersonationSession, error) {
	session := ImpersonationSession{}
	if len(token) != 64 {
		return session, apperror.ErrNotFound
	}
	err := r.pool.QueryRow(ctx, `
		SELECT i.id::text, i.admin_user_id, i.operator_id::text, o.name, o.better_auth_org_id,
		       i.reason, i.started_at, i.expires_at
		FROM impersonation_sessions i
		JOIN operators o ON o.id = i.operator_id
		WHERE i.token_hash = $1 AND i.ended_at IS NULL AND i.expires_at > NOW()`,
		hashToken(token)).Scan(&session.ID, &session.AdminUserID, &session.OperatorID,
		&session.OperatorName, &session.BetterAuthOrgID, &session.Reason,
		&session.StartedAt, &session.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return session, apperror.ErrNotFound
	}
	if err != nil {
		return session, databaseError(err)
	}
	return session, nil
}

// End closes a session early. Ending one that is already closed is not an
// error: the panel closing a session it has forgotten about must not fail.
func (r *ImpersonationRepository) End(ctx context.Context, token, reason string) error {
	if len(token) != 64 {
		return apperror.ErrNotFound
	}
	if _, err := r.pool.Exec(ctx, `
		UPDATE impersonation_sessions
		SET ended_at = NOW(), ended_reason = $2
		WHERE token_hash = $1 AND ended_at IS NULL`, hashToken(token), reason); err != nil {
		return databaseError(err)
	}
	return nil
}

// ListForOperator is what the tenant page shows: who has been inside this
// account, when, for how long, and why.
func (r *ImpersonationRepository) ListForOperator(ctx context.Context, operatorID string, limit int32) ([]ImpersonationSession, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT i.id::text, i.admin_user_id, COALESCE(NULLIF(u.email, ''), i.admin_user_id),
		       i.reason, i.ip, i.started_at, i.expires_at, i.ended_at, i.ended_reason
		FROM impersonation_sessions i
		LEFT JOIN "user" u ON u.id = i.admin_user_id
		WHERE i.operator_id = $1
		ORDER BY i.started_at DESC
		LIMIT $2`, operator, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	sessions := make([]ImpersonationSession, 0)
	for rows.Next() {
		var session ImpersonationSession
		if err := rows.Scan(&session.ID, &session.AdminUserID, &session.OperatorName,
			&session.Reason, &session.IP, &session.StartedAt, &session.ExpiresAt,
			&session.EndedAt, &session.EndedReason); err != nil {
			return nil, err
		}
		session.OperatorID = operatorID
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}
