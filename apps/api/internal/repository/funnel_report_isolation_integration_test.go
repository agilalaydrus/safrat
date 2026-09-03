package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Two travel agencies, one table.
//
// The test is deliberately two-way: it is not enough that an agency sees its
// own funnel, it must also see nothing of the other's. A one-way test passes
// just as happily against a query with no operator filter at all, which is the
// exact bug that would leak one agency's marketing performance to a competitor.
//
// It also pins the ordering of the channel list to registrations rather than
// visitors: a channel with a thousand readers and no registrants is not a good
// channel, and sorting by visitors would put it on top.
func TestFunnelReportIsolatesOperatorsIntegration(t *testing.T) {
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
			VALUES ($1,$2,$3,'ID',$4,$5)`, id, tag+"-"+suffix, "Uji "+tag, tag+"-"+suffix+"@example.test", tag+"-"+suffix)
		t.Cleanup(func() {
			bg := context.Background()
			_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id = $1`, id)
		})
		return id
	}

	mine, theirs := newOperator("kami"), newOperator("mereka")
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("zona: %v", err)
	}
	today := time.Now().In(jakarta).Format("2006-01-02")

	// Summaries: instagram brings the crowd, a referral brings the customers.
	exec(`INSERT INTO funnel_daily (operator_id,day,step,utm_source,visitors,events) VALUES
		($1,$2::date,'LANDING','instagram',900,1200),
		($1,$2::date,'LANDING','rujukan',40,50),
		($3,$2::date,'LANDING','instagram',7000,9000)`, mine, today, theirs)

	// Raw rows behind the hourly, regional and article figures.
	hash := func(seed string) string {
		return (uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed)).String() +
			uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed+"z")).String())[:64]
	}
	morning := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 5, 30, 0, 0, jakarta)
	exec(`INSERT INTO funnel_events (operator_id,visitor_hash,step,article_slug,city,province,occurred_at)
		VALUES ($1,$2,'ARTIKEL','visa-umroh-mandiri','Bandung','Jawa Barat',$4),
		       ($1,$2,'SELESAI','','Bandung','Jawa Barat',$4),
		       ($3,$5,'ARTIKEL','rahasia-mereka','Medan','Sumatera Utara',$4)`,
		mine, hash("pembaca"), theirs, morning, hash("mereka"))

	// Registrations carry the channel and the age.
	season := uuid.NewString()
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
		VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',40)`, season, mine)
	exec(`INSERT INTO pilgrim_registrations (operator_id,season_id,full_name,date_of_birth,utm_source)
		VALUES ($1,$2,'Ahmad','1974-01-01','rujukan'),
		       ($1,$2,'Siti','1980-01-01','rujukan')`, mine, season)

	repo := NewFunnelRepository(pool)
	report, err := repo.Report(ctx, mine, 30)
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	if len(report.Sources) == 0 {
		t.Fatal("tidak ada kanal sama sekali")
	}
	if report.Sources[0].Source != "rujukan" {
		t.Fatalf("kanal teratas = %q, mau \"rujukan\" — daftar diurutkan menurut penonton, bukan pendaftar", report.Sources[0].Source)
	}
	if report.Sources[0].Registrations != 2 {
		t.Fatalf("pendaftar rujukan = %d, mau 2", report.Sources[0].Registrations)
	}
	for _, source := range report.Sources {
		if source.Visitors > 1000 {
			t.Fatalf("kanal %q punya %d penonton — angka milik travel lain ikut terbaca", source.Source, source.Visitors)
		}
	}

	var totalLanding int32
	for _, step := range report.Steps {
		if step.Step == "LANDING" {
			totalLanding = step.Visitors
		}
	}
	if totalLanding != 940 {
		t.Fatalf("pengunjung LANDING = %d, mau 940", totalLanding)
	}

	foundHour := false
	for _, hour := range report.Hours {
		if hour.Hour == 5 {
			foundHour = true
		}
	}
	if !foundHour {
		t.Fatal("kunjungan 05:30 WIB tidak muncul di jam 5 — dikelompokkan menurut UTC?")
	}

	for _, place := range report.Places {
		if place.City == "Medan" {
			t.Fatal("daerah milik travel lain terbaca")
		}
	}
	for _, article := range report.Articles {
		if article.Slug == "rahasia-mereka" {
			t.Fatal("artikel milik travel lain terbaca")
		}
		if article.Slug == "visa-umroh-mandiri" && article.Registrations != 1 {
			t.Fatalf("pembaca artikel yang mendaftar = %d, mau 1", article.Registrations)
		}
	}

	// The other side of the boundary: their report must not contain ours.
	otherReport, err := repo.Report(ctx, theirs, 30)
	if err != nil {
		t.Fatalf("report lain: %v", err)
	}
	for _, source := range otherReport.Sources {
		if source.Registrations != 0 {
			t.Fatalf("travel lain melihat %d pendaftar milik kami", source.Registrations)
		}
	}
	for _, article := range otherReport.Articles {
		if article.Slug == "visa-umroh-mandiri" {
			t.Fatal("travel lain membaca artikel kami")
		}
	}
	if len(otherReport.ChannelAges) != 0 {
		t.Fatalf("travel lain melihat usia pendaftar kami: %+v", otherReport.ChannelAges)
	}

	// Age is measured, never inferred: only registrants have one.
	if len(report.ChannelAges) != 1 || report.ChannelAges[0].Sample != 2 {
		t.Fatalf("usia per kanal salah: %+v", report.ChannelAges)
	}
	if report.ChannelAges[0].AverageAge < 30 || report.ChannelAges[0].AverageAge > 80 {
		t.Fatalf("usia rata-rata tidak masuk akal: %v", report.ChannelAges[0].AverageAge)
	}
}
