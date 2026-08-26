package worker

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/domain"
	"github.com/hibiken/asynq"
)

const (
	TaskStorefrontAssetGC = "storefront-assets:gc"
	storefrontOrphanGrace = 7 * 24 * time.Hour
)

type storefrontAssetRegistry interface {
	ListExpiredReservations(context.Context, time.Time, int32) ([]domain.StorefrontAssetReservation, error)
	DeleteReservation(context.Context, string, string) error
	RefreshOrphans(context.Context) (int64, error)
	ListOrphans(context.Context, time.Time, int32) ([]domain.StorefrontAsset, error)
	DeleteRecord(context.Context, string, string) error
}

type storefrontObjectDeleter interface {
	DeleteStorefrontObject(context.Context, string, string) error
}

type StorefrontAssetHandler struct {
	logger   *slog.Logger
	registry storefrontAssetRegistry
	storage  storefrontObjectDeleter
	now      func() time.Time
}

func NewStorefrontAssetGCTask() *asynq.Task {
	return asynq.NewTask(TaskStorefrontAssetGC, nil, asynq.MaxRetry(3))
}

func NewStorefrontAssetHandler(logger *slog.Logger, registry storefrontAssetRegistry, objectStorage storefrontObjectDeleter) *StorefrontAssetHandler {
	return &StorefrontAssetHandler{logger: logger, registry: registry, storage: objectStorage, now: time.Now}
}

func (h *StorefrontAssetHandler) HandleGC(ctx context.Context, _ *asynq.Task) error {
	now := h.now()
	reservations, err := h.registry.ListExpiredReservations(ctx, now, 100)
	if err != nil {
		return err
	}
	var failures []error
	purged := 0
	for _, reservation := range reservations {
		liveKey := strings.Replace(reservation.ReservationKey, "storefront-pending/", "storefront/", 1)
		if err := h.storage.DeleteStorefrontObject(ctx, reservation.OperatorID, liveKey); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := h.registry.DeleteReservation(ctx, reservation.OperatorID, reservation.ReservationKey); err != nil {
			failures = append(failures, err)
			continue
		}
		purged++
	}
	refreshed, err := h.registry.RefreshOrphans(ctx)
	if err != nil {
		return errors.Join(append(failures, err)...)
	}
	assets, err := h.registry.ListOrphans(ctx, now.Add(-storefrontOrphanGrace), 100)
	if err != nil {
		return errors.Join(append(failures, err)...)
	}

	deleted := 0
	for _, asset := range assets {
		if err := h.storage.DeleteStorefrontObject(ctx, asset.OperatorID, asset.ObjectKey); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := h.registry.DeleteRecord(ctx, asset.OperatorID, asset.ObjectKey); err != nil {
			failures = append(failures, err)
			continue
		}
		deleted++
	}
	h.logger.Info("storefront asset cleanup", "expired_reservations_eligible", len(reservations), "expired_reservations_purged", purged, "assets_scanned", refreshed, "orphans_eligible", len(assets), "assets_deleted", deleted, "failures", len(failures))
	return errors.Join(failures...)
}
