package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OperatorDomainRepository resolves custom client hostnames to an operator.
//
// Platform subdomains are NOT stored here — they stay derived from the slug in
// the hostname (extractTenantSlug in apps/web/lib/tenant-host.ts), so existing
// tenants keep resolving exactly as before and nothing needs backfilling.
// This table holds only what cannot be derived: domains a client owns.
type OperatorDomainRepository struct {
	pool *pgxpool.Pool
}

func NewOperatorDomainRepository(pool *pgxpool.Pool) *OperatorDomainRepository {
	return &OperatorDomainRepository{pool: pool}
}

// NormalizeHostname strips the port, lowercases, and drops a trailing dot so a
// Host header can never miss a stored row on formatting alone.
func NormalizeHostname(host string) string {
	hostname := strings.TrimSpace(host)
	if index := strings.Index(hostname, ":"); index >= 0 {
		hostname = hostname[:index]
	}
	return strings.TrimSuffix(strings.ToLower(hostname), ".")
}

// ResolveVerified returns the operator that owns a hostname.
//
// Only verified domains resolve. An unverified row means someone has claimed a
// hostname but has not proven they control it, and serving it would let anyone
// point a domain at us and impersonate a tenant.
func (r *OperatorDomainRepository) ResolveVerified(ctx context.Context, host string) (string, error) {
	hostname := NormalizeHostname(host)
	if hostname == "" || len(hostname) > 253 {
		return "", apperror.ErrValidation
	}
	var operatorID string
	err := r.pool.QueryRow(ctx, `
		SELECT operator_id::text
		FROM operator_domains
		WHERE hostname = $1 AND verified_at IS NOT NULL`, hostname).Scan(&operatorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return operatorID, nil
}

// ListVerifiedHostnames backs the CORS/trusted-origin allowlist and the
// on-demand TLS "ask" check, so one table decides every place a hostname is
// trusted.
func (r *OperatorDomainRepository) ListVerifiedHostnames(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT hostname FROM operator_domains WHERE verified_at IS NOT NULL ORDER BY hostname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hostnames := make([]string, 0, 16)
	for rows.Next() {
		var hostname string
		if err := rows.Scan(&hostname); err != nil {
			return nil, err
		}
		hostnames = append(hostnames, hostname)
	}
	return hostnames, rows.Err()
}
