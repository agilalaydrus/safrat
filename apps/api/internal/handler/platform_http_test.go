package handler_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/crypto"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		repository.NewPlatformRepository(pool), repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool), repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool), repository.NewKYCRepository(pool),
		repository.NewAuditRepository(queries), repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool), repository.NewPersonalDataReadRepository(pool))
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

	authorised := func(token string) *connect.Request[hajjv1.ListOperatorsRequest] {
		request := connect.NewRequest(&hajjv1.ListOperatorsRequest{})
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
	// Platform access also demands an enrolled second factor; that rule has its
	// own test below, so here it is simply satisfied.
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, userID); err != nil {
		t.Fatalf("enrol: %v", err)
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

func TestPlatformPlanControlRPCAccessRevocationAndMutationIntegration(t *testing.T) {
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
	var userID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token=$1`, fixture.sessionToken).Scan(&userID); err != nil {
		t.Fatalf("read session user: %v", err)
	}

	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool),
		repository.NewKYCRepository(pool), repository.NewAuditRepository(queries), repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool), repository.NewPersonalDataReadRepository(pool))
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

	planKey := "plan-http-" + fixture.operatorID
	overrideKey := "override-http-" + fixture.operatorID
	deleteKey := "delete-http-" + fixture.operatorID
	trialKey := "trial-http-" + fixture.operatorID
	var originalTrialDays string
	if err := pool.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key='trial_days'`).Scan(&originalTrialDays); err != nil {
		t.Fatalf("read trial setting: %v", err)
	}
	billingOperatorID := uuid.NewString()
	billingPeriod := time.Now().UTC().Truncate(time.Microsecond).Add(72 * time.Hour)
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan)
		VALUES ($1,$2,'Billing Row Success','ID',$3,$4,'STARTER')`, billingOperatorID, "billing-"+billingOperatorID,
		billingOperatorID[:8]+"@example.com", "billing-"+billingOperatorID[:8]); err != nil {
		t.Fatalf("prepare billing operator: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO subscriptions (operator_id,plan,status,access_until)
		VALUES ($1,'STARTER','ACTIVE',$2)`, billingOperatorID, billingPeriod); err != nil {
		t.Fatalf("prepare billing subscription: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, billingOperatorID) })
	prorationOperatorID := uuid.NewString()
	// Deliberately off a whole-day boundary, and built by Postgres rather than
	// by Go.
	//
	// Proration rounds remaining time up to whole days. A subscription sitting
	// at exactly fifteen days sits exactly on that boundary, and the preview and
	// the apply read the clock at two different instants — through two different
	// clocks, since one value came from Go and the other from NOW(). A few
	// microseconds either way flipped the preview to sixteen days while the
	// apply saw fifteen, and the guard correctly refused to charge an amount
	// nobody had approved. The production code was right; the fixture was
	// balanced on a knife edge.
	prorationInterval := "15 days 6 hours"
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan)
		VALUES ($1,$2,'Proration Row Success','ID',$3,$4,'STARTER')`, prorationOperatorID, "proration-"+prorationOperatorID,
		prorationOperatorID[:8]+"@example.com", "proration-"+prorationOperatorID[:8]); err != nil {
		t.Fatalf("prepare proration operator: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO subscriptions (operator_id,plan,status,access_until)
		VALUES ($1,'STARTER','ACTIVE',NOW() + $2::interval)`, prorationOperatorID, prorationInterval); err != nil {
		t.Fatalf("prepare proration subscription: %v", err)
	}
	prorationPreview, err := repository.NewSubscriptionRepository(pool).PreviewPlanChange(ctx, prorationOperatorID, "GROWTH")
	if err != nil {
		t.Fatalf("prepare proration preview: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, prorationOperatorID)
	})
	auth := func(request interface{ Header() http.Header }, token string) {
		if token != "" {
			request.Header().Set("Authorization", "Bearer "+token)
		}
	}
	calls := []struct {
		name string
		call func(string) error
	}{
		{"ListPlanLimits", func(token string) error {
			req := connect.NewRequest(&hajjv1.ListPlanLimitsRequest{})
			auth(req, token)
			_, err := client.ListPlanLimits(ctx, req)
			return err
		}},
		{"ListUsage", func(token string) error {
			req := connect.NewRequest(&hajjv1.ListUsageRequest{})
			auth(req, token)
			_, err := client.ListUsage(ctx, req)
			return err
		}},
		{"PreviewPlanLimitChange", func(token string) error {
			req := connect.NewRequest(&hajjv1.PreviewPlanLimitChangeRequest{Plan: "STARTER",
				MaxPilgrims: &hajjv1.QuotaValue{Value: 200}, MaxBranches: &hajjv1.QuotaValue{Value: 0}})
			auth(req, token)
			_, err := client.PreviewPlanLimitChange(ctx, req)
			return err
		}},
		{"SetPlanLimit", func(token string) error {
			req := connect.NewRequest(&hajjv1.SetPlanLimitRequest{Plan: "STARTER",
				MaxPilgrims: &hajjv1.QuotaValue{Value: 200}, MaxBranches: &hajjv1.QuotaValue{Value: 0},
				Reason: "verifikasi HTTP", Confirmation: "STARTER", IdempotencyKey: planKey})
			auth(req, token)
			_, err := client.SetPlanLimit(ctx, req)
			return err
		}},
		{"ListPlanOverrides", func(token string) error {
			req := connect.NewRequest(&hajjv1.ListPlanOverridesRequest{})
			auth(req, token)
			_, err := client.ListPlanOverrides(ctx, req)
			return err
		}},
		{"SetPlanOverride", func(token string) error {
			max := int32(201)
			req := connect.NewRequest(&hajjv1.SetPlanOverrideRequest{OperatorId: fixture.operatorID,
				MaxPilgrims: &max, Note: "verifikasi HTTP", IdempotencyKey: overrideKey})
			auth(req, token)
			_, err := client.SetPlanOverride(ctx, req)
			return err
		}},
		{"DeletePlanOverride", func(token string) error {
			req := connect.NewRequest(&hajjv1.DeletePlanOverrideRequest{OperatorId: fixture.operatorID,
				Reason: "verifikasi HTTP selesai", IdempotencyKey: deleteKey})
			auth(req, token)
			_, err := client.DeletePlanOverride(ctx, req)
			return err
		}},
		{"PreviewSubscriptionBilling", func(token string) error {
			req := connect.NewRequest(&hajjv1.PreviewSubscriptionBillingRequest{})
			auth(req, token)
			_, err := client.PreviewSubscriptionBilling(ctx, req)
			return err
		}},
		{"IssueSubscriptionBilling", func(token string) error {
			req := connect.NewRequest(&hajjv1.IssueSubscriptionBillingRequest{Targets: []*hajjv1.SubscriptionBillingTarget{
				{OperatorId: fixture.operatorID, Plan: "STARTER", PeriodStart: timestamppb.Now(), ExpectedBaseAmountIdr: 1},
				{OperatorId: billingOperatorID, Plan: "STARTER", PeriodStart: timestamppb.New(billingPeriod), ExpectedBaseAmountIdr: 589000},
			}})
			auth(req, token)
			response, err := client.IssueSubscriptionBilling(ctx, req)
			if err == nil && (response.Msg.IssuedCount != 1 || response.Msg.FailedCount != 1 || len(response.Msg.Results) != 2) {
				t.Fatalf("per-row billing result = issued %d failed %d rows %d, want 1/1/2",
					response.Msg.IssuedCount, response.Msg.FailedCount, len(response.Msg.Results))
			}
			return err
		}},
		{"GetSubscriptionBillingSettings", func(token string) error {
			req := connect.NewRequest(&hajjv1.GetSubscriptionBillingSettingsRequest{})
			auth(req, token)
			_, err := client.GetSubscriptionBillingSettings(ctx, req)
			return err
		}},
		{"SetTrialDays", func(token string) error {
			req := connect.NewRequest(&hajjv1.SetTrialDaysRequest{
				TrialDays: 10, Reason: "verifikasi HTTP trial", Confirmation: "TRIAL", IdempotencyKey: trialKey,
			})
			auth(req, token)
			_, err := client.SetTrialDays(ctx, req)
			return err
		}},
		{"SetSubscriptionGracePeriod", func(token string) error {
			days := int32(2)
			req := connect.NewRequest(&hajjv1.SetSubscriptionGracePeriodRequest{
				OperatorId: billingOperatorID, GracePeriodDays: &days, Reason: "verifikasi HTTP grace",
				Confirmation: "Billing Row Success", IdempotencyKey: "grace-http-" + billingOperatorID,
			})
			auth(req, token)
			_, err := client.SetSubscriptionGracePeriod(ctx, req)
			return err
		}},
		{"PreviewSubscriptionPlanChange", func(token string) error {
			req := connect.NewRequest(&hajjv1.PreviewSubscriptionPlanChangeRequest{OperatorId: prorationOperatorID, NewPlan: "GROWTH"})
			auth(req, token)
			_, err := client.PreviewSubscriptionPlanChange(ctx, req)
			return err
		}},
		// The funnel reads across every tenant at once. If it ever answered an
		// operator's own session it would hand one travel agency the visitor and
		// registration numbers of all its competitors.
		{"GetPlatformFunnel", func(token string) error {
			req := connect.NewRequest(&hajjv1.GetPlatformFunnelRequest{Days: 30})
			auth(req, token)
			_, err := client.GetPlatformFunnel(ctx, req)
			return err
		}},
		{"ApplySubscriptionPlanChange", func(token string) error {
			req := connect.NewRequest(&hajjv1.ApplySubscriptionPlanChangeRequest{
				OperatorId: prorationOperatorID, NewPlan: "GROWTH", ExpectedAdjustmentIdr: prorationPreview.AdjustmentIDR,
				Reason: "verifikasi HTTP prorata", Confirmation: "Proration Row Success",
				IdempotencyKey: "proration-http-" + prorationOperatorID,
			})
			auth(req, token)
			_, err := client.ApplySubscriptionPlanChange(ctx, req)
			return err
		}},
	}

	for _, call := range calls {
		if err := call.call(""); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Fatalf("%s tanpa sesi = %v (%v), want unauthenticated", call.name, connect.CodeOf(err), err)
		}
		if err := call.call(fixture.sessionToken); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatalf("%s oleh owner operator = %v (%v), want permission_denied", call.name, connect.CodeOf(err), err)
		}
	}

	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id,note) VALUES ($1,'uji plan control')`, userID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled"=true WHERE id=$1`, userID); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `UPDATE platform_settings SET value=$1 WHERE key='trial_days'`, originalTrialDays)
		_, _ = pool.Exec(bg, `DELETE FROM platform_admins WHERE user_id=$1`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM privileged_actions WHERE requested_by=$1`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM audit_logs WHERE user_id=$1 AND action IN ('plan_limit_changed','plan_override_set','plan_override_deleted','trial_days_changed')`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM plan_overrides WHERE operator_id=$1`, fixture.operatorID)
	})
	for _, call := range calls {
		if err := call.call(fixture.sessionToken); err != nil {
			t.Fatalf("%s oleh admin platform: %v", call.name, err)
		}
	}

	if _, err := pool.Exec(ctx, `DELETE FROM platform_admins WHERE user_id=$1`, userID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	for _, call := range calls {
		if err := call.call(fixture.sessionToken); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatalf("%s sesudah revoke = %v (%v), want permission_denied", call.name, connect.CodeOf(err), err)
		}
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
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, userID); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, userID)
	})
	if err := pool.QueryRow(ctx, `SELECT id::text FROM products WHERE operator_id = $1`, fixture.operatorID).Scan(&productID); err != nil {
		t.Fatalf("read product: %v", err)
	}

	queries := db.New(pool)
	costs := repository.NewSupplierCostRepository(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool), costs, repository.NewSupplierRepository(pool), repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool), repository.NewKYCRepository(pool), repository.NewAuditRepository(queries), repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool), repository.NewPersonalDataReadRepository(pool))
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

// Platform access reads every tenant's data, so being granted is not enough on
// its own — the account has to carry a second factor. Without this check the
// second factor would be optional for precisely the identity where it matters
// most.
func TestPlatformAccessRequiresTwoFactorIntegration(t *testing.T) {
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
	var userID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&userID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id, note) VALUES ($1, 'uji 2fa')`, userID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, userID)
	})

	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool), repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool), repository.NewKYCRepository(pool), repository.NewAuditRepository(queries), repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool), repository.NewPersonalDataReadRepository(pool))
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

	list := func() error {
		request := connect.NewRequest(&hajjv1.ListOperatorsRequest{})
		request.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
		_, err := client.ListOperators(ctx, request)
		return err
	}

	// Granted, but no second factor enrolled.
	if err := list(); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("an admin without 2FA got %v (%v), want failed_precondition", connect.CodeOf(err), err)
	}

	// The panel must be able to tell that apart from a plain refusal, or it
	// would tell an admin to ask for access they already have.
	selfRequest := connect.NewRequest(&hajjv1.AmIPlatformAdminRequest{})
	selfRequest.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	self, err := client.AmIPlatformAdmin(ctx, selfRequest)
	if err != nil {
		t.Fatalf("AmIPlatformAdmin: %v", err)
	}
	if !self.Msg.IsPlatformAdmin || self.Msg.TwoFactorEnabled {
		t.Fatalf("reported admin=%v twoFactor=%v, want true and false",
			self.Msg.IsPlatformAdmin, self.Msg.TwoFactorEnabled)
	}

	// Enrolled, and it opens.
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, userID); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	if err := list(); err != nil {
		t.Fatalf("an enrolled admin was refused: %v", err)
	}
}

