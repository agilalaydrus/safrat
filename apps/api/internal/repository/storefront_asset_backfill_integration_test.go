package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStorefrontAssetBackfillIntegration(t *testing.T) {
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	t.Cleanup(pool.Close)

	operatorID := uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1, $2, 'Backfill Test', 'ID', 'backfill@example.com', $3)`,
		operatorID, "backfill-test-"+uuid.NewString(), "backfill-"+operatorID[:8])
	if err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	assets := NewStorefrontAssetRepository(pool)
	legacy := importFor(operatorID, "hero", "legacy.webp", 4096)
	orphanOperator := importFor(uuid.NewString(), "hero", "ghost.webp", 2048)
	oversized := importFor(operatorID, "gallery", "huge.webp", 20*1024*1024)
	batch := []domain.StorefrontAssetImport{legacy, orphanOperator, oversized}

	// A dry run reports exactly what an apply would do, and writes nothing.
	dryRun, err := assets.BackfillLive(ctx, batch, false)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dryRun.Inserted != 1 || dryRun.UnknownOperator != 1 || dryRun.InvalidSize != 1 || dryRun.AlreadyRegistered != 0 {
		t.Fatalf("dry run report = %+v, want 1 inserted / 1 unknown operator / 1 invalid size", dryRun)
	}
	usage, err := assets.Usage(ctx, operatorID)
	if err != nil {
		t.Fatalf("usage after dry run: %v", err)
	}
	if usage.UsedBytes != 0 || usage.AssetCount != 0 {
		t.Fatalf("dry run wrote rows: usage = %+v", usage)
	}

	applied, err := assets.BackfillLive(ctx, batch, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied != dryRun {
		t.Fatalf("apply report = %+v, want the dry run's %+v", applied, dryRun)
	}
	usage, err = assets.Usage(ctx, operatorID)
	if err != nil {
		t.Fatalf("usage after apply: %v", err)
	}
	if usage.UsedBytes != legacy.SizeBytes || usage.AssetCount != 1 || usage.PendingCount != 0 {
		t.Fatalf("usage = %+v, want %d bytes across 1 live asset", usage, legacy.SizeBytes)
	}

	// Re-running must adopt nothing and must not double-count the quota.
	repeat, err := assets.BackfillLive(ctx, batch, true)
	if err != nil {
		t.Fatalf("repeat apply: %v", err)
	}
	if repeat.Inserted != 0 || repeat.AlreadyRegistered != 1 {
		t.Fatalf("repeat report = %+v, want 0 inserted / 1 already registered", repeat)
	}
	usage, err = assets.Usage(ctx, operatorID)
	if err != nil {
		t.Fatalf("usage after repeat: %v", err)
	}
	if usage.UsedBytes != legacy.SizeBytes || usage.AssetCount != 1 {
		t.Fatalf("repeat run changed usage: %+v", usage)
	}

	// An adopted asset behaves like any other: unreferenced means orphaned,
	// and a snapshot reference clears that.
	if _, err := assets.RefreshOrphans(ctx); err != nil {
		t.Fatalf("refresh orphans: %v", err)
	}
	orphans, err := assets.ListOrphans(ctx, nowPlusMinute(), 100)
	if err != nil {
		t.Fatalf("list orphans: %v", err)
	}
	if !containsObjectKey(orphans, legacy.ObjectKey) {
		t.Fatalf("adopted asset is not managed by the cleanup sweep: %+v", orphans)
	}

	storefronts := NewStorefrontRepository(pool)
	draft := []byte(`{"displayName":"Backfill Test","heroImageUrl":"` + legacy.PublicURL + `"}`)
	if _, err := storefronts.SaveDraft(ctx, operatorID, draft, 0); err != nil {
		t.Fatalf("save referenced draft: %v", err)
	}
	if _, err := assets.RefreshOrphans(ctx); err != nil {
		t.Fatalf("refresh referenced asset: %v", err)
	}
	orphans, err = assets.ListOrphans(ctx, nowPlusMinute(), 100)
	if err != nil {
		t.Fatalf("list orphans after reference: %v", err)
	}
	if containsObjectKey(orphans, legacy.ObjectKey) {
		t.Fatalf("referenced adopted asset remained orphaned: %+v", orphans)
	}
}

func importFor(operatorID, kind, name string, sizeBytes int64) domain.StorefrontAssetImport {
	objectKey := "storefront/" + operatorID + "/" + kind + "/" + name
	return domain.StorefrontAssetImport{
		ReservationKey: "storefront-pending/" + operatorID + "/" + kind + "/" + name,
		ObjectKey:      objectKey,
		OperatorID:     operatorID,
		Kind:           kind,
		PublicURL:      "https://assets.example.com/" + objectKey,
		SizeBytes:      sizeBytes,
	}
}

func containsObjectKey(assets []domain.StorefrontAsset, objectKey string) bool {
	for _, asset := range assets {
		if asset.ObjectKey == objectKey {
			return true
		}
	}
	return false
}

func nowPlusMinute() time.Time { return time.Now().Add(time.Minute) }
