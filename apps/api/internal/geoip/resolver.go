// Package geoip resolves a visitor's city/province from their IP address —
// K2.8 in TUGAS-CORONG.md. City level, deliberately no finer, and the raw IP
// never survives the lookup: FunnelService calls Resolver.Lookup with the
// same clientIP it's about to hash and throw away, and only the resulting
// place names (or nothing, if the address isn't found) reach funnel_events.
package geoip

import (
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// cityRecord mirrors the subset of DB-IP City Lite's schema (itself
// deliberately compatible with MaxMind's GeoIP2-City layout) this package
// actually uses. English names only — DB-IP Lite's free tier does not ship
// an "id" locale, so city/province surface in English (e.g. "Central Java").
type cityRecord struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
}

// Resolver wraps an mmdb file that can be swapped out at runtime (see
// Reload), because internal/worker/geoip_refresh.go replaces it monthly
// without restarting the server.
type Resolver struct {
	mu sync.RWMutex
	db *maxminddb.Reader
}

// Open loads path if given. A missing file, an empty path, or a corrupt
// database all leave the resolver silently unconfigured — geolocation is a
// convenience layer on the funnel, never a requirement of it, so a bad path
// must not stop the server from starting or the funnel from recording.
func Open(path string) *Resolver {
	r := &Resolver{}
	if strings.TrimSpace(path) == "" {
		return r
	}
	_ = r.Reload(path)
	return r
}

// Reload swaps in the database at path, closing the previous one only after
// the new one is live — a lookup in flight never sees a closed reader.
func (r *Resolver) Reload(path string) error {
	db, err := maxminddb.Open(path)
	if err != nil {
		return err
	}
	r.mu.Lock()
	old := r.db
	r.db = db
	r.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// WatchForChanges polls path's mtime every interval and reloads when it
// changes, in a background goroutine that runs for the life of the process.
// This is how the server picks up the database when it wasn't there yet at
// boot (a fresh deploy, before cmd/worker's first download completes) and
// how it picks up cmd/worker's monthly refresh afterwards — both without a
// server restart. A no-op if path is empty.
func (r *Resolver) WatchForChanges(path string, interval time.Duration) {
	if strings.TrimSpace(path) == "" {
		return
	}
	go func() {
		var lastMod time.Time
		for {
			if info, err := os.Stat(path); err == nil && info.ModTime().After(lastMod) {
				if err := r.Reload(path); err == nil {
					lastMod = info.ModTime()
				}
			}
			time.Sleep(interval)
		}
	}()
}

// Lookup returns (city, province) for ipStr, both empty if the resolver has
// no database loaded, the address doesn't parse, or the address simply
// isn't in the database (private ranges, localhost in dev, freshly
// allocated blocks the monthly snapshot hasn't caught up to yet).
func (r *Resolver) Lookup(ipStr string) (city, province string) {
	r.mu.RLock()
	db := r.db
	r.mu.RUnlock()
	if db == nil {
		return "", ""
	}
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return "", ""
	}
	var record cityRecord
	if err := db.Lookup(ip, &record); err != nil {
		return "", ""
	}
	city = record.City.Names["en"]
	if len(record.Subdivisions) > 0 {
		province = record.Subdivisions[0].Names["en"]
	}
	return city, province
}
