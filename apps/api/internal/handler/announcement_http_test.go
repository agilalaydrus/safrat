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

// E2 (TUGAS-PANEL-SAAS.md §10.1 DESAIN), over HTTP: a platform admin sends an
// announcement to two specific tenants; each tenant sees only their own copy
// and reads it independently; the platform's own history shows how many of
// them actually read it; and one tenant can never touch the other's read
// state or see an announcement that was never actually sent to them.
func TestAnnouncementDeliveryIsPerTenantAndReadIsScopedIntegration(t *testing.T) {
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

	admin := newHTTPFixture(t, pool)
	tenantA := newHTTPFixture(t, pool)
	tenantB := newHTTPFixture(t, pool)

	var adminUserID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, admin.sessionToken).Scan(&adminUserID); err != nil {
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
		_, _ = pool.Exec(context.Background(), `DELETE FROM announcements WHERE admin_user_id = $1`, adminUserID)
	})

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
	asAdmin := func(r interface{ Header() http.Header }) { r.Header().Set("Authorization", "Bearer "+admin.sessionToken) }
	asTenantA := func(r interface{ Header() http.Header }) { r.Header().Set("Authorization", "Bearer "+tenantA.sessionToken) }
	asTenantB := func(r interface{ Header() http.Header }) { r.Header().Set("Authorization", "Bearer "+tenantB.sessionToken) }

	filter := &hajjv1.AnnouncementRecipientFilter{Mode: "MANUAL", OperatorIds: []string{tenantA.operatorID, tenantB.operatorID}}

	// Preview: exactly these two, no overlap yet (nothing has ever been sent
	// to them).
	previewReq := connect.NewRequest(&hajjv1.PreviewAnnouncementRecipientsRequest{Filter: filter})
	asAdmin(previewReq)
	preview, err := platformClient.PreviewAnnouncementRecipients(ctx, previewReq)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Msg.GetCount() != 2 {
		t.Fatalf("preview count = %d, mau 2", preview.Msg.GetCount())
	}
	if preview.Msg.GetOverlappingRecentCount() != 0 {
		t.Fatalf("overlap sebelum pernah dikirim = %d, mau 0", preview.Msg.GetOverlappingRecentCount())
	}

	// Send now.
	sendKey := "ann-" + uuid.NewString()
	sendReq := connect.NewRequest(&hajjv1.SendAnnouncementRequest{
		Title: "Pemeliharaan Terjadwal", Body: "Sistem akan tidak dapat diakses Sabtu 02:00-04:00 WIB.",
		Channels: []string{"IN_APP"}, Filter: filter, IdempotencyKey: sendKey,
	})
	asAdmin(sendReq)
	sent, err := platformClient.SendAnnouncement(ctx, sendReq)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	announcementID := sent.Msg.GetAnnouncement().GetId()
	if sent.Msg.GetAnnouncement().GetSentAt() == nil {
		t.Fatal("terkirim sekarang tapi sent_at kosong")
	}
	if sent.Msg.GetAnnouncement().GetRecipientCount() != 2 {
		t.Fatalf("recipient_count = %d, mau 2", sent.Msg.GetAnnouncement().GetRecipientCount())
	}

	// Each tenant sees their own copy, unread.
	listA := connect.NewRequest(&hajjv1.ListMyAnnouncementsRequest{})
	asTenantA(listA)
	inboxA, err := announcementClient.ListMyAnnouncements(ctx, listA)
	if err != nil {
		t.Fatalf("inbox A: %v", err)
	}
	if len(inboxA.Msg.GetAnnouncements()) != 1 || inboxA.Msg.GetAnnouncements()[0].GetId() != announcementID {
		t.Fatalf("inbox A = %+v, mau satu baris untuk %s", inboxA.Msg.GetAnnouncements(), announcementID)
	}
	if inboxA.Msg.GetAnnouncements()[0].GetReadAt() != nil {
		t.Fatal("inbox A sudah menyebut terbaca sebelum ditandai")
	}

	listB := connect.NewRequest(&hajjv1.ListMyAnnouncementsRequest{})
	asTenantB(listB)
	inboxB, err := announcementClient.ListMyAnnouncements(ctx, listB)
	if err != nil {
		t.Fatalf("inbox B: %v", err)
	}
	if len(inboxB.Msg.GetAnnouncements()) != 1 {
		t.Fatalf("inbox B = %+v, mau satu baris", inboxB.Msg.GetAnnouncements())
	}

	// A marks it read — must not affect B's copy.
	markA := connect.NewRequest(&hajjv1.MarkAnnouncementReadRequest{AnnouncementId: announcementID})
	asTenantA(markA)
	if _, err := announcementClient.MarkAnnouncementRead(ctx, markA); err != nil {
		t.Fatalf("mark read A: %v", err)
	}
	req := connect.NewRequest(&hajjv1.ListMyAnnouncementsRequest{})
	asTenantA(req)
	inboxAAfter, err := announcementClient.ListMyAnnouncements(ctx, req)
	if err != nil {
		t.Fatalf("inbox A after read: %v", err)
	}
	if inboxAAfter.Msg.GetAnnouncements()[0].GetReadAt() == nil {
		t.Fatal("A menandai terbaca tapi inbox A sendiri masih menyebut belum")
	}

	reqB := connect.NewRequest(&hajjv1.ListMyAnnouncementsRequest{})
	asTenantB(reqB)
	inboxBAfter, err := announcementClient.ListMyAnnouncements(ctx, reqB)
	if err != nil {
		t.Fatalf("inbox B after A's read: %v", err)
	}
	if inboxBAfter.Msg.GetAnnouncements()[0].GetReadAt() != nil {
		t.Fatal("B ikut tercatat terbaca gara-gara A membacanya")
	}

	// Platform history shows exactly one read.
	historyReq := connect.NewRequest(&hajjv1.ListPlatformAnnouncementsRequest{})
	asAdmin(historyReq)
	history, err := platformClient.ListPlatformAnnouncements(ctx, historyReq)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	var found *hajjv1.Announcement
	for _, a := range history.Msg.GetAnnouncements() {
		if a.GetId() == announcementID {
			found = a
		}
	}
	if found == nil {
		t.Fatal("pengumuman tidak muncul di riwayat platform")
	}
	if found.GetReadCount() != 1 {
		t.Fatalf("read_count riwayat = %d, mau 1", found.GetReadCount())
	}

	// Readiness score's overlap check now sees both recipients.
	previewAgain := connect.NewRequest(&hajjv1.PreviewAnnouncementRecipientsRequest{Filter: filter})
	asAdmin(previewAgain)
	overlapPreview, err := platformClient.PreviewAnnouncementRecipients(ctx, previewAgain)
	if err != nil {
		t.Fatalf("preview again: %v", err)
	}
	if overlapPreview.Msg.GetOverlappingRecentCount() != 2 {
		t.Fatalf("overlap setelah dikirim = %d, mau 2", overlapPreview.Msg.GetOverlappingRecentCount())
	}

	// Idempotent replay does not double the recipient count or the delivery rows.
	replay := connect.NewRequest(&hajjv1.SendAnnouncementRequest{
		Title: "Pemeliharaan Terjadwal", Body: "Sistem akan tidak dapat diakses Sabtu 02:00-04:00 WIB.",
		Channels: []string{"IN_APP"}, Filter: filter, IdempotencyKey: sendKey,
	})
	asAdmin(replay)
	replayed, err := platformClient.SendAnnouncement(ctx, replay)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if replayed.Msg.GetAnnouncement().GetId() != announcementID {
		t.Fatal("pengulangan kunci yang sama membuat pengumuman baru")
	}
	var deliveryRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM announcement_deliveries WHERE announcement_id = $1`, announcementID).Scan(&deliveryRows); err != nil {
		t.Fatal(err)
	}
	if deliveryRows != 2 {
		t.Fatalf("%d baris pengiriman untuk satu pengumuman, mau 2", deliveryRows)
	}

	// A second announcement sent only to A: B must never see it, and must not
	// be able to mark it read either.
	onlyAKey := "ann-" + uuid.NewString()
	onlyAReq := connect.NewRequest(&hajjv1.SendAnnouncementRequest{
		Title: "Khusus A", Body: "Pesan ini hanya untuk satu travel.",
		Channels: []string{"IN_APP"}, Filter: &hajjv1.AnnouncementRecipientFilter{Mode: "MANUAL", OperatorIds: []string{tenantA.operatorID}},
		IdempotencyKey: onlyAKey,
	})
	asAdmin(onlyAReq)
	onlyA, err := platformClient.SendAnnouncement(ctx, onlyAReq)
	if err != nil {
		t.Fatalf("send only-A: %v", err)
	}
	onlyAID := onlyA.Msg.GetAnnouncement().GetId()

	reqB2 := connect.NewRequest(&hajjv1.ListMyAnnouncementsRequest{})
	asTenantB(reqB2)
	inboxBFinal, err := announcementClient.ListMyAnnouncements(ctx, reqB2)
	if err != nil {
		t.Fatalf("inbox B final: %v", err)
	}
	for _, a := range inboxBFinal.Msg.GetAnnouncements() {
		if a.GetId() == onlyAID {
			t.Fatal("pengumuman khusus A terlihat di inbox B")
		}
	}

	markOnlyAByB := connect.NewRequest(&hajjv1.MarkAnnouncementReadRequest{AnnouncementId: onlyAID})
	asTenantB(markOnlyAByB)
	if _, err := announcementClient.MarkAnnouncementRead(ctx, markOnlyAByB); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("B menandai pengumuman yang tidak pernah dikirim ke B = %v, mau not_found", connect.CodeOf(err))
	}
}
