package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Platform health, and the judgement the owner asked for: healthy signals are
// shown too.
//
// A screen that only lists problems cannot be told apart from a screen that has
// stopped working. "No warnings" has to mean "checked, and fine" — which is
// only true if the check runs and reports when it finds nothing.
func TestPlatformHealthReportsHealthyAndUnmonitoredSignalsIntegration(t *testing.T) {
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

	repo := NewPlatformRepository(pool)
	signals, err := repo.PlatformHealth(ctx)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	byKey := map[string]HealthSignal{}
	for _, signal := range signals {
		byKey[signal.Key] = signal
		if signal.Title == "" || signal.Detail == "" {
			t.Fatalf("sinyal %q tanpa judul atau penjelasan", signal.Key)
		}
		// Every signal has to say where its number came from, or reading this
		// at 2am means guessing which table to open.
		if signal.Source == "" {
			t.Fatalf("sinyal %q tidak menyebut sumbernya", signal.Key)
		}
		switch signal.Status {
		case HealthOK, HealthWarn, HealthAlert, HealthUnmonitored:
		default:
			t.Fatalf("sinyal %q berstatus %q", signal.Key, signal.Status)
		}
	}

	for _, key := range []string{"outbox_dead_letter", "outbox_backlog", "bank_poller",
		"supplier_failures", "stuck_invoices", "held_orders", "backup"} {
		if _, ok := byKey[key]; !ok {
			t.Fatalf("sinyal %q hilang dari layar kesehatan", key)
		}
	}

	// Backups are never green. Nothing in this database knows whether the R2
	// backup ran, and a green light that checks nothing is worse than none.
	if byKey["backup"].Status != HealthUnmonitored {
		t.Fatalf("backup dilaporkan %q padahal tidak ada yang memeriksanya", byKey["backup"].Status)
	}

	// A dead-letter event must be visible, counted, and attributed to a tenant.
	operatorID, suffix := uuid.NewString(), uuid.NewString()[:8]
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Uji Kesehatan','ID',$3,$4)`, operatorID, "hl-"+suffix, "hl-"+suffix+"@example.test", "hl-"+suffix); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	before := byKey["outbox_dead_letter"]
	if _, err := pool.Exec(ctx, `
		INSERT INTO cascade_events (operator_id, event_type, entity_id, payload, processed, attempts, last_error, idempotency_key)
		VALUES ($1,'uji.kesehatan',$2,'{}'::jsonb,false,5,'menyerah',$3)`,
		operatorID, uuid.NewString(), "hl-"+uuid.NewString()); err != nil {
		t.Fatalf("fixture event: %v", err)
	}

	after, err := repo.PlatformHealth(ctx)
	if err != nil {
		t.Fatalf("health kedua: %v", err)
	}
	var dead HealthSignal
	for _, signal := range after {
		if signal.Key == "outbox_dead_letter" {
			dead = signal
		}
	}
	if dead.Count != before.Count+1 {
		t.Fatalf("event menyerah = %d, mau %d", dead.Count, before.Count+1)
	}
	if dead.Status == HealthOK {
		t.Fatal("event yang sudah menyerah dilaporkan sehat")
	}
	// Every item names how many tenants feel it — the whole reason this screen
	// is not an infrastructure console.
	if dead.AffectedTenants < 1 {
		t.Fatalf("tidak menyebut berapa tenant terdampak: %+v", dead)
	}
	if dead.OldestSeen == nil {
		t.Fatal("tidak menyebut sejak kapan")
	}
}
