package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// F4 (TUGAS-PANEL-SAAS.md): every new platform RPC tested both ways — no
// session, a real operator owner (senior in their own organisation and
// still not a platform admin), and a granted platform admin. E2's three new
// PlatformService RPCs all route through the same requirePlatformAdmin
// helper as every other one, but a copy-paste bug in a new handler could
// still skip that call; this proves each of the three actually enforces it,
// not just that the shared helper works somewhere else. The operator-facing
// pair only needs a valid session — never platform admin — so they get the
// opposite assertion: an ordinary operator owner succeeds.
func TestAnnouncementRPCsEnforceTheRightIdentityIntegration(t *testing.T) {
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

	queries := db.New(pool)
	announcementRepository := repository.NewAnnouncementRepository(pool)
	platformService := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool),
		repository.NewKYCRepository(pool), repository.NewAuditRepository(queries),
		repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool),
		repository.NewPersonalDataReadRepository(pool), nil, repository.NewSupportRepository(queries),
		repository.NewDataExportRepository(pool), announcementRepository)
	announcementService := service.NewAnnouncementService(repository.NewOperatorRepository(queries), announcementRepository)

	mux := http.NewServeMux()
	platformPath, platformServiceHandler := hajjv1connect.NewPlatformServiceHandler(handler.NewPlatformHandler(platformService),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool,
			repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)),
			repository.NewSubscriptionRepository(pool))))
	announcementPath, announcementServiceHandler := hajjv1connect.NewAnnouncementServiceHandler(handler.NewAnnouncementHandler(announcementService),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool,
			repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)),
			repository.NewSubscriptionRepository(pool))))
	mux.Handle(platformPath, platformServiceHandler)
	mux.Handle(announcementPath, announcementServiceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	platformClient := hajjv1connect.NewPlatformServiceClient(server.Client(), server.URL)
	announcementClient := hajjv1connect.NewAnnouncementServiceClient(server.Client(), server.URL)

	withToken := func(token string) http.Header {
		h := http.Header{}
		if token != "" {
			h.Set("Authorization", "Bearer "+token)
		}
		return h
	}

	previewReq := func(token string) *connect.Request[hajjv1.PreviewAnnouncementRecipientsRequest] {
		r := connect.NewRequest(&hajjv1.PreviewAnnouncementRecipientsRequest{Filter: &hajjv1.AnnouncementRecipientFilter{Mode: "ALL"}})
		r.Header().Set("Authorization", withToken(token).Get("Authorization"))
		return r
	}
	sendReq := func(token string) *connect.Request[hajjv1.SendAnnouncementRequest] {
		r := connect.NewRequest(&hajjv1.SendAnnouncementRequest{
			Title: "uji akses", Body: "uji akses dua arah",
			Filter: &hajjv1.AnnouncementRecipientFilter{Mode: "MANUAL", OperatorIds: []string{fixture.operatorID}},
			IdempotencyKey: "access-test-key",
		})
		if token != "" {
			r.Header().Set("Authorization", "Bearer "+token)
		}
		return r
	}
	historyReq := func(token string) *connect.Request[hajjv1.ListPlatformAnnouncementsRequest] {
		r := connect.NewRequest(&hajjv1.ListPlatformAnnouncementsRequest{})
		if token != "" {
			r.Header().Set("Authorization", "Bearer "+token)
		}
		return r
	}

	// No session at all: unauthenticated on all three.
	if _, err := platformClient.PreviewAnnouncementRecipients(ctx, previewReq("")); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("Preview tanpa sesi = %v, mau unauthenticated", connect.CodeOf(err))
	}
	if _, err := platformClient.SendAnnouncement(ctx, sendReq("")); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("Send tanpa sesi = %v, mau unauthenticated", connect.CodeOf(err))
	}
	if _, err := platformClient.ListPlatformAnnouncements(ctx, historyReq("")); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("History tanpa sesi = %v, mau unauthenticated", connect.CodeOf(err))
	}

	// A real operator owner, not a platform admin: permission_denied on all three.
	if _, err := platformClient.PreviewAnnouncementRecipients(ctx, previewReq(fixture.sessionToken)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("Preview oleh pemilik travel = %v, mau permission_denied", connect.CodeOf(err))
	}
	if _, err := platformClient.SendAnnouncement(ctx, sendReq(fixture.sessionToken)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("Send oleh pemilik travel = %v, mau permission_denied", connect.CodeOf(err))
	}
	if _, err := platformClient.ListPlatformAnnouncements(ctx, historyReq(fixture.sessionToken)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("History oleh pemilik travel = %v, mau permission_denied", connect.CodeOf(err))
	}

	// Grant, and satisfy the 2FA requirement platform access also demands.
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id, note) VALUES ($1, 'uji akses pengumuman')`, userID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, userID); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM announcements WHERE admin_user_id = $1`, userID)
	})

	if _, err := platformClient.PreviewAnnouncementRecipients(ctx, previewReq(fixture.sessionToken)); err != nil {
		t.Fatalf("Preview oleh admin platform ditolak: %v", err)
	}
	if _, err := platformClient.SendAnnouncement(ctx, sendReq(fixture.sessionToken)); err != nil {
		t.Fatalf("Send oleh admin platform ditolak: %v", err)
	}
	if _, err := platformClient.ListPlatformAnnouncements(ctx, historyReq(fixture.sessionToken)); err != nil {
		t.Fatalf("History oleh admin platform ditolak: %v", err)
	}

	// Revoking bites on the very next request.
	if _, err := pool.Exec(ctx, `DELETE FROM platform_admins WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := platformClient.PreviewAnnouncementRecipients(ctx, previewReq(fixture.sessionToken)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("admin yang dicabut masih bisa Preview (%v)", connect.CodeOf(err))
	}

	// The operator-facing pair: a plain operator session (never a platform
	// admin) must succeed — this is the opposite assertion from above,
	// because these two RPCs are for every tenant, not just platform staff.
	listMineReq := connect.NewRequest(&hajjv1.ListMyAnnouncementsRequest{})
	listMineReq.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	if _, err := announcementClient.ListMyAnnouncements(ctx, listMineReq); err != nil {
		t.Fatalf("ListMyAnnouncements ditolak untuk pemilik travel biasa: %v", err)
	}
	noSessionReq := connect.NewRequest(&hajjv1.ListMyAnnouncementsRequest{})
	if _, err := announcementClient.ListMyAnnouncements(ctx, noSessionReq); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("ListMyAnnouncements tanpa sesi = %v, mau unauthenticated", connect.CodeOf(err))
	}
}
