package bankfeed

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The signature has to cover the exact bytes sent. Signing one representation
// and sending another is how a signature ends up proving nothing.
func TestDeliverSignsWhatItActuallySends(t *testing.T) {
	const secret = "rahasia-uji"
	var seenBody []byte
	var seenSignature string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenBody, _ = io.ReadAll(r.Body)
		seenSignature = r.Header.Get("X-Signature")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"received":1,"recorded":1,"matched":1}`))
	}))
	defer server.Close()

	result, err := Deliver(context.Background(), server.URL, secret, "SCRAPER", []Mutation{{
		ExternalID: "REF-1", AmountIDR: 1_500_000, Description: "TRANSFER",
		OccurredAtRFC3339: time.Now().Format(time.RFC3339),
	}}, server.Client())
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if result.Matched != 1 {
		t.Fatalf("hasil = %+v", result)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(seenBody)
	if seenSignature != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatal("tanda tangan tidak cocok dengan badan yang benar-benar dikirim")
	}
}

// A rejected batch must not be reported as a successful run: the credits are
// still unrecorded, and a poller that says "done" would hide that.
func TestDeliverReportsRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	if _, err := Deliver(context.Background(), server.URL, "x", "SCRAPER", []Mutation{{
		ExternalID: "REF-1", AmountIDR: 1,
	}}, server.Client()); err == nil {
		t.Fatal("penolakan 401 dilaporkan sebagai sukses")
	}
}

// Refused before any request is built. An unsigned batch would be rejected by
// the endpoint anyway; failing here says why.
func TestDeliverRefusesWithoutASecret(t *testing.T) {
	if _, err := Deliver(context.Background(), "http://invalid", "", "SCRAPER",
		[]Mutation{{ExternalID: "A", AmountIDR: 1}}, nil); err == nil {
		t.Fatal("pengiriman tanpa secret diterima")
	}
}
