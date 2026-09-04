package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The export request queue: claiming, idempotency, and the tenant boundary on
// the download link.
func TestDataExportClaimIsSingleUseAndScopedToOperatorIntegration(t *testing.T) {
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
	operatorID, suffix := uuid.NewString(), uuid.NewString()[:8]
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Uji Ekspor','ID',$3,$4)`, operatorID, "ex-"+suffix, "ex-"+suffix+"@example.test", "ex-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	other := uuid.NewString()
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Lain','ID',$3,$4)`, other, "exlain-"+suffix, "exlain-"+suffix+"@example.test", "exlain-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, other) })

	repo := NewDataExportRepository(pool)

	// A retried request with the same key settles the same job.
	key := "req-" + uuid.NewString()
	first, err := repo.Request(ctx, operatorID, "staf-1", key)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	second, err := repo.Request(ctx, operatorID, "staf-1", key)
	if err != nil {
		t.Fatalf("request ulang: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("kunci idempotensi yang sama membuka dua permintaan: %s vs %s", first.ID, second.ID)
	}
	if first.Status != "PENDING" {
		t.Fatalf("status awal = %q, mau PENDING", first.Status)
	}

	// The claim is a lock: it takes the oldest pending job and moves it out of
	// PENDING in the same statement, so a second claim right after finds
	// nothing to do.
	claimed, ok, err := repo.ClaimNext(ctx)
	if err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	if claimed.ID != first.ID {
		t.Fatalf("claim mengambil job lain: %s", claimed.ID)
	}
	if claimed.Status != "PROCESSING" {
		t.Fatalf("status setelah diklaim = %q, mau PROCESSING", claimed.Status)
	}
	_, okAgain, err := repo.ClaimNext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if okAgain {
		t.Fatal("job yang sama diklaim dua kali")
	}

	if err := repo.MarkReady(ctx, claimed.ID, "exports/"+operatorID+"/"+claimed.ID+".zip", 4096, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("mark ready: %v", err)
	}
	ready, err := repo.Get(ctx, operatorID, claimed.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ready.Status != "READY" || ready.SizeBytes != 4096 {
		t.Fatalf("status setelah selesai = %+v", ready)
	}

	// Another operator cannot read this row by id, even knowing it exists.
	if _, err := repo.Get(ctx, other, claimed.ID); err == nil {
		t.Fatal("travel lain bisa membaca ekspor travel ini")
	}
	otherList, err := repo.ListForOperator(ctx, other, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range otherList {
		if row.ID == claimed.ID {
			t.Fatal("ekspor travel ini muncul di daftar travel lain")
		}
	}

	// A second, distinct request for the same operator is a different job.
	third, err := repo.Request(ctx, operatorID, "staf-1", "req-"+uuid.NewString())
	if err != nil {
		t.Fatalf("permintaan kedua: %v", err)
	}
	if third.ID == first.ID {
		t.Fatal("kunci berbeda menghasilkan job yang sama")
	}

	// A failure is recorded, not silently retried forever — MaxRetry(1) on the
	// task itself is what bounds the retry, this just proves the row says why.
	if err := repo.MarkFailed(ctx, third.ID, "gagal diuji dengan sengaja"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	failed, err := repo.Get(ctx, operatorID, third.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "FAILED" || failed.Error == "" {
		t.Fatalf("kegagalan tidak tercatat: %+v", failed)
	}

	// Expiry: a READY export past its expires_at is found by the sweep and
	// flips to FAILED once "deleted" — this test stands in for the storage
	// delete, which the worker test covers with a mock.
	if err := repo.MarkReady(ctx, third.ID, "exports/"+operatorID+"/"+third.ID+".zip", 10, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("mark ready (kedaluwarsa): %v", err)
	}
	expired, err := repo.ListExpired(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range expired {
		if row.ID == third.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("ekspor yang sudah kedaluwarsa tidak muncul di sapuan")
	}
	if err := repo.MarkExpired(ctx, third.ID); err != nil {
		t.Fatalf("mark expired: %v", err)
	}
	afterExpiry, err := repo.Get(ctx, operatorID, third.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterExpiry.Status == "READY" {
		t.Fatal("tautan yang sudah kedaluwarsa masih dilaporkan siap diunduh")
	}
}
