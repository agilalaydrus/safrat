package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// F5/F6 (TUGAS-PANEL-SAAS.md): VoidInvoice writes an audit_logs entry for
// every void, and the same admin voiding a SECOND, different invoice must
// not be blocked by the first. It once was: the audit write used an empty
// idempotency_key, and audit_logs' uniqueness is (user_id, action,
// idempotency_key) with no invoice or operator in it at all — so the first
// invoice any admin ever voided permanently claimed that (user, action, "")
// triple, and voiding anything else afterward hit a unique violation.
func TestVoidInvoiceWritesAuditAndAllowsASecondDifferentInvoiceIntegration(t *testing.T) {
	pool := subscriptionTestPool(t)
	ctx := context.Background()
	subscriptions := NewSubscriptionRepository(pool)
	operatorID := newTestOperator(t, pool, "GROWTH")
	actorUserID := "void-test-admin-" + uuid.NewString()

	newPendingInvoice := func(amount int64) string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO subscription_invoices (operator_id,plan,channel,base_amount_idr,amount_idr,period_start,period_end,due_at)
			VALUES ($1,'GROWTH','BANK_TRANSFER',$2,$2,NOW(),NOW()+INTERVAL '30 days',NOW()+INTERVAL '7 days')
			RETURNING id::text`, operatorID, amount).Scan(&id); err != nil {
			t.Fatalf("fixture invoice: %v", err)
		}
		return id
	}

	// Only one PENDING invoice per operator at a time, so the second is
	// created only after the first is voided — same admin, two different
	// invoices, one after another, which is exactly the regression.
	first := newPendingInvoice(100037)
	if err := subscriptions.VoidInvoice(ctx, first, "alasan pembatalan pertama", actorUserID); err != nil {
		t.Fatalf("void pertama: %v", err)
	}
	second := newPendingInvoice(100091)
	if err := subscriptions.VoidInvoice(ctx, second, "alasan pembatalan kedua", actorUserID); err != nil {
		t.Fatalf("void kedua oleh admin yang sama: %v", err)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs
		WHERE user_id = $1 AND action = 'subscription_invoice_voided' AND entity_id IN ($2,$3)`,
		actorUserID, first, second).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 2 {
		t.Fatalf("%d jejak audit untuk dua pembatalan berbeda, mau 2", auditCount)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE user_id = $1`, actorUserID)
	})

	var firstStatus, secondStatus string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM subscription_invoices WHERE id = $1`, first).Scan(&firstStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status::text FROM subscription_invoices WHERE id = $1`, second).Scan(&secondStatus); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "CANCELLED" || secondStatus != "CANCELLED" {
		t.Fatalf("status setelah dibatalkan = %q, %q, mau CANCELLED keduanya", firstStatus, secondStatus)
	}

	// A second void of the SAME invoice is a no-op, not an error that would
	// suggest something needs retrying.
	if err := subscriptions.VoidInvoice(ctx, first, "coba lagi", actorUserID); err == nil {
		t.Fatal("membatalkan invoice yang sudah dibatalkan seharusnya menolak (failed_precondition), bukan berhasil diam-diam")
	}
}
