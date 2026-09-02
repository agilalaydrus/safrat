package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestFunnelClientIPRequiresFreshValidSignature(t *testing.T) {
	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	secret := "a-separate-ingest-secret-with-32-bytes"
	handler := &FunnelHandler{ingestSecret: []byte(secret), now: func() time.Time { return now }}

	signed := func(clientIP string, at time.Time, userAgent string) http.Header {
		timestamp := strconv.FormatInt(at.Unix(), 10)
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(timestamp + "\n" + clientIP + "\n" + userAgent))
		return http.Header{
			"User-Agent":         []string{userAgent},
			"X-Real-Ip":          []string{"172.19.0.8"},
			"X-Funnel-Client-Ip": []string{clientIP},
			"X-Funnel-Timestamp": []string{timestamp},
			"X-Funnel-Signature": []string{hex.EncodeToString(mac.Sum(nil))},
		}
	}

	const visitorIP = "103.150.60.10"
	const browser = "Mozilla/5.0 Safari/605.1.15"
	valid := signed(visitorIP, now, browser)
	if got := handler.clientIP(valid, "172.19.0.8:8080"); got != visitorIP {
		t.Fatalf("valid signed address = %q, want %q", got, visitorIP)
	}

	tampered := valid.Clone()
	tampered.Set("X-Funnel-Client-IP", "203.0.113.5")
	if got := handler.clientIP(tampered, "172.19.0.8:8080"); got != "172.19.0.8" {
		t.Fatalf("tampered signed address was trusted: %q", got)
	}

	stale := signed(visitorIP, now.Add(-funnelSignatureMaxAge-time.Second), browser)
	if got := handler.clientIP(stale, "172.19.0.8:8080"); got != "172.19.0.8" {
		t.Fatalf("stale signed address was trusted: %q", got)
	}

	shortSecret := &FunnelHandler{ingestSecret: []byte("too-short"), now: func() time.Time { return now }}
	if got := shortSecret.clientIP(valid, "172.19.0.8:8080"); got != "172.19.0.8" {
		t.Fatalf("short secret trusted forwarding: %q", got)
	}
}
