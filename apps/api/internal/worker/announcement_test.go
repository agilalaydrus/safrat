package worker

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A "terjadwal" announcement (§10.1 DESAIN) must not go out before its
// scheduled moment, and must actually go out once the sweep runs past it —
// re-resolving recipients fresh rather than trusting whatever the preview
// said when it was composed.
func TestAnnouncementDispatchSweepRespectsScheduleIntegration(t *testing.T) {
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

	adminUserID := "sweep-admin-" + uuid.NewString()
	operatorID, orgID, suffix := uuid.NewString(), uuid.NewString(), uuid.NewString()[:8]
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO organization (id, name, slug, "createdAt") VALUES ($1,'Sweep',$2,NOW())`, orgID, "sweep-"+suffix)
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
	      VALUES ($1,$2,'Travel Terjadwal','ID',$3,$4)`, operatorID, orgID, "sweep-"+suffix+"@example.test", "sweep-"+suffix)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id = $1`, operatorID)
		_, _ = pool.Exec(bg, `DELETE FROM organization WHERE id = $1`, orgID)
	})

	announcements := repository.NewAnnouncementRepository(pool)
	future := repository.RecipientFilter{Mode: "MANUAL", OperatorIDs: []string{operatorID}}
	announcement, err := announcements.Create(ctx, adminUserID, "Belum waktunya", "Isi pesan.", "",
		[]string{"IN_APP"}, future, time.Now().Add(1*time.Hour), "sweep-key-"+uuid.NewString())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM announcements WHERE id = $1`, announcement.ID) })

	handler := NewAnnouncementHandler(slog.Default(), announcements)

	// Not due yet: the sweep must leave it alone.
	if err := handler.HandleDispatch(ctx, nil); err != nil {
		t.Fatalf("sweep before due: %v", err)
	}
	untouched, err := announcements.Get(ctx, announcement.ID)
	if err != nil {
		t.Fatal(err)
	}
	if untouched.SentAt != nil {
		t.Fatal("terkirim sebelum jadwalnya")
	}

	// Move the schedule into the past and sweep again.
	exec(`UPDATE announcements SET scheduled_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, announcement.ID)
	if err := handler.HandleDispatch(ctx, nil); err != nil {
		t.Fatalf("sweep at due: %v", err)
	}
	sent, err := announcements.Get(ctx, announcement.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sent.SentAt == nil {
		t.Fatal("sweep tidak mengirim pengumuman yang sudah lewat jadwalnya")
	}
	if sent.RecipientCount != 1 {
		t.Fatalf("recipient_count = %d, mau 1", sent.RecipientCount)
	}
	var deliveryRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM announcement_deliveries WHERE announcement_id = $1`, announcement.ID).Scan(&deliveryRows); err != nil {
		t.Fatal(err)
	}
	if deliveryRows != 1 {
		t.Fatalf("%d baris pengiriman, mau 1", deliveryRows)
	}

	// Sweeping again must not re-send: DueForDispatch only ever returns
	// unsent rows, so a second pass finds nothing left to do.
	if err := handler.HandleDispatch(ctx, nil); err != nil {
		t.Fatalf("sweep again: %v", err)
	}
	var deliveryRowsAfter int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM announcement_deliveries WHERE announcement_id = $1`, announcement.ID).Scan(&deliveryRowsAfter); err != nil {
		t.Fatal(err)
	}
	if deliveryRowsAfter != 1 {
		t.Fatalf("sweep kedua menggandakan pengiriman: %d baris", deliveryRowsAfter)
	}
}
