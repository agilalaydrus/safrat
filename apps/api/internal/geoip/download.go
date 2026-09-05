package geoip

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// httpClient bounds the download so a network hiccup can never hang the
// worker's startup or a scheduled tick indefinitely — the file is 60MB+,
// so this is generous, not tight.
var httpClient = &http.Client{Timeout: 5 * time.Minute}

// DownloadURL is DB-IP's free-tier City Lite database. No account, license
// key, or credential of any kind is needed — unlike MaxMind, this is exactly
// why DB-IP was chosen (see TUGAS-CORONG.md K2.8). The filename embeds the
// release month; DB-IP publishes a new one on the 1st of every month and
// keeps the previous month's file reachable for a while, but not forever.
func DownloadURL(month time.Time) string {
	return fmt.Sprintf("https://download.db-ip.com/free/dbip-city-lite-%s.mmdb.gz", month.Format("2006-01"))
}

// DownloadLatest fetches this month's database and decompresses it to
// destPath, writing to a temp file first and renaming over the old one so a
// reader opening destPath never sees a half-written file.
func DownloadLatest(destPath string) error {
	return download(DownloadURL(time.Now().UTC()), destPath)
}

func download(url, destPath string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("geoip: download %s: status %d", url, resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".geoip-download-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, gz); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, destPath)
}
