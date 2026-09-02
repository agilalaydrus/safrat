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
	"github.com/jackc/pgx/v5/pgxpool"
)

func subscriptionTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newTestOperator(t *testing.T, pool *pgxpool.Pool, plan string) string {
	t.Helper()
	operatorID := uuid.NewString()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan) VALUES ($1, $2, 'Sub Test', 'ID', $3, $4, $5::plan)`,
		operatorID, "sub-"+uuid.NewString(), operatorID[:8]+"@example.com", "sub-"+operatorID[:8], plan)
	if err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })
	return operatorID
}

func TestSubscriptionTrialAndAccessIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")

	if err := subscriptions.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	access, err := subscriptions.GetAccess(ctx, operatorID)
	if err != nil || !access.Allowed || access.Status != "TRIALING" {
		t.Fatalf("trial access = %+v, err %v", access, err)
	}
	if remaining := time.Until(access.AccessUntil); remaining > (TrialDays+1)*24*time.Hour || remaining < 0 {
		t.Fatalf("trial runs until %v, which is not %d days out", access.AccessUntil, TrialDays)
	}

	// Calling again must not restart the trial — that would hand out unlimited
	// free time to anyone who triggers it.
	before := access.AccessUntil
	if err := subscriptions.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	again, _ := subscriptions.GetAccess(ctx, operatorID)
	if !again.AccessUntil.Equal(before) {
		t.Fatalf("trial restarted: %v -> %v", before, again.AccessUntil)
	}

	// Access is decided by time, so an expired subscription is refused even
	// while its status still reads TRIALING.
	if _, err := pool.Exec(ctx, `UPDATE subscriptions SET access_until = NOW() - INTERVAL '1 hour' WHERE operator_id = $1`, operatorID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	expired, _ := subscriptions.GetAccess(ctx, operatorID)
	if expired.Allowed {
		t.Fatal("expired subscription still allowed access")
	}
}

func TestSubscriptionBankTransferAmountsNeverCollideIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)

	// Many operators requesting the same plan at the same moment is exactly
	// when a check-then-insert would hand two of them the same amount, and a
	// bank mutation would then credit the wrong travel agency.
	const concurrent = 40
	operators := make([]string, concurrent)
	for i := range operators {
		operators[i] = newTestOperator(t, pool, "STARTER")
	}

	var wg sync.WaitGroup
	amounts := make([]int64, concurrent)
	failures := make([]error, concurrent)
	for i, operatorID := range operators {
		wg.Add(1)
		go func(index int, id string) {
			defer wg.Done()
			invoice, err := subscriptions.IssueBankTransferInvoice(ctx, id, "STARTER")
			amounts[index], failures[index] = invoice.Amount, err
		}(i, operatorID)
	}
	wg.Wait()

	seen := map[int64]bool{}
	for i, amount := range amounts {
		if failures[i] != nil {
			t.Fatalf("operator %d: %v", i, failures[i])
		}
		if seen[amount] {
			t.Fatalf("amount %d issued twice — a bank mutation could not be attributed", amount)
		}
		seen[amount] = true
		if amount <= 589000 || amount > 589000+transferSuffixMax {
			t.Fatalf("amount %d is outside the expected unique-suffix range", amount)
		}
	}
}

func TestSubscriptionPaymentGrantsAccessIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")
	if err := subscriptions.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	// Upgrading is buying the higher plan: paying a GROWTH invoice is what
	// grants GROWTH, not an separate admin action.
	invoice, err := subscriptions.IssueBankTransferInvoice(ctx, operatorID, "GROWTH")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	found, foundOperator, foundName, err := subscriptions.FindPayableByAmount(ctx, invoice.Amount)
	if err != nil || found != invoice.ID || foundOperator != operatorID {
		t.Fatalf("lookup by amount = %q/%q (%v)", found, foundOperator, err)
	}
	// The name comes back with the lookup so a confirmation can say whose
	// payment it was — read here rather than afterwards, because once the
	// invoice is settled it is no longer pending and a second query finds
	// nothing.
	if foundName == "" {
		t.Fatal("nama travel tidak ikut terbaca; konfirmasi tidak dapat menyebut pemiliknya")
	}

	if err := subscriptions.MarkPaid(ctx, invoice.ID); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	access, err := subscriptions.GetAccess(ctx, operatorID)
	if err != nil || !access.Allowed || access.Status != "ACTIVE" || access.Plan != "GROWTH" {
		t.Fatalf("access after payment = %+v (%v)", access, err)
	}
	var operatorPlan string
	if err := pool.QueryRow(ctx, `SELECT plan::text FROM operators WHERE id = $1`, operatorID).Scan(&operatorPlan); err != nil || operatorPlan != "GROWTH" {
		t.Fatalf("operator plan = %q (%v), want GROWTH", operatorPlan, err)
	}

	// Delivering the same payment twice must not buy a second period.
	first := access.AccessUntil
	if err := subscriptions.MarkPaid(ctx, invoice.ID); err != nil {
		t.Fatalf("replay: %v", err)
	}
	replayed, _ := subscriptions.GetAccess(ctx, operatorID)
	if !replayed.AccessUntil.Equal(first) {
		t.Fatalf("replayed payment extended access: %v -> %v", first, replayed.AccessUntil)
	}

	// A settled amount must no longer match an incoming mutation.
	if _, _, _, err := subscriptions.FindPayableByAmount(ctx, invoice.Amount); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("paid invoice still matched by amount: %v", err)
	}
}

func TestSubscriptionOverdueInvoiceReleasesItsAmountIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")

	invoice, err := subscriptions.IssueBankTransferInvoice(ctx, operatorID, "STARTER")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE subscription_invoices SET due_at = NOW() - INTERVAL '1 day' WHERE id = $1::uuid`, invoice.ID); err != nil {
		t.Fatalf("age invoice: %v", err)
	}
	if _, err := subscriptions.ExpireOverdueInvoices(ctx); err != nil {
		t.Fatalf("expire: %v", err)
	}
	// The amount no longer settles anything...
	if _, _, _, err := subscriptions.FindPayableByAmount(ctx, invoice.Amount); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expired invoice still payable: %v", err)
	}
	// ...but it is NOT free again today. Expiry releases the pending hold; the
	// per-day rule still applies, because a mutation arriving later today would
	// otherwise be ambiguous between the expired invoice and a new one.
	_, err = pool.Exec(ctx, `INSERT INTO subscription_invoices (operator_id,plan,channel,base_amount_idr,amount_idr,period_start,period_end,due_at)
		VALUES ($1,'STARTER','BANK_TRANSFER',589000,$2,NOW(),NOW()+INTERVAL '30 days',NOW()+INTERVAL '7 days')`, operatorID, invoice.Amount)
	if err == nil {
		t.Fatal("an expired amount was reissued on the same day")
	}
	// On another day it is available again.
	if _, err := pool.Exec(ctx, `INSERT INTO subscription_invoices (operator_id,plan,channel,base_amount_idr,amount_idr,period_start,period_end,due_at,created_at)
		VALUES ($1,'STARTER','BANK_TRANSFER',589000,$2,NOW(),NOW()+INTERVAL '30 days',NOW()+INTERVAL '7 days',NOW()-INTERVAL '3 days')`, operatorID, invoice.Amount); err != nil {
		t.Fatalf("amount was not released on a later day: %v", err)
	}
}

