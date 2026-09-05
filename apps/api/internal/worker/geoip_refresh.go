package worker

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/geoip"
	"github.com/hibiken/asynq"
)

const TaskGeoIPRefresh = "geoip:refresh"

func NewGeoIPRefreshTask() *asynq.Task {
	return asynq.NewTask(TaskGeoIPRefresh, nil, asynq.MaxRetry(3))
}

// GeoIPRefreshHandler keeps the DB-IP City Lite database current — see
// K2.8 in TUGAS-CORONG.md. DB-IP publishes a new file every month; this
// checks daily and only actually downloads (60MB+) when the month has
// rolled over, tracked by a small marker file next to the database itself.
type GeoIPRefreshHandler struct {
	logger   *slog.Logger
	dbPath   string
	resolver *geoip.Resolver
}

func NewGeoIPRefreshHandler(logger *slog.Logger, dbPath string, resolver *geoip.Resolver) *GeoIPRefreshHandler {
	return &GeoIPRefreshHandler{logger: logger, dbPath: dbPath, resolver: resolver}
}

// EnsureCurrent runs the same check as HandleRefresh, called once at worker
// startup so a fresh deployment has geolocation working immediately instead
// of waiting for the first scheduled tick (up to 24h away).
func (h *GeoIPRefreshHandler) EnsureCurrent(ctx context.Context) {
	if err := h.HandleRefresh(ctx, nil); err != nil {
		h.logger.Warn("geoip initial refresh failed; will retry on schedule", "error", err)
	}
}

func (h *GeoIPRefreshHandler) HandleRefresh(ctx context.Context, _ *asynq.Task) error {
	if strings.TrimSpace(h.dbPath) == "" {
		return nil
	}
	month := time.Now().UTC().Format("2006-01")
	marker := h.dbPath + ".month"
	if current, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(current)) == month {
		return nil
	}
	// A failed download (network hiccup, DB-IP not yet published this
	// month's file) is never fatal — the previous month's database keeps
	// serving lookups until the next check succeeds.
	if err := geoip.DownloadLatest(h.dbPath); err != nil {
		h.logger.Warn("geoip database download failed; keeping previous database", "error", err)
		return nil
	}
	if err := h.resolver.Reload(h.dbPath); err != nil {
		h.logger.Error("geoip database downloaded but failed to load", "error", err)
		return nil
	}
	if err := os.WriteFile(marker, []byte(month), 0o644); err != nil {
		h.logger.Warn("geoip refresh marker not written; will re-download next check", "error", err)
	}
	h.logger.Info("geoip database refreshed", "month", month)
	return nil
}
