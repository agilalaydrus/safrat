package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOrderRepositoryEnforcesBranchScopeIntegration(t *testing.T) {
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

	operatorID, seasonID, productID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	bandungID, medanID := uuid.NewString(), uuid.NewString()
	bandungPilgrimID, medanPilgrimID := uuid.NewString(), uuid.NewString()
	bandungHead := "order-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan)
	      VALUES ($1,$2,'Order Scope Test','ID',$3,$4,'GROWTH')`, operatorID,
		"order-scope-"+uuid.NewString(), operatorID[:8]+"@example.test", "order-"+operatorID[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, operatorID) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1,$2,'Musim Order','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',50)`, seasonID, operatorID)
	exec(`INSERT INTO products (id, operator_id, season_id, name, type, price_idr)
	      VALUES ($1,$2,$3,'Produk Uji','UMRAH',100000)`, productID, operatorID, seasonID)
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandungID, medanID, operatorID)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, bandungHead, bandungID, operatorID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, branch_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1,$3,$4,$5,'Jamaah Bandung','ORDER-BDG','ID','1990-01-01','MALE'),
	             ($2,$3,$4,$6,'Jamaah Medan','ORDER-MDN','ID','1991-01-01','FEMALE')`,
		bandungPilgrimID, medanPilgrimID, seasonID, operatorID, bandungID, medanID)

	repo := NewOrderRepository(db.New(pool), pool)
	bandungCtx := ContextWithStaffActor(ctx, bandungHead)
	create := func(pilgrimID, key string, requestCtx context.Context) (*domain.Order, bool, error) {
		return repo.Create(requestCtx, CreateOrderParams{
			OperatorID: operatorID, SeasonID: seasonID, PilgrimID: pilgrimID,
			BuyerKind: "PILGRIM", ProductID: productID, Quantity: 1,
			UnitPriceIDR: 100_000, TotalPriceIDR: 100_000, OperatorAmountIDR: 100_000,
			IdempotencyKey: key, CheckoutChannel: "MANUAL",
		})
	}
	bandungOrder, created, err := create(bandungPilgrimID, "bandung-"+uuid.NewString(), bandungCtx)
	if err != nil || !created {
		t.Fatalf("create Bandung: order=%#v created=%v err=%v", bandungOrder, created, err)
	}
	medanOrder, created, err := create(medanPilgrimID, "medan-"+uuid.NewString(), ctx)
	if err != nil || !created {
		t.Fatalf("create Medan system path: order=%#v created=%v err=%v", medanOrder, created, err)
	}
	var storedBranch string
	if err := pool.QueryRow(ctx, `SELECT branch_id::text FROM orders WHERE id=$1`, bandungOrder.ID).Scan(&storedBranch); err != nil || storedBranch != bandungID {
		t.Fatalf("order Bandung tidak mewarisi branch: %s (%v)", storedBranch, err)
	}
	if _, _, err := create(medanPilgrimID, "blocked-"+uuid.NewString(), bandungCtx); err == nil {
		t.Fatal("kepala Bandung berhasil membuat order untuk jamaah Medan")
	}

	orders, err := repo.ListBySeason(bandungCtx, operatorID, seasonID, 50, 0)
	if err != nil || len(orders) != 1 || orders[0].ID != bandungOrder.ID {
		t.Fatalf("list order Bandung bocor: %#v (%v)", orders, err)
	}
	count, err := repo.CountBySeason(bandungCtx, operatorID, seasonID)
	if err != nil || count != 1 {
		t.Fatalf("count order Bandung=%d (%v), mau 1", count, err)
	}
	if _, err := repo.Get(bandungCtx, operatorID, medanOrder.ID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca order Medan: %v", err)
	}
	if _, err := repo.MarkPaidManually(bandungCtx, operatorID, medanOrder.ID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menandai lunas order Medan: %v", err)
	}
	exec(`UPDATE orders SET status='HELD' WHERE id=$1`, medanOrder.ID)
	if _, err := repo.ResolveHeld(bandungCtx, operatorID, medanOrder.ID, true); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menyelesaikan held order Medan: %v", err)
	}
}
