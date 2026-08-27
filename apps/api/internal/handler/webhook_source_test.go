package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func requestFrom(address string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/webhooks/xendit", nil)
	request.Header.Set("X-Real-IP", address)
	return request
}

func TestWebhookSourceGuard(t *testing.T) {
	t.Run("an empty allowlist permits everything, so a missing variable cannot stop payments", func(t *testing.T) {
		guard := NewWebhookSourceGuard(quietLogger(), "")
		if !guard.Allows(requestFrom("203.0.113.9")) {
			t.Fatal("an unconfigured guard rejected a delivery")
		}
	})

	t.Run("a configured allowlist admits listed addresses and ranges", func(t *testing.T) {
		guard := NewWebhookSourceGuard(quietLogger(), "203.0.113.1, 198.51.100.0/24")
		for _, address := range []string{"203.0.113.1", "198.51.100.7", "198.51.100.255"} {
			if !guard.Allows(requestFrom(address)) {
				t.Fatalf("%s was rejected but is allowed", address)
			}
		}
	})

	t.Run("everything else is refused", func(t *testing.T) {
		guard := NewWebhookSourceGuard(quietLogger(), "203.0.113.1, 198.51.100.0/24")
		for _, address := range []string{"203.0.113.2", "198.51.101.7", "8.8.8.8", "", "not-an-ip"} {
			if guard.Allows(requestFrom(address)) {
				t.Fatalf("%q was admitted but is not on the allowlist", address)
			}
		}
	})

	t.Run("an unparseable entry is skipped without disabling the rest", func(t *testing.T) {
		guard := NewWebhookSourceGuard(quietLogger(), "not-a-cidr, 203.0.113.1")
		if !guard.Allows(requestFrom("203.0.113.1")) {
			t.Fatal("a valid entry stopped working because another entry was malformed")
		}
		if guard.Allows(requestFrom("8.8.8.8")) {
			t.Fatal("a malformed entry opened the guard up")
		}
	})

	t.Run("without a proxy header the connection's own address is used, never a client-supplied one", func(t *testing.T) {
		guard := NewWebhookSourceGuard(quietLogger(), "192.0.2.0/24")
		request := httptest.NewRequest(http.MethodPost, "/webhooks/xendit", nil)
		request.RemoteAddr = "192.0.2.5:44321"
		if !guard.Allows(request) {
			t.Fatal("a delivery from an allowed address was rejected")
		}
		request.RemoteAddr = "8.8.8.8:44321"
		if guard.Allows(request) {
			t.Fatal("a delivery from outside the allowlist was admitted")
		}
	})
}
