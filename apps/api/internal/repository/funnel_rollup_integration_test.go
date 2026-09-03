package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The daily rollup, and the four things that make its numbers trustworthy.
//
// Visitors are counted once per day however many pages they open; the day
// boundary is Asia/Jakarta rather than UTC; a token behaving like a machine is
// dropped whole; and running twice replaces rather than doubles.
func TestFunnelRollUpCountsVisitorsOnceAndIsIdempotentIntegration(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Rollup Uji','ID',$3,$4)`, operatorID, "ru-"+suffix, "ru-"+suffix+"@example.test", "ru-"+suffix); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM funnel_daily WHERE operator_id = $1`, operatorID)
		_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id = $1`, operatorID)
	})

	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("zona: %v", err)
	}
	day := time.Now().In(jakarta)
	token := func(seed string) string {
		return uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed)).String() + uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed+"x")).String()
	}
	alice, bob, crawler := token("alice" + suffix)[:64], token("bob" + suffix)[:64], token("crawler" + suffix)[:64]

	add := func(hash, step string, at time.Time) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO funnel_events (operator_id, visitor_hash, step, utm_source, occurred_at)
			VALUES ($1,$2,$3,'instagram',$4)`, operatorID, hash, step, at); err != nil {
			t.Fatalf("event: %v", err)
		}
	}

	// One person, four pages. That is one visitor, not four.
	noon := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, jakarta)
	for range 4 {
		add(alice, "LANDING", noon)
	}
	add(bob, "LANDING", noon)

	// 01:00 WIB is the previous day in UTC. Grouping by UTC would move this
	// visit to yesterday, and the "when are people active" figure would be
	// wrong by seven hours.
	earlyMorning := time.Date(day.Year(), day.Month(), day.Day(), 1, 0, 0, 0, jakarta)
	add(bob, "KATALOG", earlyMorning)

	// A token behaving like a machine, dropped whole rather than trimmed.
	//
	// A fixed number, not one derived from dailyBotEventCap. Deriving it from
	// the constant under test makes the fixture move with the threshold, so the
	// filter always fires and the assertion can never fail — which is what my
	// first version did.
	const crawlerEvents = 80
	if crawlerEvents <= dailyBotEventCap {
		t.Fatalf("fixture crawler (%d) tidak melewati ambang (%d)", crawlerEvents, dailyBotEventCap)
	}
	for range crawlerEvents {
		add(crawler, "LANDING", noon)
	}

	repo := NewFunnelRepository(pool)
	if _, err := repo.RollUpDay(ctx, day); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	read := func(step string) (int, int) {
		t.Helper()
		var visitors, events int
		if err := pool.QueryRow(ctx, `SELECT visitors, events FROM funnel_daily
			WHERE operator_id = $1 AND day = $2::date AND step = $3`,
			operatorID, day.Format("2006-01-02"), step).Scan(&visitors, &events); err != nil {
			t.Fatalf("baca %s: %v", step, err)
		}
		return visitors, events
	}

	visitors, events := read("LANDING")
	if visitors != 2 {
		t.Fatalf("pengunjung LANDING = %d, mau 2 — satu orang membuka empat halaman tetap satu orang", visitors)
	}
	if events != 5 {
		t.Fatalf("kejadian LANDING = %d, mau 5 — crawler seharusnya dibuang, orang sungguhan tidak", events)
	}
	if morningVisitors, _ := read("KATALOG"); morningVisitors != 1 {
		t.Fatalf("kunjungan 01:00 WIB tidak masuk hari ini: %d — dikelompokkan menurut UTC?", morningVisitors)
	}

	// Running twice replaces. A daily job that doubles eventually reports twice
	// the truth and nobody can tell which day it started.
	if _, err := repo.RollUpDay(ctx, day); err != nil {
		t.Fatalf("rollup kedua: %v", err)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM funnel_daily
		WHERE operator_id = $1 AND day = $2::date AND step = 'LANDING'`, operatorID, day.Format("2006-01-02")).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("%d baris ringkasan untuk satu langkah satu hari", rows)
	}
	if visitorsAgain, eventsAgain := read("LANDING"); visitorsAgain != visitors || eventsAgain != events {
		t.Fatalf("jalan kedua mengubah angka: %d/%d → %d/%d", visitors, events, visitorsAgain, eventsAgain)
	}
}
