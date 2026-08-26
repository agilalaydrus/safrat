package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOperatorDomainResolutionIntegration(t *testing.T) {
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	t.Cleanup(pool.Close)

	operatorID := uuid.NewString()
	// GROWTH: custom domains are a paid entitlement, so a STARTER fixture would
	// correctly refuse to resolve and this test would be measuring the wrong thing.
	_, err = pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan) VALUES ($1, $2, 'Domain Test', 'ID', 'domain@example.com', $3, 'GROWTH')`,
		operatorID, "domain-test-"+uuid.NewString(), "domain-"+operatorID[:8])
	if err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	domains := NewOperatorDomainRepository(pool)
	const hostname = "e2e-custom-domain.example"

	if _, err := pool.Exec(ctx, `INSERT INTO operator_domains (operator_id, hostname) VALUES ($1, $2)`, operatorID, hostname); err != nil {
		t.Fatalf("insert domain: %v", err)
	}

	// Unverified must not resolve: otherwise anyone could point a hostname at
	// us and be served as that tenant.
	if _, err := domains.ResolveVerified(ctx, hostname); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unverified domain resolved: %v", err)
	}
	if hostnames, err := domains.ListVerifiedHostnames(ctx); err != nil || contains(hostnames, hostname) {
		t.Fatalf("unverified hostname appeared in the allowlist: %v, %v", hostnames, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE operator_domains SET verified_at = NOW() WHERE hostname = $1`, hostname); err != nil {
		t.Fatalf("verify domain: %v", err)
	}

	resolved, err := domains.ResolveVerified(ctx, hostname)
	if err != nil || resolved != operatorID {
		t.Fatalf("resolved = %q (%v), want %q", resolved, err, operatorID)
	}
	// A Host header carries a port and arbitrary case; neither may cause a miss.
	for _, variant := range []string{"E2E-Custom-Domain.Example", hostname + ":443", hostname + "."} {
		got, err := domains.ResolveVerified(ctx, variant)
		if err != nil || got != operatorID {
			t.Fatalf("variant %q resolved = %q (%v)", variant, got, err)
		}
	}
	if _, err := domains.ResolveVerified(ctx, "not-a-tenant.example"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unknown hostname error = %v, want not found", err)
	}

	hostnames, err := domains.ListVerifiedHostnames(ctx)
	if err != nil || !contains(hostnames, hostname) {
		t.Fatalf("verified hostname missing from the allowlist: %v, %v", hostnames, err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// A verified domain must stop resolving when the operator's plan no longer
// includes custom domains — otherwise a downgrade leaves it served forever.
func TestOperatorDomainRespectsPlanEntitlementIntegration(t *testing.T) {
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	t.Cleanup(pool.Close)

	operatorID := uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan) VALUES ($1, $2, 'Plan Test', 'ID', 'plan@example.com', $3, 'STARTER')`,
		operatorID, "plan-test-"+uuid.NewString(), "plan-"+operatorID[:8])
	if err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	domains := NewOperatorDomainRepository(pool)
	const hostname = "starter-should-not-serve.example"

	// Starter cannot claim one at all.
	if _, err := domains.Add(ctx, operatorID, hostname); !errors.Is(err, ErrPlanForbidsCustomDomain) {
		t.Fatalf("Starter was allowed to add a domain: %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE operators SET plan = 'GROWTH' WHERE id = $1`, operatorID); err != nil {
		t.Fatalf("upgrade plan: %v", err)
	}
	domain, err := domains.Add(ctx, operatorID, hostname)
	if err != nil {
		t.Fatalf("Growth could not add a domain: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE operator_domains SET verified_at = NOW() WHERE id = $1::uuid`, domain.ID); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if resolved, err := domains.ResolveVerified(ctx, hostname); err != nil || resolved != operatorID {
		t.Fatalf("verified domain on Growth did not resolve: %q, %v", resolved, err)
	}

	// Downgrade: the row stays, but it must no longer be served or trusted.
	if _, err := pool.Exec(ctx, `UPDATE operators SET plan = 'STARTER' WHERE id = $1`, operatorID); err != nil {
		t.Fatalf("downgrade plan: %v", err)
	}
	if _, err := domains.ResolveVerified(ctx, hostname); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("domain still resolved after downgrade: %v", err)
	}
	hostnames, err := domains.ListVerifiedHostnames(ctx)
	if err != nil || contains(hostnames, hostname) {
		t.Fatalf("downgraded domain still in the CORS allowlist: %v, %v", hostnames, err)
	}
}

// The first domain an operator verifies should become canonical on its own —
// otherwise they prove ownership and search engines still point at the platform
// subdomain until someone finds a separate control.
func TestOperatorDomainFirstVerifiedBecomesPrimaryIntegration(t *testing.T) {
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	t.Cleanup(pool.Close)

	operatorID := uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan) VALUES ($1, $2, 'Primary Test', 'ID', 'primary@example.com', $3, 'GROWTH')`,
		operatorID, "primary-test-"+uuid.NewString(), "primary-"+operatorID[:8])
	if err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	domains := NewOperatorDomainRepository(pool)
	first, err := domains.Add(ctx, operatorID, "first-domain.example")
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	second, err := domains.Add(ctx, operatorID, "second-domain.example")
	if err != nil {
		t.Fatalf("add second: %v", err)
	}

	// Nothing is canonical while nothing is verified.
	if host, err := domains.PrimaryHostname(ctx, operatorID); err != nil || host != "" {
		t.Fatalf("primary before verification = %q (%v), want empty", host, err)
	}

	if err := domains.MarkVerified(ctx, operatorID, first.ID); err != nil {
		t.Fatalf("verify first: %v", err)
	}
	if host, err := domains.PrimaryHostname(ctx, operatorID); err != nil || host != "first-domain.example" {
		t.Fatalf("primary after first verification = %q (%v)", host, err)
	}

	// Verifying a second must not silently move the canonical address; that is
	// a deliberate choice an operator makes, not a side effect.
	if err := domains.MarkVerified(ctx, operatorID, second.ID); err != nil {
		t.Fatalf("verify second: %v", err)
	}
	if host, err := domains.PrimaryHostname(ctx, operatorID); err != nil || host != "first-domain.example" {
		t.Fatalf("primary moved on second verification: %q (%v)", host, err)
	}

	// A downgrade must drop the canonical host, not point search engines at a
	// domain we have stopped serving.
	if _, err := pool.Exec(ctx, `UPDATE operators SET plan = 'STARTER' WHERE id = $1`, operatorID); err != nil {
		t.Fatalf("downgrade: %v", err)
	}
	if host, err := domains.PrimaryHostname(ctx, operatorID); err != nil || host != "" {
		t.Fatalf("primary after downgrade = %q (%v), want empty", host, err)
	}
}
