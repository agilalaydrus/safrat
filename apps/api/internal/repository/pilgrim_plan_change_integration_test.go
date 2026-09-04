package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pindah paket, and the two outcomes that must never be confused: money owed
// back to the pilgrim, and money still owed to the travel agency.
func TestChangePlanRecordsOverpaymentAndShortfallCorrectlyIntegration(t *testing.T) {
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
	operatorID, seasonID := uuid.NewString(), uuid.NewString()
	suffix := uuid.NewString()[:8]
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Uji Pindah Paket','ID',$3,$4)`, operatorID, "pp-"+suffix, "pp-"+suffix+"@example.test", "pp-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
		VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',60)`, seasonID, operatorID)

	cheapProduct, expensiveProduct := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO products (id,operator_id,season_id,name,price_idr) VALUES ($1,$2,$3,'Paket Hemat',20000000)`,
		cheapProduct, operatorID, seasonID)
	exec(`INSERT INTO products (id,operator_id,season_id,name,price_idr) VALUES ($1,$2,$3,'Paket Utama',35000000)`,
		expensiveProduct, operatorID, seasonID)

	newPilgrim := func(name string) string {
		id := uuid.NewString()
		exec(`INSERT INTO pilgrims (id,season_id,operator_id,full_name,passport_number,nationality,date_of_birth,gender)
			VALUES ($1,$2,$3,$4,$5,'ID','1990-01-01'::timestamptz,'MALE')`,
			id, seasonID, operatorID, name, "P-"+uuid.NewString()[:8])
		return id
	}
	newOrder := func(pilgrimID, productID string, total int64) string {
		var orderID string
		if err := pool.QueryRow(ctx, `
			INSERT INTO orders (operator_id,season_id,pilgrim_id,product_id,unit_price_idr,total_price_idr,
			  paid_amount_idr,status,paid_at)
			VALUES ($1,$2,$3,$4,$5,$5,$5,'PAID',NOW()) RETURNING id::text`,
			operatorID, seasonID, pilgrimID, productID, total).Scan(&orderID); err != nil {
			t.Fatalf("fixture pesanan: %v", err)
		}
		return orderID
	}

	repo := NewPlanChangeRepository(pool)

	// Downgrading: the pilgrim already paid for the pricier package and moves
	// to the cheaper one. The difference must become an open credit, not
	// disappear.
	downgradePilgrim := newPilgrim("Turun Paket")
	downgradeOrder := newOrder(downgradePilgrim, expensiveProduct, 35_000_000)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.LockOrderForPlanChange(ctx, tx, operatorID, downgradeOrder)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	if snapshot.PaidAmountIDR != 35_000_000 || snapshot.OldTotalIDR != 35_000_000 {
		t.Fatalf("snapshot salah: %+v", snapshot)
	}
	result, err := repo.ChangePlan(ctx, tx, snapshot, ChangePlanInput{
		OperatorID: operatorID, OrderID: downgradeOrder, ToProductID: cheapProduct, ToProductName: "Paket Hemat",
		NewTotalIDR: 20_000_000, NewUnitIDR: 20_000_000, NewBaseIDR: 20_000_000,
		Reason: "jamaah minta turun ke paket lebih murah", ActorUserID: "uji-staf",
		IdempotencyKey: "pc-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("change plan (turun): %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if result.OverpaymentIDR != 15_000_000 {
		t.Fatalf("kelebihan bayar = %d, mau 15.000.000", result.OverpaymentIDR)
	}
	if result.ShortfallIDR != 0 {
		t.Fatalf("turun paket seharusnya tidak punya kekurangan bayar: %d", result.ShortfallIDR)
	}
	if result.CreditID == "" {
		t.Fatal("kredit tidak dibuat untuk kelebihan bayar")
	}

	// The order itself must now describe the package the pilgrim is on, and
	// paid_amount_idr — what actually moved — must be untouched.
	var productAfter string
	var paidAfter, totalAfter int64
	if err := pool.QueryRow(ctx, `SELECT product_id::text, paid_amount_idr, total_price_idr FROM orders WHERE id = $1`,
		downgradeOrder).Scan(&productAfter, &paidAfter, &totalAfter); err != nil {
		t.Fatal(err)
	}
	if productAfter != cheapProduct || totalAfter != 20_000_000 {
		t.Fatalf("pesanan tidak mencerminkan paket baru: produk=%s total=%d", productAfter, totalAfter)
	}
	if paidAfter != 35_000_000 {
		t.Fatalf("paid_amount_idr berubah dari 35.000.000 menjadi %d — uang yang benar-benar diterima diedit", paidAfter)
	}

	var creditAmount int64
	var creditStatus string
	if err := pool.QueryRow(ctx, `SELECT amount_idr, status FROM pilgrim_credits WHERE id = $1`, result.CreditID).
		Scan(&creditAmount, &creditStatus); err != nil {
		t.Fatal(err)
	}
	if creditAmount != 15_000_000 || creditStatus != "OPEN" {
		t.Fatalf("kredit salah: %d %s", creditAmount, creditStatus)
	}

	// Upgrading: the pilgrim owes more, and that must be reported, not silently
	// absorbed or turned into a credit going the wrong direction.
	upgradePilgrim := newPilgrim("Naik Paket")
	upgradeOrder := newOrder(upgradePilgrim, cheapProduct, 20_000_000)
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshot2, err := repo.LockOrderForPlanChange(ctx, tx2, operatorID, upgradeOrder)
	if err != nil {
		t.Fatalf("lock (naik): %v", err)
	}
	result2, err := repo.ChangePlan(ctx, tx2, snapshot2, ChangePlanInput{
		OperatorID: operatorID, OrderID: upgradeOrder, ToProductID: expensiveProduct, ToProductName: "Paket Utama",
		NewTotalIDR: 35_000_000, NewUnitIDR: 35_000_000, NewBaseIDR: 35_000_000,
		Reason: "jamaah minta naik ke paket utama", ActorUserID: "uji-staf",
		IdempotencyKey: "pc-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("change plan (naik): %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if result2.ShortfallIDR != 15_000_000 {
		t.Fatalf("kekurangan bayar = %d, mau 15.000.000", result2.ShortfallIDR)
	}
	if result2.OverpaymentIDR != 0 || result2.CreditID != "" {
		t.Fatal("naik paket keliru membuat kredit")
	}

	// A PAID order marked paid by hand never records paid_amount_idr at all —
	// MarkOrderPaidManually asserts the total was settled without capturing the
	// exact bank figure. That NULL must fall back to the order's total, not to
	// zero, or every cash sale would look like a total loss the moment its
	// pilgrim moves to a different package.
	manualPilgrim := newPilgrim("Bayar Manual")
	var manualOrder string
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (operator_id,season_id,pilgrim_id,product_id,unit_price_idr,total_price_idr,status,paid_at)
		VALUES ($1,$2,$3,$4,$5,$5,'PAID',NOW()) RETURNING id::text`,
		operatorID, seasonID, manualPilgrim, expensiveProduct, int64(35_000_000)).Scan(&manualOrder); err != nil {
		t.Fatalf("fixture pesanan manual: %v", err)
	}
	var paidIsNull bool
	if err := pool.QueryRow(ctx, `SELECT paid_amount_idr IS NULL FROM orders WHERE id = $1`, manualOrder).Scan(&paidIsNull); err != nil {
		t.Fatal(err)
	}
	if !paidIsNull {
		t.Fatal("fixture ini seharusnya meniru paid_amount_idr yang belum tercatat")
	}
	txManual, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = txManual.Rollback(ctx) }()
	manualSnapshot, err := repo.LockOrderForPlanChange(ctx, txManual, operatorID, manualOrder)
	if err != nil {
		t.Fatalf("lock (manual): %v", err)
	}
	if manualSnapshot.PaidAmountIDR != 35_000_000 {
		t.Fatalf("paid_amount_idr NULL dibaca sebagai %d, mau turun ke total 35.000.000", manualSnapshot.PaidAmountIDR)
	}
	manualResult, err := repo.ChangePlan(ctx, txManual, manualSnapshot, ChangePlanInput{
		OperatorID: operatorID, OrderID: manualOrder, ToProductID: cheapProduct, ToProductName: "Paket Hemat",
		NewTotalIDR: 20_000_000, NewUnitIDR: 20_000_000, NewBaseIDR: 20_000_000,
		Reason: "uji pembayaran manual tanpa paid_amount_idr", ActorUserID: "uji-staf",
		IdempotencyKey: "pc-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("change plan (manual): %v", err)
	}
	if err := txManual.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if manualResult.OverpaymentIDR != 15_000_000 {
		t.Fatalf("kelebihan bayar dari pesanan manual = %d, mau 15.000.000 — paid_amount_idr NULL salah dibaca sebagai nol", manualResult.OverpaymentIDR)
	}

	// The history is readable back, for both pilgrims.
	changes, err := repo.ListForPilgrim(ctx, operatorID, downgradePilgrim, 10)
	if err != nil {
		t.Fatalf("riwayat: %v", err)
	}
	if len(changes) != 1 || changes[0].OverpaymentIDR != 15_000_000 {
		t.Fatalf("riwayat salah: %+v", changes)
	}

	// A retried request with the same idempotency key must not create a
	// second credit for the same event.
	tx3, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx3.Rollback(ctx) }()
	snapshot3, err := repo.LockOrderForPlanChange(ctx, tx3, operatorID, upgradeOrder)
	if err != nil {
		t.Fatalf("lock ulang: %v", err)
	}
	sameKey := "pc-retry-" + uuid.NewString()
	if _, err := repo.ChangePlan(ctx, tx3, snapshot3, ChangePlanInput{
		OperatorID: operatorID, OrderID: upgradeOrder, ToProductID: cheapProduct, ToProductName: "Paket Hemat",
		NewTotalIDR: 20_000_000, NewUnitIDR: 20_000_000, NewBaseIDR: 20_000_000,
		Reason: "percobaan pertama", ActorUserID: "uji-staf", IdempotencyKey: sameKey,
	}); err != nil {
		t.Fatalf("percobaan pertama: %v", err)
	}
	if err := tx3.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx4, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx4.Rollback(ctx) }()
	snapshot4, err := repo.LockOrderForPlanChange(ctx, tx4, operatorID, upgradeOrder)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ChangePlan(ctx, tx4, snapshot4, ChangePlanInput{
		OperatorID: operatorID, OrderID: upgradeOrder, ToProductID: expensiveProduct, ToProductName: "Paket Utama",
		NewTotalIDR: 35_000_000, NewUnitIDR: 35_000_000, NewBaseIDR: 35_000_000,
		Reason: "percobaan kedua dengan kunci yang sama", ActorUserID: "uji-staf", IdempotencyKey: sameKey,
	}); err == nil {
		t.Fatal("kunci idempotensi yang sama diterima dua kali")
	}

	// Resolving a credit twice must not succeed twice.
	if err := repo.ResolveCredit(ctx, operatorID, result.CreditID, "REFUNDED", "", "dikembalikan tunai", "uji-staf"); err != nil {
		t.Fatalf("selesaikan kredit: %v", err)
	}
	if err := repo.ResolveCredit(ctx, operatorID, result.CreditID, "REFUNDED", "", "dikembalikan lagi", "uji-staf"); err == nil {
		t.Fatal("kredit yang sudah selesai bisa diselesaikan lagi")
	}
}