// An amount must not be reused within the same day even after its invoice is
// settled. A bank mutation carries a date, not a precise instant, so a code
// paid at 09:00 and reissued at 10:00 leaves that day's mutation ambiguous —
// it could belong to either invoice.
func TestSubscriptionTransferAmountIsUniquePerDayIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	first := newTestOperator(t, pool, "STARTER")
	second := newTestOperator(t, pool, "STARTER")

	invoice, err := subscriptions.IssueBankTransferInvoice(ctx, first, "STARTER")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	// Settle it: under the old rule this released the amount immediately.
	if err := subscriptions.MarkPaid(ctx, invoice.ID); err != nil {
		t.Fatalf("settle: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO subscription_invoices (operator_id, plan, channel, base_amount_idr, amount_idr, period_start, period_end, due_at)
		VALUES ($1, 'STARTER', 'BANK_TRANSFER', 589000, $2, NOW(), NOW() + INTERVAL '30 days', NOW() + INTERVAL '7 days')`,
		second, invoice.Amount)
	if err == nil {
		t.Fatal("a settled amount was reissued on the same day")
	}
	if !IsUniqueViolation(err, "subscription_invoices_transfer_daily_idx") {
		t.Fatalf("rejected for the wrong reason: %v", err)
	}

	// The next day it is free again — the constraint is per day, not forever.
	_, err = pool.Exec(ctx, `
		INSERT INTO subscription_invoices (operator_id, plan, channel, base_amount_idr, amount_idr, period_start, period_end, due_at, created_at)
		VALUES ($1, 'STARTER', 'BANK_TRANSFER', 589000, $2, NOW(), NOW() + INTERVAL '30 days', NOW() + INTERVAL '7 days', NOW() - INTERVAL '2 days')`,
		second, invoice.Amount)
	if err != nil {
		t.Fatalf("amount was not released on a different day: %v", err)
	}

	// And the issuer still finds a free suffix rather than failing. A third
	// operator is needed: `second` now holds a pending invoice, so issuing for
	// them correctly returns that one instead of minting a new amount.
	third := newTestOperator(t, pool, "STARTER")
	next, err := subscriptions.IssueBankTransferInvoice(ctx, third, "STARTER")
	if err != nil {
		t.Fatalf("issue after collision: %v", err)
	}
	if next.Amount == invoice.Amount {
		t.Fatalf("issued the same amount twice on one day: %d", next.Amount)
	}
}

// A double-clicked button or a retried request must not leave an operator
// holding two live unique amounts. Checking for an existing invoice before
// inserting cannot prevent this — both requests pass the check — so the
// database decides, and the loser is handed the invoice that won.
func TestSubscriptionConcurrentRequestsYieldOneInvoiceIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")

	const attempts = 12
	var wg sync.WaitGroup
	results := make([]Invoice, attempts)
	failures := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], failures[index] = subscriptions.IssueBankTransferInvoice(ctx, operatorID, "STARTER")
		}(i)
	}
	wg.Wait()

	ids := map[string]bool{}
	for i, invoice := range results {
		if failures[i] != nil {
			t.Fatalf("attempt %d failed instead of returning the existing invoice: %v", i, failures[i])
		}
		ids[invoice.ID] = true
	}
	if len(ids) != 1 {
		t.Fatalf("%d distinct invoices issued for one operator, want 1", len(ids))
	}

	var pending int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM subscription_invoices WHERE operator_id = $1 AND status = 'PENDING'`, operatorID).Scan(&pending); err != nil {
		t.Fatalf("count: %v", err)
	}
	if pending != 1 {
		t.Fatalf("%d pending invoices in the database, want 1", pending)
	}
}

