package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCashFlowRepositoryEnforcesBranchScopeIntegration(t *testing.T) {
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
	bandungID, medanID := uuid.NewString(), uuid.NewString()
	bandungHead := "cashflow-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan)
	      VALUES ($1,$2,'Cashflow Scope Test','ID',$3,$4,'GROWTH')`, operatorID,
		"cashflow-scope-"+uuid.NewString(), operatorID[:8]+"@example.test", "cashflow-"+operatorID[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, operatorID) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1,$2,'Musim Cashflow','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',50)`, seasonID, operatorID)
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandungID, medanID, operatorID)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, bandungHead, bandungID, operatorID)

	repo := NewCashFlowRepository(db.New(pool))
	bandungCtx := ContextWithStaffActor(ctx, bandungHead)
	dueDate := time.Now().AddDate(0, 0, 7)
	central, err := repo.CreatePayment(ctx, operatorID, seasonID, "Vendor Pusat", "HOTEL", "", 400_000, dueDate)
	if err != nil {
		t.Fatalf("create pusat: %v", err)
	}
	bandung, err := repo.CreatePayment(bandungCtx, operatorID, seasonID, "Vendor Bandung", "TRANSPORT", "", 250_000, dueDate)
	if err != nil {
		t.Fatalf("create Bandung: %v", err)
	}
	payments, err := repo.ListPayments(bandungCtx, operatorID, seasonID)
	if err != nil || len(payments) != 1 || payments[0].ID != bandung.ID {
		t.Fatalf("daftar cashflow Bandung bocor: %#v (%v)", payments, err)
	}
	if _, err := repo.UpdatePaymentStatus(bandungCtx, operatorID, central.ID, "PAID"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengubah tagihan pusat: %v", err)
	}
	if err := repo.DeletePayment(bandungCtx, operatorID, central.ID); err != nil {
		t.Fatalf("delete lintas cabang harus no-op aman: %v", err)
	}
	var centralExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vendor_payments WHERE id=$1)`, central.ID).Scan(&centralExists); err != nil || !centralExists {
		t.Fatalf("tagihan pusat terhapus: exists=%v err=%v", centralExists, err)
	}
	summary, err := repo.GetSummary(bandungCtx, operatorID, seasonID)
	if err != nil || summary.TotalCommittedIDR != 250_000 {
		t.Fatalf("ringkasan cashflow Bandung bocor: %#v (%v)", summary, err)
	}
	headOffice, err := repo.ListPayments(ContextWithStaffActor(ctx, "cashflow-hq-"+uuid.NewString()), operatorID, seasonID)
	if err != nil || len(headOffice) != 2 {
		t.Fatalf("kantor pusat harus melihat dua tagihan: %#v (%v)", headOffice, err)
	}
}
