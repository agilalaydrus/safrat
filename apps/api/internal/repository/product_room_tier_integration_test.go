package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Room tiers, and the one thing that makes a quota real: it must be impossible
// to sell the same last seat twice.
func TestRoomTierQuotaCannotBeOversoldIntegration(t *testing.T) {
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
	operatorID, seasonID, productID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	suffix := uuid.NewString()[:8]
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Uji Tier','ID',$3,$4)`, operatorID, "tier-"+suffix, "tier-"+suffix+"@example.test", "tier-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
		VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',80)`, seasonID, operatorID)
	exec(`INSERT INTO products (id,operator_id,season_id,name,price_idr)
		VALUES ($1,$2,$3,'Paket Uji',30000000)`, productID, operatorID, seasonID)

	repo := NewProductRepository(db.New(pool), pool)
	quad, double := int32(0), int32(1)
	saved, basePrice, err := repo.SetRoomTiers(ctx, operatorID, productID, []RoomTier{
		{Tier: "DOUBLE", PriceDeltaIDR: 5_000_000, SeatQuota: &double, IsActive: true},
		{Tier: "QUAD", PriceDeltaIDR: -2_000_000, SeatQuota: nil, IsActive: true},
		{Tier: "TRIPLE", PriceDeltaIDR: 0, SeatQuota: &quad, IsActive: true},
	})
	if err != nil {
		t.Fatalf("set room tiers: %v", err)
	}
	if basePrice != 30_000_000 {
		t.Fatalf("harga dasar = %d", basePrice)
	}
	// Cheapest first, as a person reads a ladder.
	if len(saved) != 3 || saved[0].Tier != "QUAD" || saved[2].Tier != "DOUBLE" {
		t.Fatalf("urutan tier salah: %+v", saved)
	}
	if saved[0].SeatQuota != nil {
		t.Fatal("kuota kosong tersimpan sebagai angka — tanpa batas dan nol tidak boleh sama")
	}
	if saved[1].SeatQuota == nil || *saved[1].SeatQuota != 0 {
		t.Fatalf("kuota nol hilang: %+v", saved[1])
	}

	pilgrims := make([]string, 0, 4)
	for i := range 4 {
		id := uuid.NewString()
		exec(`INSERT INTO pilgrims (id,season_id,operator_id,full_name,passport_number,nationality,date_of_birth,gender)
			VALUES ($1,$2,$3,$4,$5,'ID','1990-01-01'::timestamptz,'MALE')`,
			id, seasonID, operatorID, "Jamaah", "P-"+suffix+"-"+uuid.NewString()[:6])
		pilgrims = append(pilgrims, id)
		_ = i
	}

	order := func(pilgrimID, tier string) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO orders (operator_id,season_id,pilgrim_id,product_id,unit_price_idr,total_price_idr,status,room_tier)
			VALUES ($1,$2,$3,$4,35000000,35000000,'PAID',$5)`,
			operatorID, seasonID, pilgrimID, productID, tier)
		return err
	}

	// The one DOUBLE seat, sold twice at the same moment. Exactly one must win.
	//
	// Both inserts run inside their own open transaction, and that detail is
	// the whole test. In autocommit the two statements simply do not overlap
	// and the race never happens — an earlier version of this test "passed"
	// with the lock removed, proving nothing. Held open, transaction two cannot
	// see transaction one's uncommitted row, so without serialisation both
	// count zero seats taken and both are allowed.
	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = first.Rollback(ctx) }()
	second, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = second.Rollback(ctx) }()

	const insertOrder = `
		INSERT INTO orders (operator_id,season_id,pilgrim_id,product_id,unit_price_idr,total_price_idr,status,room_tier)
		VALUES ($1,$2,$3,$4,35000000,35000000,'PAID',$5)`
	if _, err := first.Exec(ctx, insertOrder, operatorID, seasonID, pilgrims[0], productID, "DOUBLE"); err != nil {
		t.Fatalf("pemesanan pertama ditolak: %v", err)
	}

	// The second insert must not be able to finish before the first commits.
	// Started in the background because the advisory lock is expected to block
	// it — a foreground call would deadlock the test rather than fail it.
	secondResult := make(chan error, 1)
	go func() {
		_, err := second.Exec(ctx, insertOrder, operatorID, seasonID, pilgrims[1], productID, "DOUBLE")
		secondResult <- err
	}()

	select {
	case err := <-secondResult:
		t.Fatalf("pemesanan kedua selesai sebelum yang pertama di-commit (%v) — kursi terakhir terjual dua kali", err)
	case <-time.After(300 * time.Millisecond):
		// Blocked, which is what the lock is for.
	}

	if err := first.Commit(ctx); err != nil {
		t.Fatalf("commit pertama: %v", err)
	}
	select {
	case err := <-secondResult:
		if err == nil {
			t.Fatal("pemesanan kedua berhasil untuk kuota 1 — kursi terakhir terjual dua kali")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pemesanan kedua tidak pernah selesai setelah yang pertama di-commit")
	}
	_ = second.Rollback(ctx)

	// A tier with a quota of zero exists but has nothing to sell.
	if err := order(pilgrims[2], "TRIPLE"); err == nil {
		t.Fatal("tier berkuota nol tetap bisa dipesan")
	}

	// No quota means no limit, not a limit of zero.
	if err := order(pilgrims[2], "QUAD"); err != nil {
		t.Fatalf("tier tanpa batas menolak pemesanan: %v", err)
	}
	if err := order(pilgrims[3], "QUAD"); err != nil {
		t.Fatalf("tier tanpa batas menolak pemesanan kedua: %v", err)
	}

	// Seats taken is read from the same definition the trigger enforces.
	tiers, _, err := repo.ListRoomTiers(ctx, operatorID, productID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, tier := range tiers {
		switch tier.Tier {
		case "DOUBLE":
			if tier.SeatsTaken != 1 {
				t.Fatalf("DOUBLE terisi %d, mau 1", tier.SeatsTaken)
			}
		case "QUAD":
			if tier.SeatsTaken != 2 {
				t.Fatalf("QUAD terisi %d, mau 2", tier.SeatsTaken)
			}
		}
	}

	// A tier somebody has already bought cannot be removed.
	if _, _, err := repo.SetRoomTiers(ctx, operatorID, productID, []RoomTier{
		{Tier: "QUAD", PriceDeltaIDR: -2_000_000, IsActive: true},
	}); err == nil {
		t.Fatal("tier yang sudah terjual bisa dihapus — pesanannya akan menunjuk tier yang tidak ada")
	}

	// Nor can its quota be set below what is already sold.
	tooSmall := int32(0)
	if _, _, err := repo.SetRoomTiers(ctx, operatorID, productID, []RoomTier{
		{Tier: "QUAD", PriceDeltaIDR: -2_000_000, IsActive: true},
		{Tier: "TRIPLE", PriceDeltaIDR: 0, IsActive: true},
		{Tier: "DOUBLE", PriceDeltaIDR: 5_000_000, SeatQuota: &tooSmall, IsActive: true},
	}); err == nil {
		t.Fatal("kuota bisa disetel di bawah yang sudah terjual — layar akan bilang ada kursi, pemicu akan menolak semuanya")
	}

	// A price that would go negative is a typo, not a discount.
	if _, _, err := repo.SetRoomTiers(ctx, operatorID, productID, []RoomTier{
		{Tier: "QUAD", PriceDeltaIDR: -40_000_000, IsActive: true},
	}); err == nil {
		t.Fatal("harga tier boleh jatuh di bawah nol")
	}

	// Another operator sees nothing of this, and cannot write to it.
	other := uuid.NewString()
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Lain','ID',$3,$4)`, other, "lain-"+suffix, "lain-"+suffix+"@example.test", "lain-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, other) })
	if _, _, err := repo.ListRoomTiers(ctx, other, productID); err == nil {
		t.Fatal("travel lain bisa membaca tier paket ini")
	}
	if _, _, err := repo.SetRoomTiers(ctx, other, productID, []RoomTier{
		{Tier: "QUAD", PriceDeltaIDR: 0, IsActive: true},
	}); err == nil {
		t.Fatal("travel lain bisa menulis tier paket ini")
	}
	_ = apperror.ErrValidation
}