// A bank feed redelivers, and a scraper re-reads the same page. Neither may
// settle an invoice twice, and neither may lose a credit that matched nothing —
// unaccounted money is the row that matters most here.
func TestBankMutationsSettleOnceAndKeepWhatDidNotMatchIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")
	if err := subscriptions.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	invoice, err := subscriptions.IssueBankTransferInvoice(ctx, operatorID, "GROWTH")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	external := "MUT-" + uuid.NewString()
	record := func(amount int64) (*BankMutation, bool) {
		m, fresh, err := subscriptions.RecordMutation(ctx, BankMutation{
			ExternalID: external, Source: "SCRAPER", AmountIDR: amount,
			Description: "TRANSFER MASUK", OccurredAt: time.Now(),
		})
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return m, fresh
	}

	mutation, fresh := record(invoice.Amount)
	if !fresh {
		t.Fatal("mutasi pertama dilaporkan bukan baru")
	}

	// Redelivery: the same entry, stored once, and reported as already known.
	if _, again := record(invoice.Amount); again {
		t.Fatal("pengiriman ulang tercatat sebagai mutasi baru")
	}

	settled, err := subscriptions.SettleInvoiceWithMutation(ctx, mutation.ID, invoice.ID, "uji", "cocok otomatis")
	if err != nil || !settled {
		t.Fatalf("settle = %v (%v)", settled, err)
	}

	// The second attempt changes nothing and says so, which is what makes an
	// automatic matcher and a person clicking safe to run at the same time.
	if again, err := subscriptions.SettleInvoiceWithMutation(ctx, mutation.ID, invoice.ID, "uji", "klik kedua"); err != nil || again {
		t.Fatalf("penyelesaian kedua = %v (%v), mau false", again, err)
	}

	var status, matched string
	if err := pool.QueryRow(ctx,
		`SELECT status, COALESCE(matched_invoice_id::text,'') FROM bank_mutations WHERE id = $1`,
		mutation.ID).Scan(&status, &matched); err != nil {
		t.Fatalf("read mutation: %v", err)
	}
	if status != "MATCHED" || matched != invoice.ID {
		t.Fatalf("mutasi = %s/%s", status, matched)
	}

	// Access was granted, not merely recorded as paid.
	var plan string
	if err := pool.QueryRow(ctx, `SELECT plan::text FROM operators WHERE id = $1`, operatorID).Scan(&plan); err != nil {
		t.Fatalf("read plan: %v", err)
	}
	if plan != "GROWTH" {
		t.Fatalf("paket = %s, mau GROWTH — pembayaran dicatat tanpa akses yang dibelinya", plan)
	}

	// A credit matching nothing is kept, not dropped. It is money that arrived
	// and has not been accounted for.
	orphan, fresh, err := subscriptions.RecordMutation(ctx, BankMutation{
		ExternalID: "MUT-" + uuid.NewString(), Source: "SCRAPER",
		AmountIDR: invoice.Amount + 7, Description: "TIDAK DIKENAL", OccurredAt: time.Now(),
	})
	if err != nil || !fresh {
		t.Fatalf("record orphan: %v", err)
	}
	if orphan.Status != "UNMATCHED" {
		t.Fatalf("mutasi tak cocok = %s", orphan.Status)
	}

	unmatched, err := subscriptions.ListMutations(ctx, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var seen bool
	for _, m := range unmatched {
		if m.ID == orphan.ID {
			seen = true
		}
		if m.ID == mutation.ID {
			t.Fatal("mutasi yang sudah cocok masih di antrean kerja")
		}
	}
	if !seen {
		t.Fatal("mutasi tak cocok hilang dari antrean")
	}
}

