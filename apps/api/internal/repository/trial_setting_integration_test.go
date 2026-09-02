package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Trial length is a setting, and two things about it matter.
//
// It has to actually be read — the row said ten for a day while the code still
// handed out three, which is the failure this test exists to prevent recurring.
// And lowering it must never cut short a trial already running: an agency keeps
// the terms that applied when it signed up.
func TestTrialLengthComesFromSettingsAndDoesNotShortenRunningTrialsIntegration(t *testing.T) {
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	var original string
	if err := pool.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = 'trial_days'`).Scan(&original); err != nil {
		t.Fatalf("baca setelan: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE platform_settings SET value = $1 WHERE key = 'trial_days'`, original)
	})

	newOperator := func(label string) string {
		t.Helper()
		id := uuid.NewString()
		suffix := uuid.NewString()[:8]
		if _, err := pool.Exec(ctx, `INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
			VALUES ($1,$2,$3,'ID',$4,$5)`, id, "trial-"+suffix, "Trial "+label, "trial-"+suffix+"@example.test", "trial-"+suffix); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, id)
		})
		return id
	}
	daysGranted := func(operatorID string) float64 {
		t.Helper()
		var until time.Time
		if err := pool.QueryRow(ctx, `SELECT access_until FROM subscriptions WHERE operator_id = $1`, operatorID).Scan(&until); err != nil {
			t.Fatalf("baca akses: %v", err)
		}
		return time.Until(until).Hours() / 24
	}

	subscriptions := NewSubscriptionRepository(pool)

	// The decided value, read from the database rather than written here — if
	// somebody changes the decision, this follows it instead of going stale.
	if _, err := pool.Exec(ctx, `UPDATE platform_settings SET value = '10' WHERE key = 'trial_days'`); err != nil {
		t.Fatalf("set 10: %v", err)
	}
	first := newOperator("sepuluh")
	if err := subscriptions.EnsureForOperator(ctx, first); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if got := daysGranted(first); got < 9.5 || got > 10.5 {
		t.Fatalf("trial %.2f hari, mau 10 — setelan tidak dibaca", got)
	}

	// Lowering it changes what new agencies get...
	if _, err := pool.Exec(ctx, `UPDATE platform_settings SET value = '2' WHERE key = 'trial_days'`); err != nil {
		t.Fatalf("set 2: %v", err)
	}
	second := newOperator("dua")
	if err := subscriptions.EnsureForOperator(ctx, second); err != nil {
		t.Fatalf("ensure kedua: %v", err)
	}
	if got := daysGranted(second); got < 1.5 || got > 2.5 {
		t.Fatalf("trial kedua %.2f hari, mau 2", got)
	}
	// ...and must leave the one already running exactly where it was.
	if got := daysGranted(first); got < 9.5 {
		t.Fatalf("trial berjalan dipotong jadi %.2f hari saat setelan diturunkan", got)
	}

	// A malformed row must not stop signups. Falling back is the only safe
	// behaviour: refusing to create a subscription would block registration
	// entirely over a typo in a settings table.
	if _, err := pool.Exec(ctx, `UPDATE platform_settings SET value = 'sepuluh hari' WHERE key = 'trial_days'`); err != nil {
		t.Fatalf("set rusak: %v", err)
	}
	third := newOperator("rusak")
	if err := subscriptions.EnsureForOperator(ctx, third); err != nil {
		t.Fatalf("nilai rusak menghentikan pendaftaran: %v", err)
	}
	if got := daysGranted(third); got < 9.5 || got > 10.5 {
		t.Fatalf("fallback %.2f hari, mau 10", got)
	}
}
