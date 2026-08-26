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
	_, err = pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1, $2, 'Domain Test', 'ID', 'domain@example.com', $3)`,
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
