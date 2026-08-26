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

	found, foundOperator, err := subscriptions.FindPayableByAmount(ctx, invoice.Amount)
	if err != nil || found != invoice.ID || foundOperator != operatorID {
		t.Fatalf("lookup by amount = %q/%q (%v)", found, foundOperator, err)
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
	if _, _, err := subscriptions.FindPayableByAmount(ctx, invoice.Amount); !errors.Is(err, apperror.ErrNotFound) {
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
	// The amount is released for reuse, and no longer settles anything.
	if _, _, err := subscriptions.FindPayableByAmount(ctx, invoice.Amount); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expired invoice still payable: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO subscription_invoices (operator_id,plan,channel,base_amount_idr,amount_idr,period_start,period_end,due_at)
		VALUES ($1,'STARTER','BANK_TRANSFER',589000,$2,NOW(),NOW()+INTERVAL '30 days',NOW()+INTERVAL '7 days')`, operatorID, invoice.Amount); err != nil {
		t.Fatalf("amount was not released for reuse: %v", err)
	}
}
