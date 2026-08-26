package middleware

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type stubSource struct {
	hostnames []string
	err       error
	calls     int
}

func (s *stubSource) ListVerifiedHostnames(context.Context) ([]string, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.hostnames, nil
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestTenantOriginAllowlistMatchesOnlyVerifiedHosts(t *testing.T) {
	source := &stubSource{hostnames: []string{"UmrohVacana.com"}}
	allowlist := NewTenantOriginAllowlist(source, quietLogger(), time.Minute, false)
	ctx := context.Background()

	if !allowlist.Allows(ctx, "https://umrohvacana.com") {
		t.Fatal("a verified hostname was rejected")
	}
	// Stored case must not decide the match.
	if !allowlist.Allows(ctx, "https://UMROHVACANA.COM") {
		t.Fatal("case-different origin was rejected")
	}
	// A verified apex must not admit subdomains the operator may not control.
	if allowlist.Allows(ctx, "https://evil.umrohvacana.com") {
		t.Fatal("a subdomain of a verified host was allowed")
	}
	if allowlist.Allows(ctx, "https://umrohvacana.com.attacker.test") {
		t.Fatal("a suffix-extended host was allowed")
	}
	if allowlist.Allows(ctx, "https://not-a-tenant.test") {
		t.Fatal("an unknown host was allowed")
	}
	// Plain http must be refused unless explicitly enabled for local dev.
	if allowlist.Allows(ctx, "http://umrohvacana.com") {
		t.Fatal("http origin allowed while allowPlainHTTP is false")
	}
	if allowlist.Allows(ctx, "") || allowlist.Allows(ctx, "null") {
		t.Fatal("an empty or opaque origin was allowed")
	}
}

func TestTenantOriginAllowlistAllowsPlainHTTPOnlyWhenEnabled(t *testing.T) {
	source := &stubSource{hostnames: []string{"umrohvacana.test"}}
	allowlist := NewTenantOriginAllowlist(source, quietLogger(), time.Minute, true)
	if !allowlist.Allows(context.Background(), "http://umrohvacana.test:3141") {
		t.Fatal("local http origin rejected while allowPlainHTTP is true")
	}
}

func TestTenantOriginAllowlistCachesAndSurvivesFailures(t *testing.T) {
	source := &stubSource{hostnames: []string{"umrohvacana.com"}}
	allowlist := NewTenantOriginAllowlist(source, quietLogger(), time.Hour, false)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		allowlist.Allows(ctx, "https://umrohvacana.com")
	}
	if source.calls != 1 {
		t.Fatalf("source called %d times, want 1 within the TTL", source.calls)
	}

	// A failing refresh must not lock every tenant out of the API.
	allowlist.ttl = 0
	source.err = errors.New("database unavailable")
	if !allowlist.Allows(ctx, "https://umrohvacana.com") {
		t.Fatal("a refresh failure revoked an already-verified origin")
	}
}