// Renewal has to be issued before access runs out, and exactly once. Two
// outstanding invoices would put two unique amounts in play for one operator,
// and a transfer against the older one would arrive looking unmatched.
func TestRenewalIsIssuedOnceAndNotForTheCancelledIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)

	operatorID := newTestOperator(t, pool, "STARTER")
	if err := subscriptions.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	// Access running out in three days: inside the billing lead time.
	if _, err := pool.Exec(ctx,
		`UPDATE subscriptions SET status = 'ACTIVE', access_until = NOW() + INTERVAL '3 days'
		 WHERE operator_id = $1`, operatorID); err != nil {
		t.Fatalf("set access: %v", err)
	}

	contains := func(due []RenewalDue, id string) bool {
		for _, item := range due {
			if item.OperatorID == id {
				return true
			}
		}
		return false
	}

	due, err := subscriptions.ListDueForRenewal(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	if !contains(due, operatorID) {
		t.Fatal("langganan yang hampir habis tidak masuk daftar tagih")
	}

	if _, err := subscriptions.IssueBankTransferInvoice(ctx, operatorID, "STARTER"); err != nil {
		t.Fatalf("issue: %v", err)
	}

	// Now billed, so the next sweep must leave it alone.
	due, err = subscriptions.ListDueForRenewal(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	if contains(due, operatorID) {
		t.Fatal("langganan yang sudah punya tagihan tertagih lagi")
	}

	// Somebody who cancelled should not receive a bill.
	cancelled := newTestOperator(t, pool, "STARTER")
	if err := subscriptions.EnsureForOperator(ctx, cancelled); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE subscriptions SET status = 'ACTIVE', access_until = NOW() + INTERVAL '1 day',
		 cancelled_at = NOW() WHERE operator_id = $1`, cancelled); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	due, err = subscriptions.ListDueForRenewal(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	if contains(due, cancelled) {
		t.Fatal("langganan yang sudah dibatalkan tetap ditagih")
	}
}

func TestMassBillingPreviewAndDatabaseIdempotencyIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")
	if err := subscriptions.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	periodStart := time.Now().UTC().Truncate(time.Microsecond).Add(72 * time.Hour)
	if _, err := pool.Exec(ctx, `UPDATE subscriptions SET status='ACTIVE', access_until=$2 WHERE operator_id=$1`, operatorID, periodStart); err != nil {
		t.Fatalf("prepare period: %v", err)
	}

	candidates, err := subscriptions.ListBillingCandidates(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	var candidate *BillingCandidate
	for i := range candidates {
		if candidates[i].OperatorID == operatorID {
			candidate = &candidates[i]
			break
		}
	}
	if candidate == nil {
		t.Fatal("periode yang jatuh tempo tidak muncul di pratinjau")
	}
	if candidate.BaseAmount != 589000 || !candidate.PeriodStart.Equal(periodStart) {
		t.Fatalf("candidate = %+v, want STARTER price and stable period", *candidate)
	}

	first, _, created, err := subscriptions.IssueBillingPeriod(ctx, operatorID, "STARTER", periodStart, 589000, "billing-test")
	if err != nil || !created {
		t.Fatalf("first issue: created=%v err=%v", created, err)
	}
	second, _, createdAgain, err := subscriptions.IssueBillingPeriod(ctx, operatorID, "STARTER", periodStart, 589000, "billing-test")
	if err != nil || createdAgain {
		t.Fatalf("replay: created=%v err=%v", createdAgain, err)
	}
	if second.ID != first.ID {
		t.Fatalf("replay returned invoice %s, want original %s", second.ID, first.ID)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM subscription_invoices WHERE operator_id=$1 AND period_start=$2`, operatorID, periodStart).Scan(&count); err != nil {
		t.Fatalf("count invoices: %v", err)
	}
	if count != 1 {
		t.Fatalf("database contains %d invoices for one commercial period", count)
	}

	after, err := subscriptions.ListBillingCandidates(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("preview after issue: %v", err)
	}
	for _, item := range after {
		if item.OperatorID == operatorID {
			t.Fatal("periode yang sudah ditagih masih muncul di pratinjau")
		}
	}
}

