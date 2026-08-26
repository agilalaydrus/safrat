package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStorefrontAssetQuotaAndReferenceAwareCleanupIntegration(t *testing.T) {
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
	_, err = pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1, $2, 'Asset Test', 'ID', 'asset@example.com', $3)`, operatorID, "asset-test-"+uuid.NewString(), "asset-"+operatorID[:8])
	if err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	assets := NewStorefrontAssetRepository(pool)
	storefronts := NewStorefrontRepository(pool)
	expiresAt := time.Now().Add(10 * time.Minute)
	reservationKey := "storefront-pending/" + operatorID + "/hero/reservation.webp"
	objectKey := "storefront/" + operatorID + "/hero/reservation.webp"
	publicURL := "https://assets.example.com/" + objectKey
	if err := assets.Reserve(ctx, operatorID, reservationKey, "hero", 600, expiresAt, 1000); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := assets.Confirm(ctx, operatorID, reservationKey, objectKey, publicURL, 600); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := assets.Reserve(ctx, operatorID, reservationKey+"-second", "hero", 500, expiresAt, 1000); !errors.Is(err, ErrStorefrontStorageQuota) {
		t.Fatalf("second reserve error = %v, want quota error", err)
	}
	usage, err := assets.Usage(ctx, operatorID)
	if err != nil || usage.UsedBytes != 600 || usage.AssetCount != 1 || usage.PendingCount != 0 {
		t.Fatalf("usage = %+v, error %v", usage, err)
	}

	if _, err := assets.RefreshOrphans(ctx); err != nil {
		t.Fatalf("mark orphan: %v", err)
	}
	// Scoped to this test's own asset. ListOrphans is global, so asserting a
	// total count silently depends on the database being otherwise empty —
	// which stopped being true once the browser suite started creating real
	// uploads against the same database.
	orphans, err := assets.ListOrphans(ctx, time.Now().Add(time.Minute), 500)
	if err != nil || !containsObjectKey(orphans, objectKey) {
		t.Fatalf("unreferenced asset was not marked orphaned: %+v, error %v", orphans, err)
	}
	draft := []byte(`{"displayName":"Asset Test","heroImageUrl":"` + publicURL + `"}`)
	if _, err := storefronts.SaveDraft(ctx, operatorID, draft, 0); err != nil {
		t.Fatalf("save referenced draft: %v", err)
	}
	if _, err := assets.RefreshOrphans(ctx); err != nil {
		t.Fatalf("refresh referenced asset: %v", err)
	}
	orphans, err = assets.ListOrphans(ctx, time.Now().Add(time.Minute), 500)
	if err != nil || containsObjectKey(orphans, objectKey) {
		t.Fatalf("referenced asset remained orphaned: %+v, error %v", orphans, err)
	}
}
