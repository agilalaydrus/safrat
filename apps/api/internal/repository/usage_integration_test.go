package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The usage snapshot, and the two properties that make it trustworthy.
//
// It has to count the same things the entitlement trigger counts — a usage
// figure that disagrees with the limit it is shown against sends somebody
// chasing a discrepancy that does not exist. And it has to be safe to run
// twice, because a daily worker that duplicates rows is a daily worker that
// eventually reports double.
func TestUsageSnapshotMatchesEntitlementCountsAndIsIdempotentIntegration(t *testing.T) {
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

	operatorID, seasonID := uuid.NewString(), uuid.NewString()
	suffix := uuid.NewString()[:8]
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	// PRO so branches are allowed at all; STARTER forbids them outright.
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan)
	      VALUES ($1,$2,'Usage Uji','ID',$3,$4,'PRO')`,
		operatorID, "usage-"+suffix, "usage-"+suffix+"@example.test", "usage-"+suffix)
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
	      VALUES ($1,$2,'Musim Usage','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',50)`, seasonID, operatorID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID)
	})

	addPilgrim := func(name string, substituted bool) {
		t.Helper()
		exec(`INSERT INTO pilgrims (id,season_id,operator_id,full_name,passport_number,nationality,date_of_birth,gender,is_substituted)
		      VALUES ($1,$2,$3,$4,$5,'ID','1990-01-01','MALE',$6)`,
			uuid.NewString(), seasonID, operatorID, name, "USG-"+uuid.NewString()[:10], substituted)
	}
	addPilgrim("Aktif Satu", false)
	addPilgrim("Aktif Dua", false)
	// Substituted pilgrims are excluded by the entitlement trigger, so they must
	// be excluded here too — otherwise usage reads above a limit that is not
	// actually being consumed.
	addPilgrim("Digantikan", true)
	exec(`INSERT INTO branches (operator_id,name,city) VALUES ($1,'Bandung','Bandung'),($2,'Medan','Medan')`, operatorID, operatorID)

	repo := NewSubscriptionRepository(pool)
	if _, err := repo.RecomputeUsage(ctx); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	value := func(metric string) int64 {
		t.Helper()
		var got int64
		if err := pool.QueryRow(ctx, `SELECT value FROM usage_counters
			WHERE operator_id = $1 AND metric = $2 AND period_start = CURRENT_DATE`, operatorID, metric).Scan(&got); err != nil {
			t.Fatalf("baca %s: %v", metric, err)
		}
		return got
	}
	if got := value("pilgrims"); got != 2 {
		t.Fatalf("jamaah = %d, mau 2 — yang digantikan seharusnya tidak dihitung", got)
	}
	if got := value("branches"); got != 2 {
		t.Fatalf("cabang = %d, mau 2", got)
	}
	if got := value("storage_bytes"); got != 0 {
		t.Fatalf("penyimpanan = %d, mau 0", got)
	}

	// Running again overwrites rather than duplicating, and picks up a change.
	addPilgrim("Aktif Tiga", false)
	if _, err := repo.RecomputeUsage(ctx); err != nil {
		t.Fatalf("recompute kedua: %v", err)
	}
	var rowCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM usage_counters
		WHERE operator_id = $1 AND metric = 'pilgrims'`, operatorID).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if rowCount != 1 {
		t.Fatalf("%d baris untuk satu metrik satu hari — jalan dua kali menggandakan", rowCount)
	}
	if got := value("pilgrims"); got != 3 {
		t.Fatalf("jamaah setelah recompute = %d, mau 3", got)
	}

	// The listing must carry the limit each figure is measured against, and
	// must tell "unlimited" apart from "zero".
	rows, err := repo.ListUsage(ctx)
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	var seen int
	for _, row := range rows {
		if row.OperatorID != operatorID {
			continue
		}
		seen++
		if row.Metric == "pilgrims" && row.Limit != nil {
			t.Fatalf("batas jamaah PRO seharusnya tanpa batas, dapat %d", *row.Limit)
		}
		if row.Metric == "storage_bytes" && row.Limit != nil {
			t.Fatalf("penyimpanan belum punya batas, tapi dilaporkan %d", *row.Limit)
		}
	}
	if seen != 3 {
		t.Fatalf("%d metrik terdaftar, mau 3", seen)
	}
}
