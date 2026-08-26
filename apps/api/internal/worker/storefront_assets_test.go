package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hajj-saas/api/internal/domain"
	"github.com/hibiken/asynq"
)

type fakeStorefrontRegistry struct {
	assets              []domain.StorefrontAsset
	reservations        []domain.StorefrontAssetReservation
	before              time.Time
	deleted             []domain.StorefrontAsset
	deletedReservations []domain.StorefrontAssetReservation
}

func (f *fakeStorefrontRegistry) ListExpiredReservations(context.Context, time.Time, int32) ([]domain.StorefrontAssetReservation, error) {
	return f.reservations, nil
}
func (f *fakeStorefrontRegistry) DeleteReservation(_ context.Context, operatorID, reservationKey string) error {
	f.deletedReservations = append(f.deletedReservations, domain.StorefrontAssetReservation{OperatorID: operatorID, ReservationKey: reservationKey})
	return nil
}
func (f *fakeStorefrontRegistry) RefreshOrphans(context.Context) (int64, error) { return 4, nil }
func (f *fakeStorefrontRegistry) ListOrphans(_ context.Context, before time.Time, _ int32) ([]domain.StorefrontAsset, error) {
	f.before = before
	return f.assets, nil
}
func (f *fakeStorefrontRegistry) DeleteRecord(_ context.Context, operatorID, objectKey string) error {
	f.deleted = append(f.deleted, domain.StorefrontAsset{OperatorID: operatorID, ObjectKey: objectKey})
	return nil
}

type fakeStorefrontStorage struct {
	deleted []domain.StorefrontAsset
	err     error
}

func (f *fakeStorefrontStorage) DeleteStorefrontObject(_ context.Context, operatorID, objectKey string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, domain.StorefrontAsset{OperatorID: operatorID, ObjectKey: objectKey})
	return nil
}

func TestStorefrontAssetGCHonorsGraceAndDeletesStorageFirst(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	asset := domain.StorefrontAsset{OperatorID: "operator-1", ObjectKey: "storefront/operator-1/hero/file.webp"}
	reservation := domain.StorefrontAssetReservation{OperatorID: "operator-1", ReservationKey: "storefront-pending/operator-1/hero/pending.webp"}
	registry := &fakeStorefrontRegistry{assets: []domain.StorefrontAsset{asset}, reservations: []domain.StorefrontAssetReservation{reservation}}
	objectStorage := &fakeStorefrontStorage{}
	handler := NewStorefrontAssetHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), registry, objectStorage)
	handler.now = func() time.Time { return now }

	if err := handler.HandleGC(context.Background(), asynq.NewTask(TaskStorefrontAssetGC, nil)); err != nil {
		t.Fatalf("HandleGC: %v", err)
	}
	if want := now.Add(-storefrontOrphanGrace); !registry.before.Equal(want) {
		t.Fatalf("orphan cutoff = %v, want %v", registry.before, want)
	}
	if len(objectStorage.deleted) != 2 || len(registry.deleted) != 1 || len(registry.deletedReservations) != 1 {
		t.Fatalf("deleted storage=%d records=%d reservations=%d", len(objectStorage.deleted), len(registry.deleted), len(registry.deletedReservations))
	}
	if objectStorage.deleted[0].ObjectKey != "storefront/operator-1/hero/pending.webp" {
		t.Fatalf("expired reservation live cleanup key = %q", objectStorage.deleted[0].ObjectKey)
	}
}

func TestStorefrontAssetGCLeavesRegistryRecordWhenStorageFails(t *testing.T) {
	want := errors.New("storage unavailable")
	registry := &fakeStorefrontRegistry{assets: []domain.StorefrontAsset{{OperatorID: "operator-1", ObjectKey: "storefront/operator-1/logo/file.webp"}}}
	handler := NewStorefrontAssetHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), registry, &fakeStorefrontStorage{err: want})

	if err := handler.HandleGC(context.Background(), asynq.NewTask(TaskStorefrontAssetGC, nil)); !errors.Is(err, want) {
		t.Fatalf("HandleGC error = %v, want %v", err, want)
	}
	if len(registry.deleted) != 0 {
		t.Fatal("registry record deleted despite storage failure")
	}
}
