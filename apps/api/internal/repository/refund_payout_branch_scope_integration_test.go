package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRefundPayoutRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	bandungRequest, medanRequest := uuid.NewString(), uuid.NewString()
	head := "payout-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Payout scope','ID',$3,$4,'GROWTH')`, op, "payout-scope-"+uuid.NewString(), op[:8]+"@example.test", "payout-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,'Jamaah Bandung','PAY-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$6,'Jamaah Medan','PAY-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, medan)
	exec(`INSERT INTO pilgrim_balance_entries (operator_id,pilgrim_id,amount_idr,kind,idempotency_key) VALUES ($1,$2,200000,'REFUND',$4),($1,$3,200000,'REFUND',$5)`, op, bandungPilgrim, medanPilgrim, "balance-bdg-"+uuid.NewString(), "balance-mdn-"+uuid.NewString())
	exec(`INSERT INTO pilgrim_refund_payout_requests (id,operator_id,pilgrim_id,beneficiary_kind,amount_idr,method,idempotency_key,requested_by_user_id) VALUES ($1,$3,$4,'PILGRIM',100000,'CASH',$6,'fixture'),($2,$3,$5,'PILGRIM',100000,'CASH',$7,'fixture')`, bandungRequest, medanRequest, op, bandungPilgrim, medanPilgrim, "request-bdg-"+uuid.NewString(), "request-mdn-"+uuid.NewString())

	repo := NewRefundPayoutRepository(pool)
	bandungCtx := ContextWithStaffActor(ctx, head)
	rows, err := repo.ListByOperator(bandungCtx, op, "")
	if err != nil || len(rows) != 1 || rows[0].ID != bandungRequest {
		t.Fatalf("daftar pencairan Bandung bocor: %#v (%v)", rows, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin Bandung: %v", err)
	}
	if _, err := repo.LockByIDTx(bandungCtx, tx, op, bandungRequest); err != nil {
		t.Fatalf("kepala Bandung tidak dapat memproses pencairan sendiri: %v", err)
	}
	_ = tx.Rollback(ctx)

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin Medan: %v", err)
	}
	if _, err := repo.LockByIDTx(bandungCtx, tx, op, medanRequest); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengunci pencairan Medan: %v", err)
	}
	_ = tx.Rollback(ctx)

	rows, err = repo.ListByOperator(ctx, op, "")
	if err != nil || len(rows) != 2 {
		t.Fatalf("kantor pusat kehilangan pencairan: %#v (%v)", rows, err)
	}
}
