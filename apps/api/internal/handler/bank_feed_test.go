package handler_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hajj-saas/api/internal/handler"
	"log/slog"
	"os"
)

// This endpoint settles subscription invoices, so what it refuses matters more
// than what it accepts.
func TestBankFeedRefusesWhatItCannotTrust(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	const secret = "rahasia-uji"
	body := `{"source":"SCRAPER","mutations":[{"external_id":"A1","amount_idr":150000}]}`

	sign := func(payload, key string) string {
		mac := hmac.New(sha256.New, []byte(key))
		mac.Write([]byte(payload))
		return hex.EncodeToString(mac.Sum(nil))
	}

	call := func(h http.HandlerFunc, payload, signature string) int {
		request := httptest.NewRequest(http.MethodPost, "/webhooks/bank-feed", strings.NewReader(payload))
		request.Header.Set("X-Signature", signature)
		recorder := httptest.NewRecorder()
		h(recorder, request)
		return recorder.Code
	}

	// Unset secret: refused, not accepted unauthenticated. An endpoint granting
	// paid access must not be open because a variable was forgotten.
	if code := call(handler.NewBankFeedHandler(logger, nil, ""), body, sign(body, secret)); code != http.StatusServiceUnavailable {
		t.Fatalf("tanpa BANK_FEED_SECRET = %d, mau 503", code)
	}

	configured := handler.NewBankFeedHandler(logger, nil, secret)

	if code := call(configured, body, ""); code != http.StatusUnauthorized {
		t.Fatalf("tanpa tanda tangan = %d, mau 401", code)
	}
	if code := call(configured, body, sign(body, "kunci-salah")); code != http.StatusUnauthorized {
		t.Fatalf("tanda tangan dari kunci lain = %d, mau 401", code)
	}

	// The signature covers the body, so altering the amount after signing must
	// fail — which is the reason for signing rather than sending a token.
	tampered := strings.Replace(body, "150000", "9999999", 1)
	if code := call(configured, tampered, sign(body, secret)); code != http.StatusUnauthorized {
		t.Fatalf("badan yang diubah setelah ditandatangani = %d, mau 401", code)
	}

	// A source this system does not issue is refused before anything is stored.
	unknown := `{"source":"PALSU","mutations":[]}`
	if code := call(configured, unknown, sign(unknown, secret)); code != http.StatusBadRequest {
		t.Fatalf("sumber tidak dikenal = %d, mau 400", code)
	}
}