// The supplier catalogue end to end, through the real interceptor and gate:
// register a supplier, teach it how to read one response shape, route a product
// at it, and confirm the panel's rule tester agrees with what the worker would
// conclude.
func TestSupplierCatalogueOverHTTPIntegration(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, userID); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, userID)
	})
	// A route only means anything on a digital product: fulfilment skips
	// TRAVEL_PACKAGE outright and treats EQUIPMENT as needing no supplier. The
	// fixture's own product is a travel package, so this creates the kind of
	// product a route is actually for — platform-owned, as migration 114
	// requires of digital categories.
	productID = uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO products (id,name,code,category,price_idr,base_price_idr,is_active)
		VALUES ($1,'Pulsa Uji Routing',$2,'PPOB_CREDIT',10000,10000,true)`,
		productID, "RTE-"+uuid.NewString()[:8]); err != nil {
		t.Fatalf("create digital product: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM product_routes WHERE product_id = $1`, productID)
		_, _ = pool.Exec(bg, `DELETE FROM products WHERE id = $1`, productID)
	})

	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool), repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool), repository.NewKYCRepository(pool),
		repository.NewAuditRepository(queries), repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool), repository.NewPersonalDataReadRepository(pool))
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

	auth := func(request interface{ Header() http.Header }) {
		request.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	}

	code := "uji-" + fixture.operatorID[:8]
	saveReq := connect.NewRequest(&hajjv1.SaveSupplierRequest{
		Name: "Supplier Uji", Code: code, BaseUrl: "https://supplier.invalid/api",
		CredentialEnvVar: "SUPPLIER_UJI_KEY", Status: "ACTIVE", Notes: "dibuat oleh test",
	})
	auth(saveReq)
	saved, err := client.SaveSupplier(ctx, saveReq)
	if err != nil {
		t.Fatalf("save supplier: %v", err)
	}
	supplierID := saved.Msg.Supplier.Id
	// Routes and rules both hold the supplier with a foreign key, so deleting
	// it alone fails — and because the error was discarded, it failed silently
	// on every run since this test was written.
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM product_routes WHERE supplier_id = $1`, supplierID)
		_, _ = pool.Exec(bg, `DELETE FROM supplier_response_rules WHERE supplier_id = $1`, supplierID)
		_, _ = pool.Exec(bg, `DELETE FROM suppliers WHERE id = $1`, supplierID)
	})

	// Saving the same code again updates rather than creating a second one —
	// renaming for display must not orphan a supplier's history.
	saveReq2 := connect.NewRequest(&hajjv1.SaveSupplierRequest{
		Name: "Supplier Uji (baru)", Code: code, Status: "ACTIVE",
	})
	auth(saveReq2)
	saved2, err := client.SaveSupplier(ctx, saveReq2)
	if err != nil {
		t.Fatalf("re-save supplier: %v", err)
	}
	if saved2.Msg.Supplier.Id != supplierID {
		t.Fatalf("saving the same code created a second supplier (%s vs %s)", saved2.Msg.Supplier.Id, supplierID)
	}

	// A pattern that cannot compile must be refused here, not discovered later
	// over live transactions.
	badReq := connect.NewRequest(&hajjv1.CreateResponseRuleRequest{
		SupplierId: supplierID, Priority: 10, Pattern: "(unclosed", Outcome: "SUCCESS",
	})
	auth(badReq)
	if _, err := client.CreateResponseRule(ctx, badReq); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a malformed pattern returned %v, want invalid_argument", connect.CodeOf(err))
	}
	// So must one naming a capture group it never defines.
	missingGroup := connect.NewRequest(&hajjv1.CreateResponseRuleRequest{
		SupplierId: supplierID, Priority: 10, Pattern: "OK", Outcome: "SUCCESS", ReferenceGroup: "ref",
	})
	auth(missingGroup)
	if _, err := client.CreateResponseRule(ctx, missingGroup); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a rule naming an absent group returned %v, want invalid_argument", connect.CodeOf(err))
	}

	goodReq := connect.NewRequest(&hajjv1.CreateResponseRuleRequest{
		SupplierId: supplierID, Priority: 10,
		Pattern:        `(?i)"status"\s*:\s*"SUCCESS".*?"sn"\s*:\s*"(?P<ref>[^"]+)".*?"harga"\s*:\s*(?P<cost>[0-9.]+)`,
		Outcome:        "SUCCESS",
		ReferenceGroup: "ref", CostGroup: "cost", Description: "format JSON standar",
	})
	auth(goodReq)
	if _, err := client.CreateResponseRule(ctx, goodReq); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// The tester is the point of the whole screen: try a pattern before
	// trusting it with money.
	testReq := connect.NewRequest(&hajjv1.TestResponseRulesRequest{
		SupplierId:     supplierID,
		SampleResponse: `{"status":"SUCCESS","sn":"SN-88123","harga":18.500}`,
	})
	auth(testReq)
	reading, err := client.TestResponseRules(ctx, testReq)
	if err != nil {
		t.Fatalf("test rules: %v", err)
	}
	if reading.Msg.Outcome != "SUCCESS" || reading.Msg.Reference != "SN-88123" {
		t.Fatalf("outcome=%s reference=%s, want SUCCESS and SN-88123", reading.Msg.Outcome, reading.Msg.Reference)
	}
	if !reading.Msg.CostReported || reading.Msg.CostIdr != 18_500 {
		t.Fatalf("cost=%d reported=%v, want 18500 reported", reading.Msg.CostIdr, reading.Msg.CostReported)
	}

	// Anything the rules do not recognise reads as UNMATCHED, never as a
	// failure — a response nobody taught the system to read must not refund a
	// transaction the supplier may have delivered.
	unknownReq := connect.NewRequest(&hajjv1.TestResponseRulesRequest{
		SupplierId: supplierID, SampleResponse: "OK 4711",
	})
	auth(unknownReq)
	unknown, err := client.TestResponseRules(ctx, unknownReq)
	if err != nil {
		t.Fatalf("test unknown: %v", err)
	}
	if unknown.Msg.Outcome != "UNMATCHED" {
		t.Fatalf("outcome = %s for an unrecognised response, want UNMATCHED", unknown.Msg.Outcome)
	}

	// Routing a product at the supplier, then confirming it is what the
	// fulfilment path would find.
	routeReq := connect.NewRequest(&hajjv1.SaveProductRouteRequest{
		ProductId: productID, SupplierId: supplierID, SupplierSku: "PULSA-10K", IsActive: true,
	})
	auth(routeReq)
	if _, err := client.SaveProductRoute(ctx, routeReq); err != nil {
		t.Fatalf("save route: %v", err)
	}
	listReq := connect.NewRequest(&hajjv1.ListProductRoutesRequest{})
	auth(listReq)
	routes, err := client.ListProductRoutes(ctx, listReq)
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}
	var found bool
	for _, route := range routes.Msg.Routes {
		if route.ProductId == productID && route.SupplierSku == "PULSA-10K" {
			found = true
		}
	}
	if !found {
		t.Fatal("the saved route is not in the list")
	}

	// And an operator owner without platform access sees none of it.
	other := newHTTPFixture(t, pool)
	deniedReq := connect.NewRequest(&hajjv1.ListSuppliersRequest{})
	deniedReq.Header().Set("Authorization", "Bearer "+other.sessionToken)
	if _, err := client.ListSuppliers(ctx, deniedReq); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("an operator owner reached the supplier catalogue (%v)", connect.CodeOf(err))
	}
}

