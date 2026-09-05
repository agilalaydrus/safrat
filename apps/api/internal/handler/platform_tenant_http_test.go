package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The tenant detail page, over HTTP, as the panel will actually call it.
//
// What is being checked is not that the fields are populated — it is that they
// are populated from the right tenant. This page shows one agency's money,
// team and audit trail; a missing WHERE anywhere in it would show somebody
// else's, and the screen would look entirely normal while doing so.
func TestTenantDetailIsScopedToOneTenantIntegration(t *testing.T) {
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

	// A second tenant with its own money and its own audit trail. Nothing of
	// it may appear on the first tenant's page.
	other := uuid.NewString()
	otherSuffix := uuid.NewString()[:8]
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Travel Lain','ID',$3,$4)`, other, "lain-"+otherSuffix, "lain-"+otherSuffix+"@example.test", "lain-"+otherSuffix); err != nil {
		t.Fatalf("fixture travel lain: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, other) })
	if _, err := pool.Exec(ctx, `INSERT INTO audit_logs (operator_id, user_id, action, entity_type, entity_id)
		VALUES ($1,$2,'RAHASIA_TRAVEL_LAIN','order',$3)`, other, userID, uuid.NewString()); err != nil {
		t.Fatalf("fixture audit lain: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO audit_logs (operator_id, user_id, action, entity_type, entity_id)
		VALUES ($1,$2,'MILIK_SENDIRI','order',$3)`, fixture.operatorID, userID, uuid.NewString()); err != nil {
		t.Fatalf("fixture audit sendiri: %v", err)
	}
	// Access ended well past the 90-day grace period, so GetTenantDetail's own
	// copy of D7's deletion_eligible_at computation (repository/platform_tenant.go)
	// has something to report — the same field ListOperators already carries,
	// and the two must not drift into showing a customer two different dates.
	if _, err := pool.Exec(ctx, `INSERT INTO subscriptions (operator_id,plan,status,access_until,grace_period_days)
		VALUES ($1,'GROWTH','CANCELLED', NOW() - INTERVAL '95 days', 0)`, other); err != nil {
		t.Fatalf("fixture subscription lain: %v", err)
	}

	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool),
		repository.NewKYCRepository(pool), repository.NewAuditRepository(queries), repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool), repository.NewPersonalDataReadRepository(pool), nil, repository.NewSupportRepository(queries), repository.NewDataExportRepository(pool), repository.NewAnnouncementRepository(pool))
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
	call := func(operatorID string) (*hajjv1.GetTenantDetailResponse, error) {
		request := connect.NewRequest(&hajjv1.GetTenantDetailRequest{OperatorId: operatorID})
		request.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
		response, err := client.GetTenantDetail(ctx, request)
		if err != nil {
			return nil, err
		}
		return response.Msg, nil
	}

	detail, err := call(fixture.operatorID)
	if err != nil {
		t.Fatalf("get tenant detail: %v", err)
	}
	if detail.Operator.GetId() != fixture.operatorID {
		t.Fatalf("halaman menampilkan travel %s, diminta %s", detail.Operator.GetId(), fixture.operatorID)
	}
	if detail.Counts.GetPilgrims() < 1 || detail.Counts.GetProducts() < 1 || detail.Counts.GetSeasons() < 1 {
		t.Fatalf("hitungan kosong padahal fixture mengisinya: %+v", detail.Counts)
	}
	if detail.Counts.GetStaff() < 1 {
		t.Fatalf("tim kosong: %+v", detail.Counts)
	}
	if len(detail.Team) == 0 {
		t.Fatal("daftar tim kosong padahal ada satu owner")
	}
	if detail.Team[0].GetEmail() == "" {
		t.Fatal("anggota tim tanpa email — join ke tabel Better Auth gagal diam-diam")
	}

	// The audit trail is the sharpest test of scoping: it is the one block
	// where another tenant's row would be readable as plain text.
	sawOwn := false
	for _, entry := range detail.Audit {
		if entry.GetAction() == "RAHASIA_TRAVEL_LAIN" {
			t.Fatal("jejak audit travel lain terbaca di halaman ini")
		}
		if entry.GetAction() == "MILIK_SENDIRI" {
			sawOwn = true
		}
	}
	if !sawOwn {
		t.Fatalf("jejak audit milik sendiri tidak muncul (%d baris)", len(detail.Audit))
	}

	// The other tenant reads back as itself, not as a copy of the first.
	otherDetail, err := call(other)
	if err != nil {
		t.Fatalf("get tenant lain: %v", err)
	}
	if otherDetail.Operator.GetId() != other {
		t.Fatalf("travel lain menampilkan %s", otherDetail.Operator.GetId())
	}
	if otherDetail.Counts.GetPilgrims() != 0 {
		t.Fatalf("travel lain punya %d jamaah padahal tidak diisi", otherDetail.Counts.GetPilgrims())
	}
	if otherDetail.Operator.GetDeletionEligibleAt() == nil {
		t.Fatal("deletion_eligible_at kosong padahal akses travel lain sudah berakhir 95 hari")
	}
	if detail.Operator.GetDeletionEligibleAt() != nil {
		t.Fatalf("travel pertama belum pernah punya langganan tapi deletion_eligible_at terisi: %v", detail.Operator.GetDeletionEligibleAt())
	}
	for _, entry := range otherDetail.Audit {
		if entry.GetAction() == "MILIK_SENDIRI" {
			t.Fatal("jejak audit travel pertama bocor ke halaman travel lain")
		}
	}

	// A tenant id that is not a tenant is a wrong URL, not a server fault.
	if _, err := call(uuid.NewString()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("travel tidak dikenal = %v, mau not_found", connect.CodeOf(err))
	}
	if _, err := call("bukan-uuid"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("id ngawur = %v, mau not_found", connect.CodeOf(err))
	}
}
