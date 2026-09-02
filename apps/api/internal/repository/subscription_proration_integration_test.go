package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
)

func TestPlanUpgradeWaitsForPaymentAndIsIdempotentIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	repo := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")
	if err := repo.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	accessUntil := time.Now().UTC().Truncate(time.Microsecond).Add(15 * 24 * time.Hour)
	if _, err := pool.Exec(ctx, `UPDATE subscriptions SET status='ACTIVE', access_until=$2 WHERE operator_id=$1`, operatorID, accessUntil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	preview, err := repo.PreviewPlanChange(ctx, operatorID, "GROWTH")
	if err != nil || preview.AdjustmentIDR <= 0 {
		t.Fatalf("preview upgrade = %+v err=%v", preview, err)
	}
	change := PlanChange{
		OperatorID: operatorID, NewPlan: "GROWTH", ExpectedAdjustment: preview.AdjustmentIDR,
		Reason: "naik paket di tengah periode", Confirmation: "Sub Test",
		ActorUserID: "proration-test", IdempotencyKey: "upgrade-" + operatorID,
	}
	result, err := repo.ApplyPlanChange(ctx, change)
	if err != nil || result.Status != "PENDING_PAYMENT" || result.InvoiceID == "" || result.InvoiceAmountIDR <= result.AdjustmentIDR {
		t.Fatalf("apply upgrade = %+v err=%v", result, err)
	}
	var plan string
	if err := pool.QueryRow(ctx, `SELECT plan::text FROM subscriptions WHERE operator_id=$1`, operatorID).Scan(&plan); err != nil || plan != "STARTER" {
		t.Fatalf("unpaid upgrade activated plan %q err=%v", plan, err)
	}
	if _, err := repo.PreviewPlanChange(ctx, operatorID, "PRO"); !errors.Is(err, ErrNoProration) {
		t.Fatalf("second plan change while invoice pending = %v, want refused", err)
	}
	replayed, err := repo.ApplyPlanChange(ctx, change)
	if err != nil || replayed.AdjustmentID != result.AdjustmentID || replayed.InvoiceID != result.InvoiceID {
		t.Fatalf("replay = %+v err=%v, want original", replayed, err)
	}
	change.ExpectedAdjustment++
	if _, err := repo.ApplyPlanChange(ctx, change); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("same key different amount = %v, want conflict", err)
	}

	if err := repo.MarkPaid(ctx, result.InvoiceID); err != nil {
		t.Fatalf("pay proration: %v", err)
	}
	var paidAccess time.Time
	if err := pool.QueryRow(ctx, `SELECT plan::text,access_until FROM subscriptions WHERE operator_id=$1`, operatorID).Scan(&plan, &paidAccess); err != nil {
		t.Fatalf("read paid upgrade: %v", err)
	}
	if plan != "GROWTH" {
		t.Fatalf("paid upgrade left plan %q", plan)
	}
	if !paidAccess.Equal(accessUntil) {
		t.Fatalf("proration payment extended access %v -> %v", accessUntil, paidAccess)
	}
}

func TestPlanDowngradeAppliesCreditWithoutRewritingInvoicesIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	repo := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "GROWTH")
	if err := repo.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	accessUntil := time.Now().UTC().Truncate(time.Microsecond).Add(15 * 24 * time.Hour)
	if _, err := pool.Exec(ctx, `UPDATE subscriptions SET status='ACTIVE', access_until=$2 WHERE operator_id=$1`, operatorID, accessUntil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	preview, err := repo.PreviewPlanChange(ctx, operatorID, "STARTER")
	if err != nil || preview.AdjustmentIDR >= 0 {
		t.Fatalf("preview downgrade = %+v err=%v", preview, err)
	}
	result, err := repo.ApplyPlanChange(ctx, PlanChange{
		OperatorID: operatorID, NewPlan: "STARTER", ExpectedAdjustment: preview.AdjustmentIDR,
		Reason: "turun paket di tengah periode", Confirmation: "Sub Test",
		ActorUserID: "proration-test", IdempotencyKey: "downgrade-" + operatorID,
	})
	if err != nil || result.Status != "APPLIED" || result.InvoiceID != "" || result.CreditBalanceIDR != -preview.AdjustmentIDR {
		t.Fatalf("apply downgrade = %+v err=%v", result, err)
	}
	var plan string
	var credit int64
	var actualAccess time.Time
	if err := pool.QueryRow(ctx, `SELECT plan::text,credit_balance_idr,access_until FROM subscriptions WHERE operator_id=$1`, operatorID).Scan(&plan, &credit, &actualAccess); err != nil {
		t.Fatalf("read downgrade: %v", err)
	}
	if plan != "STARTER" || credit != -preview.AdjustmentIDR || !actualAccess.Equal(accessUntil) {
		t.Fatalf("downgrade state plan=%s credit=%d access=%v", plan, credit, actualAccess)
	}
	var invoiceCount, adjustmentCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM subscription_invoices WHERE operator_id=$1`, operatorID).Scan(&invoiceCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM subscription_adjustments WHERE operator_id=$1`, operatorID).Scan(&adjustmentCount); err != nil {
		t.Fatal(err)
	}
	if invoiceCount != 0 || adjustmentCount != 1 {
		t.Fatalf("downgrade wrote invoices=%d adjustments=%d, want 0/1", invoiceCount, adjustmentCount)
	}

	// The credit is reserved in the next renewal's payable amount, but remains
	// on the balance until money actually settles. An abandoned invoice must
	// not consume value the tenant owns.
	renewal, _, created, err := repo.IssueBillingPeriod(ctx, operatorID, "STARTER", accessUntil, 589000, "proration-test")
	if err != nil || !created {
		t.Fatalf("issue credited renewal: created=%v err=%v", created, err)
	}
	if renewal.Amount >= 589000 || renewal.Amount <= 589000-credit {
		t.Fatalf("credited invoice amount=%d, gross=589000 credit=%d", renewal.Amount, credit)
	}
	var beforePayment int64
	if err := pool.QueryRow(ctx, `SELECT credit_balance_idr FROM subscriptions WHERE operator_id=$1`, operatorID).Scan(&beforePayment); err != nil || beforePayment != credit {
		t.Fatalf("credit consumed before payment: balance=%d err=%v", beforePayment, err)
	}
	if err := repo.MarkPaid(ctx, renewal.ID); err != nil {
		t.Fatalf("pay credited renewal: %v", err)
	}
	var afterPayment, applications int64
	if err := pool.QueryRow(ctx, `SELECT credit_balance_idr FROM subscriptions WHERE operator_id=$1`, operatorID).Scan(&afterPayment); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM subscription_credit_applications WHERE invoice_id=$1`, renewal.ID).Scan(&applications); err != nil {
		t.Fatal(err)
	}
	if afterPayment != 0 || applications != 1 {
		t.Fatalf("settled credit balance=%d applications=%d, want 0/1", afterPayment, applications)
	}
}
