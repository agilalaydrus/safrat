package repository

import (
	"context"
	"fmt"
	"time"
)

// HealthSignal is one thing that can go wrong in a way a customer would feel.
//
// Status is deliberately four-valued. A screen with only two ("fine" and
// "broken") has to call something fine when nobody is checking it, and a green
// light that is not actually watching anything is worse than no light at all.
type HealthSignal struct {
	Key             string
	Title           string
	Status          string // OK | WARN | ALERT | UNMONITORED
	Detail          string
	AffectedTenants int32
	Count           int64
	// Where the number came from, so somebody reading this at 2am does not have
	// to guess which table to open.
	Source     string
	OldestSeen *time.Time
}

const (
	HealthOK          = "OK"
	HealthWarn        = "WARN"
	HealthAlert       = "ALERT"
	HealthUnmonitored = "UNMONITORED"
)

// PlatformHealth reads only what a customer would feel. It is not an
// infrastructure console: CPU and memory belong elsewhere, and mixing them in
// makes the list too long to read when something is actually wrong.
func (r *PlatformRepository) PlatformHealth(ctx context.Context) ([]HealthSignal, error) {
	signals := make([]HealthSignal, 0, 7)

	// Dead-letter events. The worker's own comment says a failed event stops
	// being claimed and remains "for ops to inspect" — which means nothing at
	// all until a screen shows it.
	var dead struct {
		count   int64
		tenants int32
		oldest  *time.Time
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::bigint, COUNT(DISTINCT operator_id)::int, MIN(created_at)
		FROM cascade_events
		WHERE processed = false AND attempts >= 5`).Scan(&dead.count, &dead.tenants, &dead.oldest); err != nil {
		return nil, databaseError(err)
	}
	signals = append(signals, HealthSignal{
		Key: "outbox_dead_letter", Title: "Event outbox berhenti dicoba ulang",
		Status: statusFor(dead.count, 1, 1), Count: dead.count, AffectedTenants: dead.tenants,
		OldestSeen: dead.oldest, Source: "cascade_events (attempts >= 5)",
		Detail: pluralDetail(dead.count,
			"Tidak ada event yang menyerah.",
			"%d event sudah berhenti dicoba ulang dan tidak akan terkirim tanpa campur tangan."),
	})

	// Backlog: still being retried, but overdue. Distinct from dead-letter —
	// this one recovers on its own if the worker is running, so it is a warning
	// rather than an alert until it is old.
	var backlog struct {
		count   int64
		tenants int32
		oldest  *time.Time
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::bigint, COUNT(DISTINCT operator_id)::int, MIN(created_at)
		FROM cascade_events
		WHERE processed = false AND attempts < 5 AND available_at < NOW() - INTERVAL '15 minutes'`).
		Scan(&backlog.count, &backlog.tenants, &backlog.oldest); err != nil {
		return nil, databaseError(err)
	}
	signals = append(signals, HealthSignal{
		Key: "outbox_backlog", Title: "Antrean event tertinggal",
		Status: statusFor(backlog.count, 1, 50), Count: backlog.count, AffectedTenants: backlog.tenants,
		OldestSeen: backlog.oldest, Source: "cascade_events (belum diproses > 15 menit)",
		Detail: pluralDetail(backlog.count,
			"Antrean bersih.",
			"%d event menunggu lebih dari 15 menit. Kalau worker berjalan, ini pulih sendiri."),
	})

	// Bank poller. "Stopped" and "never configured" are different facts and
	// must not render the same: telling somebody a feature has stopped when
	// they never turned it on sends them looking for a fault that is not there.
	var lastMutation *time.Time
	if err := r.pool.QueryRow(ctx, `SELECT MAX(received_at) FROM bank_mutations`).Scan(&lastMutation); err != nil {
		return nil, databaseError(err)
	}
	bankSignal := HealthSignal{
		Key: "bank_poller", Title: "Poller mutasi bank", Source: "bank_mutations.received_at",
		OldestSeen: lastMutation,
	}
	switch {
	case lastMutation == nil:
		bankSignal.Status = HealthUnmonitored
		bankSignal.Detail = "Belum pernah ada mutasi masuk. Selama BANK_FEED_SECRET belum diset, transfer tidak akan pernah tercocokkan otomatis."
	case time.Since(*lastMutation) > 24*time.Hour:
		bankSignal.Status = HealthAlert
		bankSignal.Detail = fmt.Sprintf("Mutasi terakhir %s lalu. Transfer yang masuk tidak akan cocok dengan tagihan mana pun.",
			humanSince(*lastMutation))
	case time.Since(*lastMutation) > 6*time.Hour:
		bankSignal.Status = HealthWarn
		bankSignal.Detail = fmt.Sprintf("Mutasi terakhir %s lalu.", humanSince(*lastMutation))
	default:
		bankSignal.Status = HealthOK
		bankSignal.Detail = fmt.Sprintf("Mutasi terakhir %s lalu.", humanSince(*lastMutation))
	}
	signals = append(signals, bankSignal)

	// Supplier calls that failed. A product sold and not fulfilled is the
	// worst of these, because the customer has already paid.
	var supplier struct {
		count   int64
		tenants int32
		latest  *time.Time
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::bigint, COUNT(DISTINCT operator_id)::int, MAX(created_at)
		FROM supplier_request_logs
		WHERE created_at >= NOW() - INTERVAL '24 hours'
		  AND (outcome IN ('FAILED', 'UNMATCHED') OR (http_status IS NOT NULL AND http_status >= 500))`).
		Scan(&supplier.count, &supplier.tenants, &supplier.latest); err != nil {
		return nil, databaseError(err)
	}
	signals = append(signals, HealthSignal{
		Key: "supplier_failures", Title: "Panggilan supplier gagal (24 jam)",
		Status: statusFor(supplier.count, 3, 10), Count: supplier.count, AffectedTenants: supplier.tenants,
		OldestSeen: supplier.latest, Source: "supplier_request_logs",
		Detail: pluralDetail(supplier.count,
			"Tidak ada kegagalan supplier dalam 24 jam terakhir.",
			"%d panggilan gagal atau tidak terbaca. Produk yang terjual bisa jadi belum dipenuhi."),
	})

	// Subscription invoices that stopped moving. Revenue stops quietly.
	var invoices struct {
		count   int64
		tenants int32
		oldest  *time.Time
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::bigint, COUNT(DISTINCT operator_id)::int, MIN(due_at)
		FROM subscription_invoices
		WHERE status::text = 'PENDING' AND voided_at IS NULL AND due_at < NOW() - INTERVAL '7 days'`).
		Scan(&invoices.count, &invoices.tenants, &invoices.oldest); err != nil {
		return nil, databaseError(err)
	}
	signals = append(signals, HealthSignal{
		Key: "stuck_invoices", Title: "Tagihan langganan macet",
		Status: statusFor(invoices.count, 1, 5), Count: invoices.count, AffectedTenants: invoices.tenants,
		OldestSeen: invoices.oldest, Source: "subscription_invoices (PENDING, lewat 7 hari)",
		Detail: pluralDetail(invoices.count,
			"Tidak ada tagihan yang tertinggal lebih dari sepekan.",
			"%d tagihan lewat jatuh tempo lebih dari sepekan dan masih PENDING."),
	})

	// Orders holding money that nobody has decided about.
	var held struct {
		count   int64
		tenants int32
		oldest  *time.Time
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::bigint, COUNT(DISTINCT operator_id)::int, MIN(created_at)
		FROM orders WHERE status = 'HELD'`).Scan(&held.count, &held.tenants, &held.oldest); err != nil {
		return nil, databaseError(err)
	}
	signals = append(signals, HealthSignal{
		Key: "held_orders", Title: "Transaksi tertahan",
		Status: statusFor(held.count, 1, 10), Count: held.count, AffectedTenants: held.tenants,
		OldestSeen: held.oldest, Source: "orders (status HELD)",
		Detail: pluralDetail(held.count,
			"Tidak ada uang yang menunggu keputusan.",
			"%d transaksi menahan uang yang sudah masuk dan menunggu keputusan."),
	})

	// Backups. Deliberately not green: nothing in this database knows whether
	// the R2 backup ran, and a green light that checks nothing is worse than
	// no light.
	signals = append(signals, HealthSignal{
		Key: "backup", Title: "Backup terakhir", Status: HealthUnmonitored,
		Source: "belum ada sumbernya",
		Detail: "Tidak dipantau dari sini. Cron backup R2 belum terpasang, dan sampai ia melaporkan hasilnya ke database, layar ini tidak bisa menjanjikan apa pun tentang backup.",
	})

	return signals, nil
}

// statusFor turns a count into a status. Two thresholds rather than one,
// because "one is a problem" and "one is normal but fifty is not" are both
// real, and collapsing them makes every signal shout equally.
func statusFor(count int64, warnAt, alertAt int64) string {
	switch {
	case count >= alertAt && alertAt > 0:
		return HealthAlert
	case count >= warnAt && warnAt > 0:
		return HealthWarn
	default:
		return HealthOK
	}
}

func pluralDetail(count int64, whenZero, whenSome string) string {
	if count == 0 {
		return whenZero
	}
	return fmt.Sprintf(whenSome, count)
}

func humanSince(at time.Time) string {
	elapsed := time.Since(at)
	switch {
	case elapsed < time.Hour:
		return fmt.Sprintf("%d menit", int(elapsed.Minutes()))
	case elapsed < 48*time.Hour:
		return fmt.Sprintf("%d jam", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%d hari", int(elapsed.Hours()/24))
	}
}
