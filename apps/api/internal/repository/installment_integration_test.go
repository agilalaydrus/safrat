package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInstallmentRepositoryEncapsulatesMoneyAndBranchScopeIntegration(t *testing.T) {
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

	op, season := uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	pilgrims := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	head := "installment-branch-head-" + uuid.NewString()
	staff := "installment-staff-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Installment scope','ID',$3,$4,'GROWTH')`, op, "installment-scope-"+uuid.NewString(), op[:8]+"@example.test", "installment-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '365 days',20)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	for index, pilgrim := range pilgrims {
		var branch any
		var email any
		switch index {
		case 0:
			branch = bandung
			email = "bandung-" + uuid.NewString() + "@example.test"
		case 1:
			branch = medan
			email = "medan-" + uuid.NewString() + "@example.test"
		default:
			branch = nil
			email = nil
		}
		exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,full_name,passport_number,nationality,date_of_birth,gender,email) VALUES ($1,$2,$3,$4,$5,$6,'ID','1990-01-01','MALE',$7)`, pilgrim, season, op, branch, "Jamaah "+string(rune('A'+index)), "CIC-"+uuid.NewString(), email)
	}

	repo := NewInstallmentRepository(pool)
	firstDue := time.Date(2027, time.January, 31, 0, 0, 0, 0, time.UTC)
	dpSchedule := []domain.InstallmentScheduleDraft{
		{Number: 1, Label: "DP 50%", DueDate: firstDue, AmountDueIDR: 50_001},
		{Number: 2, Label: "Pelunasan", DueDate: time.Date(2027, time.February, 28, 0, 0, 0, 0, time.UTC), AmountDueIDR: 50_000},
	}
	draft := domain.InstallmentPlanDraft{
		PilgrimID: pilgrims[0], Scheme: "DP_50", GrossAmountIDR: 100_001,
		FirstDueDate: firstDue, IdempotencyKey: "plan-bandung-" + uuid.NewString(),
	}
	bandungCtx := ContextWithStaffActor(ctx, head)
	bandungPlan, created, err := repo.CreatePlan(bandungCtx, op, staff, draft, dpSchedule)
	if err != nil || !created || len(bandungPlan.Installments) != 2 || bandungPlan.Plan.PayableAmountIDR != 100_001 {
		t.Fatalf("buat rencana Bandung: created=%v detail=%#v err=%v", created, bandungPlan, err)
	}
	replay, created, err := repo.CreatePlan(bandungCtx, op, staff, draft, dpSchedule)
	if err != nil || created || replay.Plan.ID != bandungPlan.Plan.ID {
		t.Fatalf("replay rencana membuat fakta baru: created=%v detail=%#v err=%v", created, replay, err)
	}
	conflicting := draft
	conflicting.GrossAmountIDR++
	if _, _, err := repo.CreatePlan(bandungCtx, op, staff, conflicting, dpSchedule); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("idempotency key menerima payload berbeda: %v", err)
	}
	conflictingSchedule := append([]domain.InstallmentScheduleDraft(nil), dpSchedule...)
	conflictingSchedule[0].AmountDueIDR--
	conflictingSchedule[1].AmountDueIDR++
	if _, _, err := repo.CreatePlan(bandungCtx, op, staff, draft, conflictingSchedule); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("idempotency key menerima jadwal berbeda: %v", err)
	}

	medanDraft := domain.InstallmentPlanDraft{PilgrimID: pilgrims[1], Scheme: "FULL", GrossAmountIDR: 80_000, FirstDueDate: firstDue, IdempotencyKey: "plan-medan-" + uuid.NewString()}
	medanPlan, _, err := repo.CreatePlan(ctx, op, staff, medanDraft, []domain.InstallmentScheduleDraft{{Number: 1, Label: "Pelunasan", DueDate: firstDue, AmountDueIDR: 80_000}})
	if err != nil {
		t.Fatalf("buat rencana Medan: %v", err)
	}
	if _, err := repo.GetPlanByPilgrim(bandungCtx, op, pilgrims[1]); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca piutang Medan: %v", err)
	}
	if _, _, err := repo.RecordPayment(bandungCtx, op, head, medanPlan.Installments[0].ID, 1, "CASH", "", "", "cross-branch-"+uuid.NewString()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membayar angsuran Medan: %v", err)
	}
	medanPayment, _, err := repo.RecordPayment(ctx, op, staff, medanPlan.Installments[0].ID, 1, "CASH", "", "", "medan-payment-"+uuid.NewString())
	if err != nil {
		t.Fatalf("catat pembayaran Medan: %v", err)
	}
	if _, err := repo.QueueReceipt(bandungCtx, op, medanPayment.ID, "cross-receipt-"+uuid.NewString()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengantre kwitansi Medan: %v", err)
	}
	branchRows, err := repo.ListReceivables(bandungCtx, op, domain.InstallmentReceivableFilter{SeasonID: season})
	if err != nil || len(branchRows.Plans) != 1 || branchRows.Plans[0].PilgrimID != pilgrims[0] {
		t.Fatalf("buku piutang Bandung bocor: %#v (%v)", branchRows, err)
	}
	emptyPage, err := repo.ListReceivables(bandungCtx, op, domain.InstallmentReceivableFilter{SeasonID: season, Limit: 10, Offset: 100})
	if err != nil || len(emptyPage.Plans) != 0 || emptyPage.TotalCount != 1 {
		t.Fatalf("total pagination hilang pada halaman kosong: %#v (%v)", emptyPage, err)
	}

	firstInstallment := bandungPlan.Installments[0]
	paymentKey := "payment-bandung-" + uuid.NewString()
	payment, created, err := repo.RecordPayment(bandungCtx, op, head, firstInstallment.ID, 30_000, "BANK_TRANSFER", "MUT-001", "DP diterima", paymentKey)
	if err != nil || !created || payment.AmountIDR != 30_000 || payment.ReceiptNumber == "" {
		t.Fatalf("catat pembayaran: created=%v payment=%#v err=%v", created, payment, err)
	}
	if _, created, err := repo.RecordPayment(bandungCtx, op, head, firstInstallment.ID, 30_000, "BANK_TRANSFER", "MUT-001", "DP diterima", paymentKey); err != nil || created {
		t.Fatalf("replay pembayaran membuat entry kedua: created=%v err=%v", created, err)
	}
	if _, _, err := repo.RecordPayment(bandungCtx, op, head, firstInstallment.ID, 30_001, "BANK_TRANSFER", "MUT-001", "DP diterima", paymentKey); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("idempotency pembayaran menerima nominal berbeda: %v", err)
	}
	if _, _, err := repo.RecordPayment(bandungCtx, op, head, firstInstallment.ID, 30_000, "CASH", "", "overpay", "overpay-"+uuid.NewString()); !errors.Is(err, apperror.ErrFailedPrecondition) {
		t.Fatalf("overpayment diterima: %v", err)
	}
	partialBook, err := repo.ListReceivables(bandungCtx, op, domain.InstallmentReceivableFilter{SeasonID: season})
	if err != nil || partialBook.TotalReceivableIDR != 70_001 || partialBook.AgingCurrentIDR != 70_001 {
		t.Fatalf("aging piutang Bandung salah setelah pembayaran: %#v (%v)", partialBook, err)
	}
	cashSummary, err := NewCashFlowRepository(db.New(pool)).GetSummary(bandungCtx, op, season)
	if err != nil || cashSummary.TotalCollectedIDR != 30_000 {
		t.Fatalf("kas masuk tidak menjumlah ledger cicilan: %#v (%v)", cashSummary, err)
	}
	receiptKey := "receipt-bandung-" + uuid.NewString()
	if created, err := repo.QueueReceipt(bandungCtx, op, payment.ID, receiptKey); err != nil || !created {
		t.Fatalf("antrekan kwitansi: created=%v err=%v", created, err)
	}
	if created, err := repo.QueueReceipt(bandungCtx, op, payment.ID, receiptKey); err != nil || created {
		t.Fatalf("replay kwitansi membuat event kedua: created=%v err=%v", created, err)
	}
	reminderKey := "reminder-bandung-" + uuid.NewString()
	queued, skipped, err := repo.QueueReminders(bandungCtx, op, season, []string{bandungPlan.Plan.ID}, false, reminderKey)
	if err != nil || queued != 1 || skipped != 0 {
		t.Fatalf("antrekan pengingat: queued=%d skipped=%d err=%v", queued, skipped, err)
	}
	queued, skipped, err = repo.QueueReminders(bandungCtx, op, season, []string{bandungPlan.Plan.ID}, false, reminderKey)
	if err != nil || queued != 0 || skipped != 1 {
		t.Fatalf("replay pengingat membuat event kedua: queued=%d skipped=%d err=%v", queued, skipped, err)
	}
	if _, _, err := repo.QueueReminders(bandungCtx, op, season, []string{medanPlan.Plan.ID}, false, "cross-reminder-"+uuid.NewString()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengantre pengingat Medan: %v", err)
	}
	assertPilgrimPaymentStatus(t, pool, pilgrims[0], "DP")

	reversalKey := "reversal-bandung-" + uuid.NewString()
	reversal, created, err := repo.ReversePayment(bandungCtx, op, head, payment.ID, "Mutasi bank salah pilih", reversalKey)
	if err != nil || !created || reversal.AmountIDR != -30_000 || reversal.OriginalPaymentID != payment.ID {
		t.Fatalf("buat reversal: created=%v reversal=%#v err=%v", created, reversal, err)
	}
	if _, created, err := repo.ReversePayment(bandungCtx, op, head, payment.ID, "Mutasi bank salah pilih", reversalKey); err != nil || created {
		t.Fatalf("replay reversal membuat entry kedua: created=%v err=%v", created, err)
	}
	if _, _, err := repo.ReversePayment(bandungCtx, op, head, payment.ID, "Pembalikan kedua", "second-reversal-"+uuid.NewString()); !errors.Is(err, apperror.ErrAlreadyExists) {
		t.Fatalf("pembayaran dapat dibalik dua kali: %v", err)
	}
	if _, err := repo.QueueReceipt(bandungCtx, op, payment.ID, "void-receipt-"+uuid.NewString()); !errors.Is(err, apperror.ErrFailedPrecondition) {
		t.Fatalf("kwitansi positif dapat dikirim setelah pembayaran dibalik: %v", err)
	}
	assertPilgrimPaymentStatus(t, pool, pilgrims[0], "UNPAID")

	fullFirst, _, err := repo.RecordPayment(bandungCtx, op, head, firstInstallment.ID, 50_001, "CASH", "", "Pelunasan DP", "full-first-"+uuid.NewString())
	if err != nil || fullFirst == nil {
		t.Fatalf("lunasi angsuran pertama: %v", err)
	}
	if _, _, err := repo.RecordPayment(bandungCtx, op, head, bandungPlan.Installments[1].ID, 50_000, "CASH", "", "Pelunasan", "full-second-"+uuid.NewString()); err != nil {
		t.Fatalf("lunasi angsuran kedua: %v", err)
	}
	assertPilgrimPaymentStatus(t, pool, pilgrims[0], "PAID")
	paidDetail, err := repo.GetPlanByPilgrim(bandungCtx, op, pilgrims[0])
	if err != nil || paidDetail.Plan.Status != "PAID" || paidDetail.Plan.OutstandingAmountIDR != 0 {
		t.Fatalf("proyeksi pelunasan salah: %#v (%v)", paidDetail, err)
	}

	// Direct edits and deletes are refused even outside repository code.
	if _, err := pool.Exec(ctx, `UPDATE installment_payment_entries SET amount_idr=1 WHERE id=$1`, fullFirst.ID); err == nil {
		t.Fatal("entry pembayaran dapat diedit langsung")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM installment_payment_entries WHERE id=$1`, fullFirst.ID); err == nil {
		t.Fatal("entry pembayaran dapat dihapus langsung")
	}
	if _, err := pool.Exec(ctx, `UPDATE installments SET amount_due_idr=1 WHERE id=$1`, firstInstallment.ID); err == nil {
		t.Fatal("jadwal angsuran dapat diedit langsung")
	}
	if _, err := pool.Exec(ctx, `UPDATE pilgrims SET payment_status='UNPAID' WHERE id=$1`, pilgrims[0]); err == nil {
		t.Fatal("status turunan ledger dapat ditimpa endpoint lama")
	}

	// Two concurrent payments against Rp50.000 cannot both commit Rp30.000.
	concurrentDraft := domain.InstallmentPlanDraft{PilgrimID: pilgrims[2], Scheme: "FULL", GrossAmountIDR: 50_000, FirstDueDate: firstDue, IdempotencyKey: "plan-race-" + uuid.NewString()}
	concurrentPlan, _, err := repo.CreatePlan(ctx, op, staff, concurrentDraft, []domain.InstallmentScheduleDraft{{Number: 1, Label: "Pelunasan", DueDate: firstDue, AmountDueIDR: 50_000}})
	if err != nil {
		t.Fatalf("buat rencana konkurensi: %v", err)
	}
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for index := 0; index < 2; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, _, err := repo.RecordPayment(ctx, op, staff, concurrentPlan.Installments[0].ID, 30_000, "CASH", "", "race", "race-payment-"+uuid.NewString())
			errCh <- err
		}(index)
	}
	wg.Wait()
	close(errCh)
	var successes, refused int
	for err := range errCh {
		if err == nil {
			successes++
		} else if errors.Is(err, apperror.ErrFailedPrecondition) {
			refused++
		} else {
			t.Fatalf("error konkurensi tak terduga: %v", err)
		}
	}
	if successes != 1 || refused != 1 {
		t.Fatalf("hasil konkurensi: sukses=%d ditolak=%d", successes, refused)
	}
	queued, skipped, err = repo.QueueReminders(ctx, op, season, []string{concurrentPlan.Plan.ID}, false, "missing-email-"+uuid.NewString())
	if err != nil || queued != 0 || skipped != 1 {
		t.Fatalf("pengingat tanpa email tidak dilewati dengan jelas: queued=%d skipped=%d err=%v", queued, skipped, err)
	}

	// Deferred database constraint rejects an incomplete or unbalanced schedule.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin jadwal invalid: %v", err)
	}
	badPlan := uuid.NewString()
	if _, err := tx.Exec(ctx, `INSERT INTO installment_plans (id,operator_id,season_id,pilgrim_id,scheme,gross_amount_idr,first_due_date,created_by_user_id,idempotency_key) VALUES ($1,$2,$3,$4,'FULL',100,'2027-01-31',$5,$6)`, badPlan, op, season, pilgrims[3], staff, "bad-plan-"+uuid.NewString()); err != nil {
		t.Fatalf("insert rencana invalid: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO installments (plan_id,operator_id,installment_number,label,due_date,amount_due_idr) VALUES ($1,$2,1,'Salah','2027-01-31',99)`, badPlan, op); err != nil {
		t.Fatalf("insert jadwal invalid: %v", err)
	}
	if err := tx.Commit(ctx); err == nil {
		t.Fatal("jadwal yang kehilangan Rp1 berhasil commit")
	}

	// Entitlement is enforced again in PostgreSQL, not only by the service.
	exec(`INSERT INTO plan_overrides (operator_id,feature_flag_overrides,note) VALUES ($1,'{"installments":false}','test') ON CONFLICT (operator_id) DO UPDATE SET feature_flag_overrides=EXCLUDED.feature_flag_overrides`, op)
	disabledDraft := domain.InstallmentPlanDraft{PilgrimID: pilgrims[4], Scheme: "FULL", GrossAmountIDR: 10_000, FirstDueDate: firstDue, IdempotencyKey: "disabled-" + uuid.NewString()}
	if _, _, err := repo.CreatePlan(ctx, op, staff, disabledDraft, []domain.InstallmentScheduleDraft{{Number: 1, Label: "Pelunasan", DueDate: firstDue, AmountDueIDR: 10_000}}); !errors.Is(err, apperror.ErrFailedPrecondition) {
		t.Fatalf("database mengizinkan cicilan saat entitlement mati: %v", err)
	}
}

func assertPilgrimPaymentStatus(t *testing.T, pool *pgxpool.Pool, pilgrimID, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(context.Background(), `SELECT payment_status FROM pilgrims WHERE id=$1`, pilgrimID).Scan(&got); err != nil {
		t.Fatalf("baca status pembayaran jamaah: %v", err)
	}
	if got != want {
		t.Fatalf("status pembayaran = %q, mau %q", got, want)
	}
}