// Granting and revoking platform access from the panel, which previously
// required a SQL client — the thing the panel exists to remove.
func TestPlatformAccountManagementIntegration(t *testing.T) {
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
	var userID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&userID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	// Start from a known state whatever a previous run left behind.
	if _, err := pool.Exec(ctx, `DELETE FROM platform_admins`); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id) VALUES ($1)`, userID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, userID); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins`) })

	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool), repository.NewKYCRepository(pool), repository.NewAuditRepository(queries), repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool), repository.NewPersonalDataReadRepository(pool))
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
	auth := func(r interface{ Header() http.Header }) {
		r.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	}

	// The account list finds the fixture staff, and reports what matters about
	// them: whether they hold platform access and whether they have 2FA.
	listReq := connect.NewRequest(&hajjv1.ListAccountsRequest{Search: "example.test", Limit: 50})
	auth(listReq)
	accounts, err := client.ListAccounts(ctx, listReq)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	var found *hajjv1.PlatformAccount
	for _, account := range accounts.Msg.Accounts {
		if account.UserId == userID {
			found = account
		}
	}
	if found == nil {
		t.Fatal("the signed-in admin is not in the account list")
	}
	if !found.IsPlatformAdmin || !found.TwoFactorEnabled || found.ActiveSessions < 1 {
		t.Fatalf("account reported admin=%v 2fa=%v sessions=%d",
			found.IsPlatformAdmin, found.TwoFactorEnabled, found.ActiveSessions)
	}

	// Revoking the only admin would lock the panel for everybody, and the way
	// back would be the SQL client this replaces.
	revokeSelf := connect.NewRequest(&hajjv1.RevokePlatformAdminRequest{UserId: userID})
	auth(revokeSelf)
	if _, err := client.RevokePlatformAdmin(ctx, revokeSelf); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("revoking the last admin returned %v, want failed_precondition", connect.CodeOf(err))
	}

	// With a second admin, revoking works.
	other := newHTTPFixture(t, pool)
	var otherUserID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, other.sessionToken).Scan(&otherUserID); err != nil {
		t.Fatalf("read second user: %v", err)
	}
	grant := connect.NewRequest(&hajjv1.GrantPlatformAdminRequest{UserId: otherUserID, Note: "cadangan"})
	auth(grant)
	if _, err := client.GrantPlatformAdmin(ctx, grant); err != nil {
		t.Fatalf("grant: %v", err)
	}
	// Granting twice is not an error: the caller asked for the access to exist.
	if _, err := client.GrantPlatformAdmin(ctx, grant); err != nil {
		t.Fatalf("second grant: %v", err)
	}
	revokeOther := connect.NewRequest(&hajjv1.RevokePlatformAdminRequest{UserId: otherUserID})
	auth(revokeOther)
	if _, err := client.RevokePlatformAdmin(ctx, revokeOther); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Ending sessions is the response to a suspected takeover.
	endSessions := connect.NewRequest(&hajjv1.RevokeSessionsRequest{UserId: otherUserID})
	auth(endSessions)
	ended, err := client.RevokeSessions(ctx, endSessions)
	if err != nil {
		t.Fatalf("revoke sessions: %v", err)
	}
	if ended.Msg.SessionsEnded < 1 {
		t.Fatalf("ended %d sessions, want at least the one that existed", ended.Msg.SessionsEnded)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM session WHERE "userId" = $1`, otherUserID).Scan(&remaining); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("%d sessions survived the revocation", remaining)
	}
}

// Reading somebody's identity number is not a neutral act, and the record of
// who looked is the only thing that makes the access reviewable afterwards.
func TestReadingAnIdentityIsAuditedIntegration(t *testing.T) {
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
	var userID, agentID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&userID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, userID); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, userID)
	})
	if err := pool.QueryRow(ctx, `SELECT id::text FROM agents WHERE operator_id = $1 LIMIT 1`, fixture.operatorID).Scan(&agentID); err != nil {
		t.Fatalf("read agent: %v", err)
	}

	// The sealer is installed from main in a real process; a test binary has to
	// do it itself. That it is required at all is the point — without a key,
	// storing an identity is refused rather than done in the clear.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}
	sealer, err := crypto.NewSealer(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	repository.SetKYCSealer(sealer)
	t.Cleanup(func() { repository.SetKYCSealer(nil) })

	queries := db.New(pool)
	kyc := repository.NewKYCRepository(pool)
	if _, err := kyc.Save(ctx, repository.KYCRecord{
		OperatorID: fixture.operatorID, UserID: userID, SubjectType: "AGENT", SubjectID: agentID,
		FullName: "Agen Uji Audit", NIK: "3174012345670009", Source: "SELF",
	}); err != nil {
		t.Fatalf("save kyc: %v", err)
	}

	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool), kyc, repository.NewAuditRepository(queries), repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool), repository.NewPersonalDataReadRepository(pool))
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

	// The list carries no identity numbers at all: one careless screenshot of
	// it would otherwise leak everybody on it.
	listReq := connect.NewRequest(&hajjv1.ListKycRecordsRequest{Limit: 20})
	listReq.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	list, err := client.ListKycRecords(ctx, listReq)
	if err != nil {
		t.Fatalf("list kyc: %v", err)
	}
	if len(list.Msg.Records) == 0 {
		t.Fatal("no identity records listed")
	}

	before := auditCount(t, pool, fixture.operatorID)

	getReq := connect.NewRequest(&hajjv1.GetKycRecordRequest{SubjectType: "AGENT", SubjectId: agentID})
	getReq.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	record, err := client.GetKycRecord(ctx, getReq)
	if err != nil {
		t.Fatalf("get kyc: %v", err)
	}
	if record.Msg.Nik != "3174012345670009" {
		t.Fatalf("identity read back as %q", record.Msg.Nik)
	}

	if after := auditCount(t, pool, fixture.operatorID); after <= before {
		t.Fatal("reading an identity number left no audit entry")
	}
}

func auditCount(t *testing.T, pool *pgxpool.Pool, operatorID string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE action = 'kyc_record_read'`).Scan(&count); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	return count
}
