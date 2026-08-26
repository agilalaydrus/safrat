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

// Domain is one hostname an operator has claimed.
type Domain struct {
	ID                string
	Hostname          string
	VerificationToken string
	Verified          bool
	IsPrimary         bool
}

// Add claims a hostname for an operator. The hostname is normalized first, so
// a claim can never differ from what the resolver will later look up.
func (r *OperatorDomainRepository) Add(ctx context.Context, operatorID, host string) (Domain, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return Domain{}, apperror.ErrValidation
	}
	hostname := NormalizeHostname(host)
	if !isRoutableHostname(hostname) {
		return Domain{}, apperror.ErrValidation
	}
	var domain Domain
	err = r.pool.QueryRow(ctx, `
		INSERT INTO operator_domains (operator_id, hostname)
		VALUES ($1, $2)
		RETURNING id::text, hostname, verification_token, verified_at IS NOT NULL, is_primary`,
		id, hostname).Scan(&domain.ID, &domain.Hostname, &domain.VerificationToken, &domain.Verified, &domain.IsPrimary)
	if err != nil {
		if strings.Contains(err.Error(), "operator_domains_hostname_key") {
			return Domain{}, apperror.ErrConflict
		}
		return Domain{}, err
	}
	return domain, nil
}

// ListForOperator returns every hostname the operator has claimed.
func (r *OperatorDomainRepository) ListForOperator(ctx context.Context, operatorID string) ([]Domain, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, hostname, verification_token, verified_at IS NOT NULL, is_primary
		FROM operator_domains WHERE operator_id = $1 ORDER BY created_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	domains := make([]Domain, 0, 4)
	for rows.Next() {
		var domain Domain
		if err := rows.Scan(&domain.ID, &domain.Hostname, &domain.VerificationToken, &domain.Verified, &domain.IsPrimary); err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	return domains, rows.Err()
}

// Get returns one of the operator's own domains. Scoping by operator here is
// what stops one operator verifying or deleting another's hostname.
func (r *OperatorDomainRepository) Get(ctx context.Context, operatorID, domainID string) (Domain, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return Domain{}, apperror.ErrValidation
	}
	target, err := pgUUID(domainID)
	if err != nil {
		return Domain{}, apperror.ErrValidation
	}
	var domain Domain
	err = r.pool.QueryRow(ctx, `
		SELECT id::text, hostname, verification_token, verified_at IS NOT NULL, is_primary
		FROM operator_domains WHERE operator_id = $1 AND id = $2`, id, target).
		Scan(&domain.ID, &domain.Hostname, &domain.VerificationToken, &domain.Verified, &domain.IsPrimary)
	if errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, apperror.ErrNotFound
	}
	return domain, err
}

// MarkVerified records proven ownership. Idempotent: re-verifying an already
// verified domain keeps the original timestamp.
func (r *OperatorDomainRepository) MarkVerified(ctx context.Context, operatorID, domainID string) error {
	id, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	target, err := pgUUID(domainID)
	if err != nil {
		return apperror.ErrValidation
	}
	command, err := r.pool.Exec(ctx, `
		UPDATE operator_domains SET verified_at = COALESCE(verified_at, NOW())
		WHERE operator_id = $1 AND id = $2`, id, target)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// Remove releases a hostname so it can be claimed again.
func (r *OperatorDomainRepository) Remove(ctx context.Context, operatorID, domainID string) error {
	id, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	target, err := pgUUID(domainID)
	if err != nil {
		return apperror.ErrValidation
	}
	command, err := r.pool.Exec(ctx, `DELETE FROM operator_domains WHERE operator_id = $1 AND id = $2`, id, target)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// isRoutableHostname rejects what could never be a client's own domain, so
// obvious mistakes fail in the CMS rather than at certificate issuance.
func isRoutableHostname(hostname string) bool {
	if len(hostname) < 4 || len(hostname) > 253 || !strings.Contains(hostname, ".") {
		return false
	}
	if strings.HasPrefix(hostname, ".") || strings.HasSuffix(hostname, ".") || strings.Contains(hostname, "..") {
		return false
	}
	for _, label := range strings.Split(hostname, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, char := range label {
			if !(char == '-' || (char >= '0' && char <= '9') || (char >= 'a' && char <= 'z')) {
				return false
			}
		}
	}
	return true
}
