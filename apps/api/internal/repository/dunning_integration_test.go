package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
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

	// Two things this fixture must not depend on, both of which made this test
	// fail intermittently when the suite ran packages in parallel.
	//
	// The grace period is pinned to zero on this row rather than left NULL.
	// NULL falls back to the platform-wide setting, which another test may
	// change while this one runs — and grace is added to access_until, so a
	// global change of two days moves this subscription out of the dunning
	// queue entirely and the failure reads "langganan yang sudah lewat tidak
	// masuk antrean", which points nowhere near the cause.
	//
	// The timestamp is built in Postgres and placed six hours past the
	// boundary, not exactly on it. days_overdue is a FLOOR over the difference
	// against Postgres NOW(); a value computed from Go's clock and landing
	// exactly on the boundary falls to the previous stage whenever the two
	// clocks disagree by a millisecond. The same trap already produced a flaky
	// proration test once.
	lastReminder := settings.ReminderDays[len(settings.ReminderDays)-1]
	exec(`INSERT INTO subscriptions (operator_id,plan,status,access_until,grace_period_days)
	      VALUES ($1,'GROWTH','ACTIVE', NOW() - make_interval(days => $2::int) - INTERVAL '6 hours', 0)`,
		operatorID, lastReminder)

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
	// GATEWAY, not BANK_TRANSFER. A pending transfer invoice must hold an amount
	// that is unique among today's transfers — that uniqueness is what ties an
	// incoming bank credit to one invoice — so a fixture inventing amounts
	// collides with itself across runs. This test is about the dunning
	// sequence, not about matching credits.
	invoiceID := uuid.NewString()
	exec(`INSERT INTO subscription_invoices
	      (id,operator_id,plan,status,channel,base_amount_idr,amount_idr,period_start,period_end,due_at)
	      VALUES ($1,$2,'GROWTH','PENDING','GATEWAY',789000,789000,NOW(),NOW()+INTERVAL '30 days',NOW()+INTERVAL '7 days')`,
		invoiceID, operatorID)
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

func TestTenantGracePeriodControlsAccessAndIdempotencyIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	repo := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")
	if err := repo.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	paidUntil := time.Now().UTC().Truncate(time.Microsecond).Add(-24 * time.Hour)
	if _, err := pool.Exec(ctx, `UPDATE subscriptions SET status='ACTIVE', access_until=$2 WHERE operator_id=$1`, operatorID, paidUntil); err != nil {
		t.Fatalf("prepare lapse: %v", err)
	}
	before, err := repo.GetAccess(ctx, operatorID)
	if err != nil || before.Allowed {
		t.Fatalf("access before grace = %+v err=%v, want closed", before, err)
	}

	days := int32(3)
	change := GracePeriodChange{
		OperatorID: operatorID, Days: &days, Reason: "beri waktu kliring transfer",
		Confirmation: "Sub Test", ActorUserID: "grace-test", IdempotencyKey: "grace-test-" + operatorID,
	}
	result, err := repo.SetGracePeriod(ctx, change)
	if err != nil || result.EffectiveDays != 3 || result.OverrideDays == nil || *result.OverrideDays != 3 {
		t.Fatalf("set grace = %+v err=%v", result, err)
	}
	after, err := repo.GetAccess(ctx, operatorID)
	if err != nil || !after.Allowed || after.GracePeriodDays != 3 {
		t.Fatalf("access in grace = %+v err=%v, want allowed", after, err)
	}
	if !after.AccessUntil.Equal(paidUntil) || !after.EffectiveAccessUntil.Equal(paidUntil.Add(72*time.Hour)) {
		t.Fatalf("paid/effective horizon changed incorrectly: %+v", after)
	}
	if _, err := repo.MarkLapsed(ctx); err != nil {
		t.Fatalf("mark lapsed: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM subscriptions WHERE operator_id=$1`, operatorID).Scan(&status); err != nil || status != "ACTIVE" {
		t.Fatalf("status during grace = %q err=%v, want ACTIVE", status, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE subscriptions SET suspended_at=NOW(), suspended_reason='uji' WHERE operator_id=$1`, operatorID); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	suspended, _ := repo.GetAccess(ctx, operatorID)
	if suspended.Allowed {
		t.Fatal("grace reopened a deliberately suspended tenant")
	}
	if _, err := repo.SetGracePeriod(ctx, change); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	other := int32(4)
	change.Days = &other
	if _, err := repo.SetGracePeriod(ctx, change); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("same key with different grace = %v, want conflict", err)
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
