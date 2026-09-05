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

// Deleting a tenant is the one action in this system that cannot be undone.
// The things worth proving: none of the three preconditions (90 days since
// access lapsed, a READY export on file, the tenant's own name typed back)
// can be skipped, and once all three are met the tenant's rows are actually
// gone while the audit trail proving it happened survives them.
func TestDeleteTenantRequiresGraceExportAndNameIntegration(t *testing.T) {
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

	customer, suffix := uuid.NewString(), uuid.NewString()[:8]
	customerOrg := "del-org-" + uuid.NewString()
	customerName := "Travel Dihapus " + suffix
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO organization (id, name, slug, "createdAt") VALUES ($1,'Del',$2,NOW())`, customerOrg, "del-"+suffix)
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan)
	      VALUES ($1,$2,$3,'ID',$4,$5,'GROWTH')`, customer, customerOrg, customerName, "del-"+suffix+"@example.test", "del-"+suffix)
	// Access ended just now, not 90 days ago, and no grace period — the clock
	// has barely started, so nothing should be deletable yet.
	exec(`INSERT INTO subscriptions (operator_id,plan,status,access_until,grace_period_days)
	      VALUES ($1,'GROWTH','CANCELLED', NOW() - INTERVAL '1 day', 0)`, customer)
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
		repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool),
		repository.NewPersonalDataReadRepository(pool), nil, repository.NewSupportRepository(queries),
		repository.NewDataExportRepository(pool))
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

	attempt := func(confirmation, key string) error {
		req := connect.NewRequest(&hajjv1.DeleteTenantRequest{
			OperatorId: customer, Reason: "keputusan pemilik untuk menghapus tenant ini",
			Confirmation: confirmation, IdempotencyKey: key,
		})
		auth(req)
		_, err := client.DeleteTenant(ctx, req)
		return err
	}

	// Too soon: the 90-day grace period since access ended hasn't passed.
	// There is also no export yet, so isolating this gate requires ruling
	// that reason out too — a request that would be refused twice over
	// still has to be refused, but this assertion only proves the 90-day
	// gate if the export gate can't be the one doing the refusing.
	if err := attempt(customerName, "del-"+uuid.NewString()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("baru berakhir 1 hari lalu = %v, mau failed_precondition", connect.CodeOf(err))
	}
	var stillExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operators WHERE id = $1)`, customer).Scan(&stillExists); err != nil {
		t.Fatal(err)
	}
	if !stillExists {
		t.Fatal("tenant terhapus walau baru berakhir 1 hari lalu")
	}

	// Offer the export through the platform-triggered RPC (D6's precondition
	// this RPC exists to satisfy), then mark it READY the way the worker
	// eventually would — this test asserts the gate, not the worker pipeline.
	exportReq := connect.NewRequest(&hajjv1.RequestTenantDataExportRequest{
		OperatorId: customer, IdempotencyKey: "export-" + uuid.NewString(),
	})
	auth(exportReq)
	exportRow, err := client.RequestTenantDataExport(ctx, exportReq)
	if err != nil {
		t.Fatalf("request export: %v", err)
	}
	if exportRow.Msg.GetStatus() == "" {
		t.Fatal("respons ekspor tidak menyebut status")
	}

	// Export is ready now, but access still only ended 1 day ago: this
	// isolates the 90-day gate — nothing else is left that could be refusing.
	exec(`UPDATE operator_data_exports SET status = 'READY', object_key = 'fake/key.zip'
	      WHERE id = $1`, exportRow.Msg.GetId())
	if err := attempt(customerName, "del-"+uuid.NewString()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("baru berakhir 1 hari lalu (ekspor sudah siap) = %v, mau failed_precondition", connect.CodeOf(err))
	}
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operators WHERE id = $1)`, customer).Scan(&stillExists); err != nil {
		t.Fatal(err)
	}
	if !stillExists {
		t.Fatal("tenant terhapus walau baru berakhir 1 hari lalu, meski ekspor sudah siap")
	}

	// Un-ready the export and move the clock forward: 91 days past access
	// ending, well past the grace period, but no export on file — this
	// isolates the export gate the same way.
	exec(`UPDATE operator_data_exports SET status = 'PENDING' WHERE id = $1`, exportRow.Msg.GetId())
	exec(`UPDATE subscriptions SET access_until = NOW() - INTERVAL '91 days' WHERE operator_id = $1`, customer)
	if err := attempt(customerName, "del-"+uuid.NewString()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("belum ada ekspor siap = %v, mau failed_precondition", connect.CodeOf(err))
	}
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operators WHERE id = $1)`, customer).Scan(&stillExists); err != nil {
		t.Fatal(err)
	}
	if !stillExists {
		t.Fatal("tenant terhapus walau ekspor belum siap")
	}

	// Make the export READY again: now both gates are satisfied.
	exec(`UPDATE operator_data_exports SET status = 'READY' WHERE id = $1`, exportRow.Msg.GetId())

	// Wrong name still refuses, even with grace period passed and export ready.
	if err := attempt("Travel Yang Lain", "del-"+uuid.NewString()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("nama salah = %v, mau invalid_argument", connect.CodeOf(err))
	}
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operators WHERE id = $1)`, customer).Scan(&stillExists); err != nil {
		t.Fatal(err)
	}
	if !stillExists {
		t.Fatal("tenant terhapus walau konfirmasi namanya salah")
	}

	// All three preconditions now met: this deletes for real.
	deleteKey := "del-" + uuid.NewString()
	deleted, err := func() (*connect.Response[hajjv1.DeleteTenantResponse], error) {
		req := connect.NewRequest(&hajjv1.DeleteTenantRequest{
			OperatorId: customer, Reason: "keputusan pemilik untuk menghapus tenant ini",
			Confirmation: "  " + customerName + " ", IdempotencyKey: deleteKey,
		})
		auth(req)
		return client.DeleteTenant(ctx, req)
	}()
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted.Msg.GetOperatorName() != customerName {
		t.Fatalf("nama di respons = %q, mau %q", deleted.Msg.GetOperatorName(), customerName)
	}
	if deleted.Msg.GetDeletedAt() == nil {
		t.Fatal("respons tidak menyebut kapan dihapus")
	}

	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operators WHERE id = $1)`, customer).Scan(&stillExists); err != nil {
		t.Fatal(err)
	}
	if stillExists {
		t.Fatal("tenant masih ada setelah dihapus")
	}

	// The audit trail outlives the tenant it was about: audit_logs rows for
	// this operator must still be there, with operator_id gone to NULL.
	var survivingAuditRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs
		WHERE operator_id IS NULL AND action = 'tenant_deleted' AND metadata->>'operator_name' = $1`,
		customerName).Scan(&survivingAuditRows); err != nil {
		t.Fatal(err)
	}
	if survivingAuditRows != 1 {
		t.Fatalf("%d entri audit penghapusan bertahan, mau 1", survivingAuditRows)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE action = 'tenant_deleted' AND metadata->>'operator_name' = $1`, customerName)
	})

	// A retry with the same key settles the same action rather than erroring
	// on a tenant that (from the caller's point of view) it already deleted.
	retry, err := func() (*connect.Response[hajjv1.DeleteTenantResponse], error) {
		req := connect.NewRequest(&hajjv1.DeleteTenantRequest{
			OperatorId: customer, Reason: "keputusan pemilik untuk menghapus tenant ini",
			Confirmation: customerName, IdempotencyKey: deleteKey,
		})
		auth(req)
		return client.DeleteTenant(ctx, req)
	}()
	if err != nil {
		t.Fatalf("pengulangan ditolak: %v", err)
	}
	if retry.Msg.GetOperatorName() != customerName {
		t.Fatalf("pengulangan mengembalikan nama berbeda: %q", retry.Msg.GetOperatorName())
	}
	var deleteRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM privileged_actions
		WHERE kind = 'DELETE_TENANT' AND payload->>'operator_id' = $1`, customer).Scan(&deleteRows); err != nil {
		t.Fatal(err)
	}
	if deleteRows != 1 {
		t.Fatalf("%d baris DELETE_TENANT untuk satu tindakan", deleteRows)
	}
}
