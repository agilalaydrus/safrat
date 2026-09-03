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

// Suspending a travel agency, and letting them back in.
//
// The three things worth proving: the typed name is really checked against the
// real one, the lock-out actually takes effect while the time they paid for is
// left alone, and the decision leaves a row that outlives it.
func TestSuspendTenantRequiresTheNameAndLeavesEvidenceIntegration(t *testing.T) {
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
	var adminUserID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&adminUserID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM platform_admins`); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id) VALUES ($1)`, adminUserID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, adminUserID); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins`) })
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM privileged_actions WHERE requested_by = $1`, adminUserID)
	})

	// A tenant who has paid: access runs a month into the future, so a
	// suspension has to be what locks them out, not an expired subscription.
	customer, suffix := uuid.NewString(), uuid.NewString()[:8]
	customerOrg := "susp-org-" + uuid.NewString()
	customerName := "Travel Ditangguhkan " + suffix
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO organization (id, name, slug, "createdAt") VALUES ($1,'Susp',$2,NOW())`, customerOrg, "susp-"+suffix)
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan)
	      VALUES ($1,$2,$3,'ID',$4,$5,'GROWTH')`, customer, customerOrg, customerName, "susp-"+suffix+"@example.test", "susp-"+suffix)
	exec(`INSERT INTO subscriptions (operator_id,plan,status,access_until)
	      VALUES ($1,'GROWTH','ACTIVE', NOW() + INTERVAL '30 days')`, customer)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id = $1`, customer)
		_, _ = pool.Exec(bg, `DELETE FROM organization WHERE id = $1`, customerOrg)
	})

	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool),
		repository.NewKYCRepository(pool), repository.NewAuditRepository(queries),
		repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool))
	path, serviceHandler := hajjv1connect.NewPlatformServiceHandler(handler.NewPlatformHandler(platform),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool,
			repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)),
			repository.NewSubscriptionRepository(pool))))
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := hajjv1connect.NewPlatformServiceClient(server.Client(), server.URL)
	auth := func(r interface{ Header() http.Header }) {
		r.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	}

	subscriptions := repository.NewSubscriptionRepository(pool)
	access, err := subscriptions.GetAccessByOrgID(ctx, customerOrg)
	if err != nil {
		t.Fatalf("access before: %v", err)
	}
	if !access.Allowed {
		t.Fatal("fixture belum punya akses sebelum ditangguhkan — ujinya tidak akan membuktikan apa pun")
	}

	// The wrong name must not suspend anybody.
	wrong := connect.NewRequest(&hajjv1.SuspendTenantRequest{
		OperatorId: customer, Reason: "diminta menghentikan sementara oleh pemilik",
		Confirmation: "Travel Yang Lain", IdempotencyKey: "susp-" + uuid.NewString(),
	})
	auth(wrong)
	if _, err := client.SuspendTenant(ctx, wrong); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("nama salah = %v, mau invalid_argument", connect.CodeOf(err))
	}
	stillAllowed, err := subscriptions.GetAccessByOrgID(ctx, customerOrg)
	if err != nil {
		t.Fatal(err)
	}
	if !stillAllowed.Allowed {
		t.Fatal("travel tertangguhkan walau konfirmasinya ditolak")
	}

	// The right name, in different case and with stray spaces: the words are
	// what matter, not the typing.
	suspendKey := "susp-" + uuid.NewString()
	right := connect.NewRequest(&hajjv1.SuspendTenantRequest{
		OperatorId: customer, Reason: "permintaan resmi pemilik travel lewat surat",
		Confirmation: "  " + customerName + " ", IdempotencyKey: suspendKey,
	})
	auth(right)
	suspended, err := client.SuspendTenant(ctx, right)
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if suspended.Msg.GetSuspendedAt() == nil {
		t.Fatal("respons tidak menyebut kapan ditangguhkan")
	}
	if suspended.Msg.GetAccessUntil() == nil {
		t.Fatal("access_until hilang dari respons — waktu yang sudah dibayar harus tetap terlihat")
	}

	locked, err := subscriptions.GetAccessByOrgID(ctx, customerOrg)
	if err != nil {
		t.Fatal(err)
	}
	if locked.Allowed {
		t.Fatal("travel masih punya akses setelah ditangguhkan")
	}
	// The paid time is untouched, which is what makes reinstating exact.
	if !locked.AccessUntil.Equal(access.AccessUntil) {
		t.Fatalf("access_until berubah saat penangguhan: %v → %v", access.AccessUntil, locked.AccessUntil)
	}

	// A retry with the same key settles the same action rather than performing
	// a second one.
	retry := connect.NewRequest(&hajjv1.SuspendTenantRequest{
		OperatorId: customer, Reason: "permintaan resmi pemilik travel lewat surat",
		Confirmation: customerName, IdempotencyKey: suspendKey,
	})
	auth(retry)
	if _, err := client.SuspendTenant(ctx, retry); err != nil {
		t.Fatalf("pengulangan ditolak: %v", err)
	}
	var suspendRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM privileged_actions
		WHERE kind = 'SUSPEND' AND payload->>'operator_id' = $1`, customer).Scan(&suspendRows); err != nil {
		t.Fatal(err)
	}
	if suspendRows != 1 {
		t.Fatalf("%d baris SUSPEND untuk satu tindakan", suspendRows)
	}

	// Reinstating returns exactly the time they bought.
	back := connect.NewRequest(&hajjv1.ReinstateTenantRequest{
		OperatorId: customer, Reason: "kesalahpahaman selesai, dibuka kembali",
		Confirmation: customerName, IdempotencyKey: "rein-" + uuid.NewString(),
	})
	auth(back)
	reinstated, err := client.ReinstateTenant(ctx, back)
	if err != nil {
		t.Fatalf("reinstate: %v", err)
	}
	restored, err := subscriptions.GetAccessByOrgID(ctx, customerOrg)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Allowed {
		t.Fatal("travel tidak pulih setelah dibuka kembali")
	}
	if !restored.AccessUntil.Equal(access.AccessUntil) {
		t.Fatalf("waktu yang dibayar tidak kembali utuh: %v → %v", access.AccessUntil, restored.AccessUntil)
	}
	if reinstated.Msg.GetAccessUntil() == nil {
		t.Fatal("respons pemulihan tidak menyebut sampai kapan aksesnya")
	}

	// The evidence survives both decisions, and says how many people could have
	// objected at the time.
	listReq := connect.NewRequest(&hajjv1.ListPrivilegedActionsRequest{OperatorId: customer})
	auth(listReq)
	actions, err := client.ListPrivilegedActions(ctx, listReq)
	if err != nil {
		t.Fatalf("list privileged actions: %v", err)
	}
	kinds := map[string]bool{}
	for _, action := range actions.Msg.GetActions() {
		kinds[action.GetKind()] = true
		if action.GetReason() == "" {
			t.Fatal("tindakan tercatat tanpa alasan")
		}
		if action.GetAdminCountAtRequest() < 1 {
			t.Fatalf("jumlah admin tidak masuk akal: %d", action.GetAdminCountAtRequest())
		}
		if action.GetRequestedBy() == "" || action.GetApprovedBy() == "" {
			t.Fatal("tindakan tanpa peminta atau penyetuju")
		}
	}
	if !kinds["SUSPEND"] || !kinds["REINSTATE"] {
		t.Fatalf("bukti tidak lengkap: %v", kinds)
	}
	var audited int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs
		WHERE operator_id = $1 AND action IN ('tenant_suspended','tenant_reinstated')`, customer).Scan(&audited); err != nil {
		t.Fatal(err)
	}
	if audited != 2 {
		t.Fatalf("%d entri audit, mau 2", audited)
	}
}
