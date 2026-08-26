package service

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// An agent must never be able to request more than they are owed. Reading the
// balance and inserting the request are separate statements, so concurrent
// calls — a double-clicked button, or a retry — both saw the same available
// figure and both passed the check.
func TestRequestPayoutCannotExceedBalanceUnderConcurrencyIntegration(t *testing.T) {
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

	operatorID := uuid.NewString()
	orgID := "payout-race-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Payout Race','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.com", "payout-"+operatorID[:8]); err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	// agents.linked_user_id references Better Auth's own table.
	userID := "user-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, name, email, "emailVerified") VALUES ($1, 'Agen Uji', $2, true)`,
		userID, userID+"@example.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID) })

	agentID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, operator_id, name, linked_user_id) VALUES ($1,$2,'Agen Uji',$3)`,
		agentID, operatorID, userID); err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	// Commission is earned through PAID orders, so the balance has to come from
	// one rather than being written directly.
	const earned = int64(1_000_000)
	seasonID, pilgrimID, productID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Uji','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID); err != nil {
		t.Fatalf("insert season: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender) VALUES ($1,$2,$3,'Jamaah Uji','P-UJI','ID','1990-01-01'::timestamptz,'MALE')`, pilgrimID, seasonID, operatorID); err != nil {
		t.Fatalf("insert pilgrim: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO products (id, operator_id, season_id, name, price_idr) VALUES ($1,$2,$3,'Produk Uji',$4)`, productID, operatorID, seasonID, earned); err != nil {
		t.Fatalf("insert product: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, agent_id, unit_price_idr, total_price_idr, agent_commission_idr, status)
		VALUES ($1,$2,$3,$4,$5,$6,$6,$6,'PAID')`, operatorID, seasonID, pilgrimID, productID, agentID, earned); err != nil {
		t.Fatalf("insert paid order: %v", err)
	}

	queries := db.New(pool)
	service := NewAgentService(repository.NewOperatorRepository(queries), repository.NewAgentRepository(queries), repository.NewAuditRepository(queries), pool)

	const attempts = 8
	var wg sync.WaitGroup
	accepted := make([]bool, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.RequestPayout(ctx, orgID, userID, &hajjv1.RequestAgentPayoutRequest{AmountIdr: earned, Note: "uji"})
			accepted[index] = err == nil
		}(i)
	}
	wg.Wait()

	granted := 0
	for _, ok := range accepted {
		if ok {
			granted++
		}
	}
	var totalRequested int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_idr),0) FROM agent_payout_requests WHERE agent_id = $1 AND status = 'PENDING'`, agentID).Scan(&totalRequested); err != nil {
		t.Fatalf("sum requests: %v", err)
	}
	if totalRequested > earned {
		t.Fatalf("agent requested %d against a balance of %d (%d requests accepted)", totalRequested, earned, granted)
	}
	if granted != 1 {
		t.Fatalf("%d concurrent requests accepted, want exactly 1", granted)
	}
}
