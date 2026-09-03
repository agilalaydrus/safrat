package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The platform funnel, and the three things that make it useful rather than
// decorative.
//
// TawafiqHub's own visits must not be mixed with its clients'; a storefront
// nobody opened must still appear, because that is the agency about to cancel;
// and a storefront with three visitors must not top the conversion board.
func TestPlatformFunnelSeparatesOwnTrafficAndFindsSilentStorefrontsIntegration(t *testing.T) {
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

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	newOperator := func(tag string) string {
		id, suffix := uuid.NewString(), uuid.NewString()[:8]
		exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
			VALUES ($1,$2,$3,'ID',$4,$5)`, id, tag+"-"+suffix, tag+" "+suffix, tag+"-"+suffix+"@example.test", tag+"-"+suffix)
		t.Cleanup(func() {
			bg := context.Background()
			_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id = $1`, id)
		})
		return id
	}

	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("zona: %v", err)
	}
	today := time.Now().In(jakarta).Format("2006-01-02")

	ramai := newOperator("ramai") // plenty of traffic, converts
	sepi := newOperator("sepi")   // traffic, no registrations — the work list
	kecil := newOperator("kecil") // below the ranking floor
	diam := newOperator("diam")   // nobody has ever opened it

	exec(`INSERT INTO funnel_daily (operator_id,day,step,utm_source,visitors,events) VALUES
		($1,$4::date,'LANDING','',500,700),
		($2,$4::date,'LANDING','',300,400),
		($3,$4::date,'LANDING','',3,4)`, ramai, sepi, kecil, today)

	// TawafiqHub's own funnel. operator_id NULL, and it must stay out of the
	// storefront totals entirely.
	//
	// The rows carry a channel unique to this run. Platform rows belong to no
	// operator, so nothing cascades them away when the fixture operators are
	// deleted — without a key of its own, a second run collides with the first
	// on funnel_daily_key_idx and the fixture fails before the test starts.
	platformSource := "uji-platform-" + uuid.NewString()[:8]
	exec(`INSERT INTO funnel_daily (operator_id,day,step,utm_source,visitors,events) VALUES
		(NULL,$1::date,'LANDING',$2,8087,9000),
		(NULL,$1::date,'DAFTAR',$2,9,11)`, today, platformSource)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM funnel_daily WHERE operator_id IS NULL AND utm_source = $1`, platformSource)
	})

	season := uuid.NewString()
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
		VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',60)`, season, ramai)
	for range 25 {
		exec(`INSERT INTO pilgrim_registrations (operator_id,season_id,full_name) VALUES ($1,$2,'Pendaftar')`, ramai, season)
	}

	repo := NewFunnelRepository(pool)
	report, err := repo.PlatformFunnel(ctx, 30)
	if err != nil {
		t.Fatalf("platform funnel: %v", err)
	}

	find := func(rows []StorefrontFunnelRow, id string) *StorefrontFunnelRow {
		for index := range rows {
			if rows[index].OperatorID == id {
				return &rows[index]
			}
		}
		return nil
	}

	// Our own traffic is ours. Mixing it in would inflate the number we quote
	// when selling with the very visitors we are selling the ability to get.
	var ownLanding int32
	for _, step := range report.PlatformSteps {
		if step.Step == "LANDING" {
			ownLanding = step.Visitors
		}
	}
	// A range rather than an exact number: this database is shared with other
	// tests, and another one may legitimately add platform rows for today. The
	// window is narrower than the smallest client fixture (300), so any client
	// traffic folded in here still fails the assertion.
	if ownLanding < 8087 || ownLanding >= 8087+300 {
		t.Fatalf("LANDING platform = %d, mau ~8087 — pengunjung klien ikut terhitung?", ownLanding)
	}
	if find(report.Storefronts, "") != nil {
		t.Fatal("baris tanpa operator masuk daftar storefront")
	}
	for _, row := range append(append([]StorefrontFunnelRow{}, report.Storefronts...), report.Silent...) {
		if row.Visitors >= 8087 {
			t.Fatalf("lalu lintas platform muncul sebagai storefront: %+v", row)
		}
	}

	// A storefront nobody opened is the whole point of the screen.
	if find(report.Silent, diam) == nil {
		t.Fatalf("storefront tanpa pengunjung tidak muncul: %+v", report.Silent)
	}
	if find(report.Storefronts, diam) != nil {
		t.Fatal("storefront tanpa pengunjung ikut diperingkat")
	}

	// Three visitors and no registrations is not a rankable rate.
	if find(report.TooFewVisitors, kecil) == nil {
		t.Fatalf("storefront di bawah ambang tidak dipisahkan: %+v", report.TooFewVisitors)
	}
	if find(report.Storefronts, kecil) != nil {
		t.Fatal("storefront 3 pengunjung ikut diperingkat")
	}

	// Best first, and the bottom is the work list.
	ranked := report.Storefronts
	if len(ranked) < 2 {
		t.Fatalf("hanya %d storefront yang diperingkat", len(ranked))
	}
	best, worst := find(ranked, ramai), find(ranked, sepi)
	if best == nil || worst == nil {
		t.Fatalf("storefront yang diharapkan tidak ada: %+v", ranked)
	}
	if best.Conversion <= worst.Conversion {
		t.Fatalf("konversi %v tidak di atas %v", best.Conversion, worst.Conversion)
	}
	for index := 1; index < len(ranked); index++ {
		if ranked[index-1].Conversion < ranked[index].Conversion {
			t.Fatalf("urutan tidak menurun di %d: %+v", index, ranked)
		}
	}
	if worst.Registrations != 0 {
		t.Fatalf("storefront tanpa pendaftar punya %d pendaftar", worst.Registrations)
	}
	if best.Registrations != 25 {
		t.Fatalf("pendaftar = %d, mau 25", best.Registrations)
	}
}
