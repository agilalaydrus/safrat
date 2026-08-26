package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
)

// A gateway payment must unlock the dashboard, and redelivering it must not buy
// a second period — Xendit retries, and a retry is not a new payment.
func TestSubscriptionGatewaySettlementIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "STARTER")
	if err := subscriptions.EnsureForOperator(ctx, operatorID); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	// Start from a lapsed subscription: this is the state a payment must fix.
	if _, err := pool.Exec(ctx, `UPDATE subscriptions SET access_until = NOW() - INTERVAL '1 day', status = 'PAST_DUE' WHERE operator_id = $1`, operatorID); err != nil {
		t.Fatalf("lapse: %v", err)
	}

	externalID := "sub-" + uuid.NewString()
	if _, err := subscriptions.IssueGatewayInvoice(ctx, operatorID, "GROWTH", externalID, "https://checkout.example/x"); err != nil {
		t.Fatalf("issue: %v", err)
	}
	if locked, _ := subscriptions.GetAccess(ctx, operatorID); locked.Allowed {
		t.Fatal("issuing an invoice granted access before payment")
	}

	if err := subscriptions.MarkPaidByExternalID(ctx, externalID); err != nil {
		t.Fatalf("settle: %v", err)
	}
	access, err := subscriptions.GetAccess(ctx, operatorID)
	if err != nil || !access.Allowed || access.Plan != "GROWTH" {
		t.Fatalf("access after gateway payment = %+v (%v)", access, err)
	}

	first := access.AccessUntil
	if err := subscriptions.MarkPaidByExternalID(ctx, externalID); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	replayed, _ := subscriptions.GetAccess(ctx, operatorID)
	if !replayed.AccessUntil.Equal(first) {
		t.Fatalf("redelivered webhook extended access: %v -> %v", first, replayed.AccessUntil)
	}

	// An id we never issued belongs to something else — the handler relies on
	// this to fall through to the order path.
	if err := subscriptions.MarkPaidByExternalID(ctx, "sub-"+uuid.NewString()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("unknown external id = %v, want not found", err)
	}

	// A late "expired" delivery must never undo a payment.
	if err := subscriptions.CloseByExternalID(ctx, externalID, "EXPIRED"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expiring a paid invoice = %v, want no-op", err)
	}
	stillPaid, _ := subscriptions.GetAccess(ctx, operatorID)
	if !stillPaid.Allowed {
		t.Fatal("a late expiry revoked a paid subscription")
	}
}
