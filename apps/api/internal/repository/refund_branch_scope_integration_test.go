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

func TestRefundRepositoryEnforcesBranchScopeBothWaysIntegration(t *testing.T) {
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

	op, season, product := uuid.NewString(), uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString()
	bandungOrder, medanOrder := uuid.NewString(), uuid.NewString()
	head := "refund-branch-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Refund scope','ID',$3,$4,'GROWTH')`, op, "refund-scope-"+uuid.NewString(), op[:8]+"@example.test", "refund-"+op[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO products (id,operator_id,season_id,name,type,price_idr) VALUES ($1,$2,$3,'Produk','UMRAH',100000)`, product, op, season)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,'Jamaah Bandung','REF-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$6,'Jamaah Medan','REF-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, medan)
	exec(`INSERT INTO orders (id,operator_id,season_id,pilgrim_id,product_id,branch_id,quantity,unit_price_idr,total_price_idr,status,idempotency_key) VALUES ($1,$3,$4,$5,$7,$8,1,100000,100000,'PAID',$9),($2,$3,$4,$6,$7,$10,1,100000,100000,'PAID',$11)`, bandungOrder, medanOrder, op, season, bandungPilgrim, medanPilgrim, product, bandung, "refund-bdg-"+uuid.NewString(), medan, "refund-mdn-"+uuid.NewString())
	exec(`INSERT INTO order_refunds (operator_id,order_id,amount_idr,reason,idempotency_key) VALUES ($1,$2,100000,'Bandung',$4),($1,$3,100000,'Medan',$5)`, op, bandungOrder, medanOrder, "history-bdg-"+uuid.NewString(), "history-mdn-"+uuid.NewString())

	repo := NewRefundRepository(pool)
	bandungCtx := ContextWithStaffActor(ctx, head)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin Bandung: %v", err)
	}
	if _, err := repo.LockOrderForRefund(bandungCtx, tx, op, bandungOrder); err != nil {
		t.Fatalf("kepala Bandung tidak dapat mengunci order sendiri: %v", err)
	}
	_ = tx.Rollback(ctx)

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin Medan: %v", err)
	}
	if _, err := repo.LockOrderForRefund(bandungCtx, tx, op, medanOrder); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengunci order Medan untuk refund: %v", err)
	}
	_ = tx.Rollback(ctx)

	rows, err := repo.ListByOrder(bandungCtx, op, bandungOrder)
	if err != nil || len(rows) != 1 {
		t.Fatalf("riwayat refund Bandung tidak terlihat: %#v (%v)", rows, err)
	}
	rows, err = repo.ListByOrder(bandungCtx, op, medanOrder)
	if err != nil || len(rows) != 0 {
		t.Fatalf("riwayat refund Medan bocor ke Bandung: %#v (%v)", rows, err)
	}
	rows, err = repo.ListByOrder(ctx, op, medanOrder)
	if err != nil || len(rows) != 1 {
		t.Fatalf("kantor pusat kehilangan riwayat refund Medan: %#v (%v)", rows, err)
	}
}
