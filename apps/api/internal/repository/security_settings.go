package repository

import (
	"context"
	"errors"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
)

type SecuritySettingsRepository struct {
	queries *db.Queries
}

func NewSecuritySettingsRepository(queries *db.Queries) *SecuritySettingsRepository {
	return &SecuritySettingsRepository{queries: queries}
}

// Get returns the zero-value (disabled, no CIDRs) when the operator has
// never configured this — the common case, and the one that must cost
// nothing more than a lookup that finds no row.
func (r *SecuritySettingsRepository) Get(ctx context.Context, operatorID string) (*domain.SecuritySettings, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.GetOperatorSecuritySettings(ctx, op)
	if errors.Is(err, pgx.ErrNoRows) {
		return &domain.SecuritySettings{OperatorID: operatorID}, nil
	}
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.SecuritySettings{OperatorID: operatorID, Enabled: row.IpAllowlistEnabled, CIDRs: row.IpAllowlistCidrs}, nil
}

func (r *SecuritySettingsRepository) Set(ctx context.Context, operatorID string, enabled bool, cidrs []string, updatedByUserID string) (*domain.SecuritySettings, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if cidrs == nil {
		cidrs = []string{}
	}
	row, err := r.queries.UpsertOperatorSecuritySettings(ctx, db.UpsertOperatorSecuritySettingsParams{
		OperatorID: op, IpAllowlistEnabled: enabled, IpAllowlistCidrs: cidrs, UpdatedByUserID: updatedByUserID,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.SecuritySettings{OperatorID: operatorID, Enabled: row.IpAllowlistEnabled, CIDRs: row.IpAllowlistCidrs}, nil
}

func (r *SecuritySettingsRepository) ListActiveSessions(ctx context.Context, operatorID string) ([]*domain.ActiveSession, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListOperatorActiveSessions(ctx, op)
	if err != nil {
		return nil, databaseError(err)
	}
	out := make([]*domain.ActiveSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.ActiveSession{
			ID: row.ID, UserName: row.Name, UserEmail: row.Email,
			IPAddress: row.IpAddress.String, UserAgent: row.UserAgent.String, CreatedAt: row.CreatedAt.Time,
		})
	}
	return out, nil
}

// RevokeSession deletes a session scoped to this operator's own members —
// see the query's own comment. Returns ErrNotFound rather than silently
// doing nothing when the id belongs to nobody in this operator (including a
// different tenant's session id), so the caller can tell "revoked" from
// "there was nothing to revoke here".
func (r *SecuritySettingsRepository) RevokeSession(ctx context.Context, operatorID, sessionID string) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	affected, err := r.queries.RevokeOperatorSession(ctx, db.RevokeOperatorSessionParams{ID: sessionID, ID_2: op})
	if err != nil {
		return databaseError(err)
	}
	if affected == 0 {
		return apperror.ErrNotFound
	}
	return nil
}
