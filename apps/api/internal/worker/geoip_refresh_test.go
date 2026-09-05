package worker

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajj-saas/api/internal/geoip"
)

// TestGeoIPRefreshSkipsDownloadWhenMarkerCurrent proves the daily check does
// not re-download 60MB+ every day: when the marker file already names the
// current month, HandleRefresh must leave the existing database untouched.
func TestGeoIPRefreshSkipsDownloadWhenMarkerCurrent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "geoip.mmdb")
	marker := dbPath + ".month"
	if err := os.WriteFile(dbPath, []byte("SENTINEL"), 0o644); err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}
	if err := os.WriteFile(marker, []byte(time.Now().UTC().Format("2006-01")), 0o644); err != nil {
		t.Fatalf("seed marker: %v", err)
	}

	handler := NewGeoIPRefreshHandler(slog.New(slog.NewTextHandler(os.Stderr, nil)), dbPath, geoip.Open(""))
	if err := handler.HandleRefresh(context.Background(), nil); err != nil {
		t.Fatalf("HandleRefresh: %v", err)
	}

	content, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read dbPath after refresh: %v", err)
	}
	if string(content) != "SENTINEL" {
		t.Fatalf("marker sudah bulan ini seharusnya tidak mengunduh ulang, tapi isi berubah: %q", content)
	}
}

func TestGeoIPRefreshNoopsWhenUnconfigured(t *testing.T) {
	handler := NewGeoIPRefreshHandler(slog.New(slog.NewTextHandler(os.Stderr, nil)), "", geoip.Open(""))
	if err := handler.HandleRefresh(context.Background(), nil); err != nil {
		t.Fatalf("HandleRefresh dengan path kosong seharusnya diam, dapat: %v", err)
	}
}
