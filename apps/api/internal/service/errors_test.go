package service

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// F2 (TUGAS-PANEL-SAAS.md): an unmapped error must not vanish silently in
// local dev just because SENTRY_DSN is unset — it has to land somewhere a
// person watching the running process can actually see, which in this
// codebase means slog.
func TestServiceErrorLogsUnmappedErrorsIntegration(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(previous)

	err := serviceError("SomeService.SomeMethod", errors.New("kegagalan yang belum dipetakan"))
	if err == nil {
		t.Fatal("nil, mau internal error")
	}

	logged := buf.String()
	if !strings.Contains(logged, "SomeService.SomeMethod") {
		t.Fatalf("nama metode tidak muncul di log: %q", logged)
	}
	if !strings.Contains(logged, "kegagalan yang belum dipetakan") {
		t.Fatalf("pesan galat tidak muncul di log: %q", logged)
	}
}
