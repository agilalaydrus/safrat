package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStorefrontDraftPublishIntegration(t *testing.T) {
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
	orgID := "storefront-test-" + uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1, $2, 'Travel Test', 'ID', 'test@example.com', $3)`, operatorID, orgID, "test-"+operatorID[:8])
	if err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	repository := NewStorefrontRepository(pool)
	draftOne := []byte(`{"displayName":"Draft Satu","brandColor":"#059669"}`)
	snapshot, err := repository.SaveDraft(ctx, operatorID, draftOne, 0)
	if err != nil {
		t.Fatalf("save first draft: %v", err)
	}
	if snapshot.DraftRevision != 1 || snapshot.PublishedRevision != 0 {
		t.Fatalf("first revisions = draft %d live %d", snapshot.DraftRevision, snapshot.PublishedRevision)
	}
	if _, _, err := repository.GetPublished(ctx, operatorID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("draft leaked to public snapshot: %v", err)
	}

	if _, err := repository.SaveDraft(ctx, operatorID, []byte(`{"displayName":"Stale"}`), 0); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("stale save error = %v, want conflict", err)
	}

	published, err := repository.Publish(ctx, operatorID, 1)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.PublishedRevision != 1 || published.PublishedAt == nil {
		t.Fatalf("published revision/time not updated: %+v", published)
	}
	publicJSON, revision, err := repository.GetPublished(ctx, operatorID)
	if err != nil || revision != 1 || string(publicJSON) == "" {
		t.Fatalf("get published = revision %d json %q error %v", revision, publicJSON, err)
	}

	draftTwo := []byte(`{"displayName":"Draft Dua","brandColor":"#0f766e"}`)
	second, err := repository.SaveDraft(ctx, operatorID, draftTwo, 1)
	if err != nil {
		t.Fatalf("save second draft: %v", err)
	}
	if second.DraftRevision != 2 || second.PublishedRevision != 1 {
		t.Fatalf("second revisions = draft %d live %d", second.DraftRevision, second.PublishedRevision)
	}
	stillPublished, stillRevision, err := repository.GetPublished(ctx, operatorID)
	if err != nil || stillRevision != 1 || string(stillPublished) != string(publicJSON) {
		t.Fatal("second draft changed the public snapshot before publish")
	}
	if _, err := repository.Publish(ctx, operatorID, 1); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("stale publish error = %v, want conflict", err)
	}
}
