package worker

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Muting a cascade's notification must never mute the cascade itself — an
// operator who turns off "beri tahu jamaah saat kota berubah" still needs
// their journey status actually updated, just silently. This proves both
// halves at once: with the toggle off, BulkUpdateForGroupAs still runs
// (state changes) but NotifyGroupPilgrims is never called (no push).
func TestNotificationToggleMutesPushNotTheStateChangeIntegration(t *testing.T) {
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

	operatorID := uuid.NewString()
	if _, err := pool.Exec(ctx,
		`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Notif Uji','ID',$3,$4)`,
		operatorID, "notif-"+operatorID, operatorID[:8]+"@example.test", "notif-"+operatorID[:8]); err != nil {
		t.Fatalf("fixture operator: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM operator_notification_settings WHERE operator_id = $1`, operatorID)
		_, _ = pool.Exec(ctx, `DELETE FROM operators WHERE id = $1`, operatorID)
	})

	queries := db.New(pool)
	settingsRepo := repository.NewNotificationSettingsRepository(queries)
	if _, err := settingsRepo.Set(ctx, domain.NotificationSettings{
		OperatorID: operatorID, NotifyGroupCityChange: false, NotifyKloterStatusChange: true, NotifyRitualBulkComplete: true,
	}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	push := &fakeCascadePusher{}
	journeys := &fakeJourneyCascader{}
	handler := &OutboxHandler{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)), push: push, journeys: journeys, notificationSettings: settingsRepo,
	}

	payload, _ := json.Marshal(domain.GroupCityUpdatedPayload{
		GroupID: "group-1", JourneyStatus: "IN_MAKKAH", UpdatedBy: "user-1", Notes: "arrived", NotificationBody: "Grup Anda kini di Makkah",
	})
	if err := handler.dispatch(ctx, domain.CascadeEvent{OperatorID: operatorID, EventType: domain.EventGroupCityUpdated, Payload: payload}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if journeys.groupID != "group-1" || journeys.status != "IN_MAKKAH" {
		t.Fatalf("status jamaah tidak ikut diperbarui walau notifikasi dimatikan: %+v", journeys)
	}
	if push.groupID != "" || push.body != "" {
		t.Fatalf("push seharusnya tidak terkirim saat notify_group_city_change=false, tapi terkirim: %+v", push)
	}
}
