package geoip

import (
	"testing"
	"time"
)

func TestDownloadURLEmbedsReleaseMonth(t *testing.T) {
	month := time.Date(2026, time.September, 17, 0, 0, 0, 0, time.UTC)
	want := "https://download.db-ip.com/free/dbip-city-lite-2026-09.mmdb.gz"
	if got := DownloadURL(month); got != want {
		t.Fatalf("DownloadURL = %q, want %q", got, want)
	}
}
