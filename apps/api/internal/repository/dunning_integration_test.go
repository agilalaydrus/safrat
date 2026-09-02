package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The dunning sequence, driven end to end against the real schema.
//
// Four things are load-bearing and each is checked rather than assumed: the
// stage chosen is the furthest one reached, a second run sends nothing,
// suspension closes access without touching data, and payment at any point
// lifts it without anybody intervening.
func TestDunningSequenceIsIdempotentAndPaymentLiftsSuspensionIntegration(t *testing.T) {
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

	operatorID := uuid.NewString()
	suffix := uuid.NewString()[:8]
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan)
	      VALUES ($1,$2,'Travel Dunning','ID',$3,$4,'GROWTH')`,
		operatorID, "dun-"+suffix, "dun-"+suffix+"@example.test", "dun-"+suffix)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id = $1`, operatorID)
	})

	repo := NewSubscriptionRepository(pool)
	settings, err := repo.Settings(ctx)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	// Read from the database, not hardcoded here: if somebody changes the
	// schedule this test follows it rather than going quietly wrong.
	if len(settings.ReminderDays) == 0 || settings.SuspendAfterDays <= settings.ReminderDays[len(settings.ReminderDays)-1] {
		t.Fatalf("jadwal dunning tidak masuk akal: %#v", settings)
	}

	lapsedAt := time.Now().Add(-time.Duration(settings.ReminderDays[len(settings.ReminderDays)-1]) * 24 * time.Hour)
	exec(`INSERT INTO subscriptions (operator_id,plan,status,access_until)
	      VALUES ($1,'GROWTH','ACTIVE',$2)`, operatorID, lapsedAt)

	mine := func(steps []DunningStep) *DunningStep {
		for i := range steps {
			if steps[i].OperatorID == operatorID {
				return &steps[i]
			}
		}
		return nil
	}

	steps, err := repo.DueDunning(ctx, settings)
	if err != nil {
		t.Fatalf("due dunning: %v", err)
	}
	step := mine(steps)
	if step == nil {
		t.Fatal("langganan yang sudah lewat tidak masuk antrean dunning")
	}
	// The furthest stage reached, not the nearest: a worker that was down for a
	// fortnight must not open with a reminder that is already stale.
	last := settings.ReminderDays[len(settings.ReminderDays)-1]
	if step.Stage != "H"+itoa(last) {
		t.Fatalf("tahap = %q, mau H%d (telat %d hari)", step.Stage, last, step.DaysOverdue)
	}

	created, err := repo.RecordDunning(ctx, *step)
	if err != nil || !created {
		t.Fatalf("catat dunning: created=%v err=%v", created, err)
	}
	// Delivery is at-least-once, so the second run is the one that matters.
	again, err := repo.RecordDunning(ctx, *step)
	if err != nil || again {
		t.Fatalf("jalan kedua mengirim ulang: created=%v err=%v", again, err)
	}
	var messages int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM cascade_events WHERE operator_id = $1 AND event_type = 'billing.subscription_dunning'`, operatorID).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if messages != 1 {
		t.Fatalf("%d pesan dibuat untuk satu tahap", messages)
	}

	// Far enough past to reach suspension.
	suspendLapse := time.Now().Add(-time.Duration(settings.SuspendAfterDays+1) * 24 * time.Hour)
	exec(`UPDATE subscriptions SET access_until = $2 WHERE operator_id = $1`, operatorID, suspendLapse)
	steps, err = repo.DueDunning(ctx, settings)
	if err != nil {
		t.Fatalf("due dunning (suspend): %v", err)
	}
	step = mine(steps)
	if step == nil || step.Stage != "SUSPEND" || !step.Suspend {
		t.Fatalf("tahap penangguhan tidak tercapai: %#v", step)
	}
	if _, err := repo.RecordDunning(ctx, *step); err != nil {
		t.Fatalf("catat penangguhan: %v", err)
	}

	var suspendedAt *time.Time
	var accessUntil time.Time
	if err := pool.QueryRow(ctx, `SELECT suspended_at, access_until FROM subscriptions WHERE operator_id = $1`, operatorID).Scan(&suspendedAt, &accessUntil); err != nil {
		t.Fatal(err)
	}
	if suspendedAt == nil {
		t.Fatal("penangguhan tidak tercatat")
	}
	// Suspension closes entry through time, which was already past. Nothing is
	// taken away, and the pilgrims are still there.
	if accessUntil.After(time.Now()) {
		t.Fatalf("akses masih terbuka setelah ditangguhkan: %v", accessUntil)
	}

	// Payment lifts it, at any stage, with nobody intervening.
	invoiceID := uuid.NewString()
	exec(`INSERT INTO subscription_invoices
	      (id,operator_id,plan,status,channel,base_amount_idr,amount_idr,period_start,period_end,due_at)
	      VALUES ($1,$2,'GROWTH','PENDING','BANK_TRANSFER',789000,789000+$3,NOW(),NOW()+INTERVAL '30 days',NOW()+INTERVAL '7 days')`,
		invoiceID, operatorID, timeSuffix())
	if err := repo.MarkPaid(ctx, invoiceID); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT suspended_at, access_until FROM subscriptions WHERE operator_id = $1`, operatorID).Scan(&suspendedAt, &accessUntil); err != nil {
		t.Fatal(err)
	}
	if suspendedAt != nil {
		t.Fatal("pembayaran tidak mencabut penangguhan — travel yang sudah bayar tetap terkunci")
	}
	if !accessUntil.After(time.Now()) {
		t.Fatalf("akses tidak pulih setelah bayar: %v", accessUntil)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// A unique amount suffix, since pending bank transfers must not share one.
func timeSuffix() int64 { return time.Now().UnixNano() % 900 }
