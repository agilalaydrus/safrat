package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newLifecycleFixture(t *testing.T, pool *pgxpool.Pool) (*httpFixture, hajjv1connect.PlatformServiceClient) {
	t.Helper()
	ctx := context.Background()
	fixture := newHTTPFixture(t, pool)
	var adminUserID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&adminUserID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id, note) VALUES ($1, 'uji siklus tenant')`, adminUserID); err != nil {
		t.Fatalf("grant platform admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, adminUserID); err != nil {
		t.Fatalf("enrol 2fa: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, adminUserID)
	})

	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool),
		repository.NewKYCRepository(pool), repository.NewAuditRepository(queries),
		repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool),
		repository.NewPersonalDataReadRepository(pool), nil, repository.NewSupportRepository(queries), repository.NewDataExportRepository(pool), repository.NewAnnouncementRepository(pool))
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
	return fixture, hajjv1connect.NewPlatformServiceClient(server.Client(), server.URL)
}

func authedRequest[T any](fixture *httpFixture, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	return req
}

// TestExtendTrialOnlyExtendsTrialingSubscriptionsIntegration is D2
// (TUGAS-PANEL-SAAS.md): extending only ever adds days, only to a
// subscription still TRIALING, confirmed against the tenant's own name, and
// idempotent under a repeated key.
func TestExtendTrialOnlyExtendsTrialingSubscriptionsIntegration(t *testing.T) {
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

	fixture, client := newLifecycleFixture(t, pool)
	originalAccessUntil := time.Now().Add(3 * 24 * time.Hour).UTC().Truncate(time.Second)
	if _, err := pool.Exec(ctx, `INSERT INTO subscriptions (operator_id, plan, status, access_until) VALUES ($1,'STARTER','TRIALING',$2)`,
		fixture.operatorID, originalAccessUntil); err != nil {
		t.Fatalf("fixture subscription: %v", err)
	}

	var operatorName string
	if err := pool.QueryRow(ctx, `SELECT name FROM operators WHERE id = $1`, fixture.operatorID).Scan(&operatorName); err != nil {
		t.Fatalf("read operator name: %v", err)
	}

	// Wrong confirmation name is refused.
	wrongName := authedRequest(fixture, &hajjv1.ExtendTrialRequest{
		OperatorId: fixture.operatorID, AdditionalDays: 7, Reason: "Masih evaluasi",
		Confirmation: "Nama Yang Salah", IdempotencyKey: "extend-" + uuid.NewString(),
	})
	if _, err := client.ExtendTrial(ctx, wrongName); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("konfirmasi nama salah seharusnya ditolak, dapat %v (%s)", err, connect.CodeOf(err))
	}

	key := "extend-" + uuid.NewString()
	req := authedRequest(fixture, &hajjv1.ExtendTrialRequest{
		OperatorId: fixture.operatorID, AdditionalDays: 7, Reason: "Masih evaluasi, minta tambahan waktu",
		Confirmation: operatorName, IdempotencyKey: key,
	})
	resp, err := client.ExtendTrial(ctx, req)
	if err != nil {
		t.Fatalf("ExtendTrial: %v", err)
	}
	wantAccessUntil := originalAccessUntil.Add(7 * 24 * time.Hour)
	if !resp.Msg.AccessUntil.AsTime().Equal(wantAccessUntil) {
		t.Fatalf("access_until = %v, mau %v", resp.Msg.AccessUntil.AsTime(), wantAccessUntil)
	}

	// Idempotent replay with the same key does not add another 7 days.
	replay := authedRequest(fixture, &hajjv1.ExtendTrialRequest{
		OperatorId: fixture.operatorID, AdditionalDays: 7, Reason: "Masih evaluasi, minta tambahan waktu",
		Confirmation: operatorName, IdempotencyKey: key,
	})
	replayResp, err := client.ExtendTrial(ctx, replay)
	if err != nil {
		t.Fatalf("ExtendTrial replay: %v", err)
	}
	if !replayResp.Msg.AccessUntil.AsTime().Equal(wantAccessUntil) {
		t.Fatalf("replay dengan kunci sama menambah hari lagi: access_until = %v, mau tetap %v", replayResp.Msg.AccessUntil.AsTime(), wantAccessUntil)
	}

	// Move the subscription to ACTIVE and confirm a fresh extend attempt is
	// refused — this is not how a paid tenant gets more time.
	if _, err := pool.Exec(ctx, `UPDATE subscriptions SET status = 'ACTIVE' WHERE operator_id = $1`, fixture.operatorID); err != nil {
		t.Fatalf("move to active: %v", err)
	}
	activeReq := authedRequest(fixture, &hajjv1.ExtendTrialRequest{
		OperatorId: fixture.operatorID, AdditionalDays: 7, Reason: "Coba lagi setelah aktif",
		Confirmation: operatorName, IdempotencyKey: "extend-" + uuid.NewString(),
	})
	if _, err := client.ExtendTrial(ctx, activeReq); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("memperpanjang trial pada langganan ACTIVE seharusnya ditolak, dapat %v (%s)", err, connect.CodeOf(err))
	}
}

// TestCancelSubscriptionLeavesAccessUntilUntouchedIntegration is D5: cancelling
// sets cancelled_at and nothing else — access_until must survive byte for
// byte — and a second cancellation is refused rather than silently repeated.
func TestCancelSubscriptionLeavesAccessUntilUntouchedIntegration(t *testing.T) {
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

	fixture, client := newLifecycleFixture(t, pool)
	accessUntil := time.Now().Add(18 * 24 * time.Hour).UTC().Truncate(time.Second)
	if _, err := pool.Exec(ctx, `INSERT INTO subscriptions (operator_id, plan, status, access_until) VALUES ($1,'GROWTH','ACTIVE',$2)`,
		fixture.operatorID, accessUntil); err != nil {
		t.Fatalf("fixture subscription: %v", err)
	}
	var operatorName string
	if err := pool.QueryRow(ctx, `SELECT name FROM operators WHERE id = $1`, fixture.operatorID).Scan(&operatorName); err != nil {
		t.Fatalf("read operator name: %v", err)
	}

	req := authedRequest(fixture, &hajjv1.CancelSubscriptionRequest{
		OperatorId: fixture.operatorID, Reason: "Pindah ke penyedia lain",
		Confirmation: operatorName, IdempotencyKey: "cancel-" + uuid.NewString(),
	})
	resp, err := client.CancelSubscription(ctx, req)
	if err != nil {
		t.Fatalf("CancelSubscription: %v", err)
	}
	if !resp.Msg.AccessUntil.AsTime().Equal(accessUntil) {
		t.Fatalf("access_until berubah oleh pembatalan: dapat %v, mau tetap %v", resp.Msg.AccessUntil.AsTime(), accessUntil)
	}
	if resp.Msg.CancelledAt == nil {
		t.Fatal("cancelled_at tidak terisi")
	}

	// A second, genuinely new cancellation attempt (different idempotency
	// key) on an already-cancelled subscription must be refused, not
	// silently accepted as a no-op — a second admin doing this by mistake
	// deserves to know nothing new happened.
	again := authedRequest(fixture, &hajjv1.CancelSubscriptionRequest{
		OperatorId: fixture.operatorID, Reason: "Coba batalkan lagi",
		Confirmation: operatorName, IdempotencyKey: "cancel-" + uuid.NewString(),
	})
	if _, err := client.CancelSubscription(ctx, again); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("membatalkan langganan yang sudah dibatalkan seharusnya ditolak, dapat %v (%s)", err, connect.CodeOf(err))
	}
}
