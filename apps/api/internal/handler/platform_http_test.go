package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The platform panel reads across every tenant, so the only thing standing
// between an ordinary operator user and everybody else's data is one check.
// This drives it over real HTTP, through the real interceptor.
func TestPlatformPanelIsClosedToOperatorStaffIntegration(t *testing.T) {
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// An owner of a real operator — the most senior identity a tenant has.
	fixture := newHTTPFixture(t, pool)

	queries := db.New(pool)
	platform := service.NewPlatformService(
		repository.NewPlatformRepository(pool), repository.NewSupplierCostRepository(pool),
		repository.NewAuditRepository(queries))
	path, serviceHandler := hajjv1connect.NewPlatformServiceHandler(
		handler.NewPlatformHandler(platform),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool,
			repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)),
			repository.NewSubscriptionRepository(pool))),
	)
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := hajjv1connect.NewPlatformServiceClient(server.Client(), server.URL)

	authorised := func(token string) *connect.Request[hajjv1.ListPlatformOperatorsRequest] {
		request := connect.NewRequest(&hajjv1.ListPlatformOperatorsRequest{})
		if token != "" {
			request.Header().Set("Authorization", "Bearer "+token)
		}
		return request
	}

	// No session at all.
	if _, err := client.ListOperators(ctx, authorised("")); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("unauthenticated call returned %v (%v), want unauthenticated", connect.CodeOf(err), err)
	}

	// A real, valid session belonging to an operator owner. This is the case
	// that matters: the session is genuine, the user is senior in their own
	// organisation, and they still must not see the platform.
	if _, err := client.ListOperators(ctx, authorised(fixture.sessionToken)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("an operator owner got %v (%v) from the platform panel, want permission_denied", connect.CodeOf(err), err)
	}

	// They may ask whether they are an admin — it answers about them alone —
	// and the answer is no.
	selfRequest := connect.NewRequest(&hajjv1.AmIPlatformAdminRequest{})
	selfRequest.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	self, err := client.AmIPlatformAdmin(ctx, selfRequest)
	if err != nil {
		t.Fatalf("AmIPlatformAdmin: %v", err)
	}
	if self.Msg.IsPlatformAdmin {
		t.Fatal("an operator owner is reported as a platform admin")
	}

	// Granted access, the same session works.
	var userID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&userID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id, note) VALUES ($1, 'uji')`, userID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, userID)
	})

	operators, err := client.ListOperators(ctx, authorised(fixture.sessionToken))
	if err != nil {
		t.Fatalf("platform admin was refused: %v", err)
	}
	if len(operators.Msg.Operators) == 0 {
		t.Fatal("the platform view returned no operators at all")
	}

	// Revoking bites on the next request — no cached decision.
	if _, err := pool.Exec(ctx, `DELETE FROM platform_admins WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := client.ListOperators(ctx, authorised(fixture.sessionToken)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("a revoked admin still had access (%v)", connect.CodeOf(err))
	}
}

// A manual cost must not overwrite what a supplier actually charged, and that
// has to hold through the panel, not only in the repository.
func TestPlatformCostSettingRespectsObservedCostsIntegration(t *testing.T) {
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	fixture := newHTTPFixture(t, pool)
	var userID, productID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&userID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id) VALUES ($1)`, userID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, userID)
	})
	if err := pool.QueryRow(ctx, `SELECT id::text FROM products WHERE operator_id = $1`, fixture.operatorID).Scan(&productID); err != nil {
		t.Fatalf("read product: %v", err)
	}

	queries := db.New(pool)
	costs := repository.NewSupplierCostRepository(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool), costs, repository.NewAuditRepository(queries))
	path, serviceHandler := hajjv1connect.NewPlatformServiceHandler(
		handler.NewPlatformHandler(platform),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool,
			repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)),
			repository.NewSubscriptionRepository(pool))),
	)
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := hajjv1connect.NewPlatformServiceClient(server.Client(), server.URL)

	setCost := func(amount int64) (*connect.Response[hajjv1.SetProductSupplierCostResponse], error) {
		request := connect.NewRequest(&hajjv1.SetProductSupplierCostRequest{ProductId: productID, SupplierCostIdr: amount})
		request.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
		return client.SetProductSupplierCost(ctx, request)
	}

	response, err := setCost(1_200_000)
	if err != nil {
		t.Fatalf("set cost: %v", err)
	}
	if response.Msg.Product.SupplierCostIdr != 1_200_000 || response.Msg.Product.SupplierCostSource != "MANUAL" {
		t.Fatalf("cost = %d (%s), want 1200000 MANUAL",
			response.Msg.Product.SupplierCostIdr, response.Msg.Product.SupplierCostSource)
	}

	// The supplier's own figure arrives and takes over.
	if err := costs.RecordObservation(ctx, fixture.operatorID, productID, "", 1_350_000, "SUP-9"); err != nil {
		t.Fatalf("observe: %v", err)
	}
	// After which no amount of typing can push it back down.
	if _, err := setCost(100_000); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("overwriting an observed cost returned %v, want failed_precondition", connect.CodeOf(err))
	}
}