// The travel has to be told, and by the same signal whichever route settled
// their payment — automatic matching, an attached credit, or an amount typed
// off the statement. Learning whether you were notified based on which path ran
// is not a design, it is an accident.
//
// The subject is read before settling, because afterwards the invoice is no
// longer pending and the lookup finds nothing.
func TestInvoiceSubjectIsReadableOnlyWhilePendingIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")
	if err := subscriptions.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	invoice, err := subscriptions.IssueBankTransferInvoice(ctx, operatorID, "GROWTH")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	subject, amount, err := subscriptions.PendingInvoiceSubject(ctx, invoice.ID)
	if err != nil {
		t.Fatalf("subject: %v", err)
	}
	if subject != operatorID || amount != invoice.Amount {
		t.Fatalf("subjek = %s/%d, mau %s/%d", subject, amount, operatorID, invoice.Amount)
	}

	if err := subscriptions.MarkPaid(ctx, invoice.ID); err != nil {
		t.Fatalf("mark paid: %v", err)
	}

	// This is the trap the ordering exists to avoid: after settlement there is
	// nothing left to read, so a caller that looked it up afterwards would
	// silently notify nobody.
	if _, _, err := subscriptions.PendingInvoiceSubject(ctx, invoice.ID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("tagihan yang sudah lunas masih terbaca sebagai pending: %v", err)
	}
}
