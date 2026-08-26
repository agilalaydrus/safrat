package repository

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ledgerTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Ledger rows refuse deletion, so tearing a fixture down needs the explicit
// purge flag — the same escape hatch a real tenant teardown would use.
func purgeOperator(t *testing.T, pool *pgxpool.Pool, operatorID string) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Logf("cleanup: %v", err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.allow_ledger_purge', 'on', true)`); err != nil {
		t.Logf("cleanup: %v", err)
		return
	}
	_, _ = tx.Exec(ctx, `DELETE FROM agent_commission_entries WHERE operator_id = $1`, operatorID)
	_, _ = tx.Exec(ctx, `DELETE FROM pilgrim_balance_entries WHERE operator_id = $1`, operatorID)
	_, _ = tx.Exec(ctx, `DELETE FROM operators WHERE id = $1`, operatorID)
	_ = tx.Commit(ctx)
}

func TestLedgerCommissionIsAuditableAndIdempotentIntegration(t *testing.T) {
	pool := ledgerTestPool(t)
	ctx := context.Background()
	ledger := NewLedgerRepository(pool)

	operatorID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Ledger Test','ID',$3,$4)`,
		operatorID, "ledger-"+uuid.NewString(), operatorID[:8]+"@example.com", "ledger-"+operatorID[:8]); err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { purgeOperator(t, pool, operatorID) })

	agentID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, operator_id, name) VALUES ($1,$2,'Agen Ledger')`, agentID, operatorID); err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	const commission = int64(750_000)
	key := "order-earned-" + uuid.NewString()
	entry := CommissionEntry{OperatorID: operatorID, AgentID: agentID, AmountIDR: commission, Kind: "EARNED", IdempotencyKey: key}

	// A redelivered webhook must not pay commission twice.
	const attempts = 6
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ledger.AppendCommission(ctx, entry); err != nil {
				t.Errorf("append: %v", err)
			}
		}()
	}
	wg.Wait()

	balance, err := ledger.CommissionBalance(ctx, agentID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if balance != commission {
		t.Fatalf("balance = %d after %d replays, want %d", balance, attempts, commission)
	}

	// A refund reverses it, and the reversal is a new row — the earning stays
	// visible, which is the whole point of an auditable trail.
	if err := ledger.AppendCommission(ctx, CommissionEntry{
		OperatorID: operatorID, AgentID: agentID, AmountIDR: -commission, Kind: "REVERSED",
		Note: "refund", IdempotencyKey: "order-reversed-" + key,
	}); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if balance, err = ledger.CommissionBalance(ctx, agentID); err != nil || balance != 0 {
		t.Fatalf("balance after reversal = %d (%v), want 0", balance, err)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_commission_entries WHERE agent_id = $1`, agentID).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 2 {
		t.Fatalf("%d ledger rows, want 2 — the reversal must not erase the earning", rows)
	}

	// An agent carrying commission history cannot be deleted; deactivating is
	// the supported path.
	if _, err := pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, agentID); err == nil {
		t.Fatal("an agent with commission history was deleted")
	}
}

func TestLedgerPilgrimBalanceIntegration(t *testing.T) {
	pool := ledgerTestPool(t)
	ctx := context.Background()
	ledger := NewLedgerRepository(pool)

	operatorID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Balance Test','ID',$3,$4)`,
		operatorID, "balance-"+uuid.NewString(), operatorID[:8]+"@example.com", "balance-"+operatorID[:8]); err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { purgeOperator(t, pool, operatorID) })

	seasonID, pilgrimID := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID); err != nil {
		t.Fatalf("insert season: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender) VALUES ($1,$2,$3,'Jamaah','P1','ID','1990-01-01'::timestamptz,'MALE')`, pilgrimID, seasonID, operatorID); err != nil {
		t.Fatalf("insert pilgrim: %v", err)
	}

	if balance, err := ledger.PilgrimBalance(ctx, pilgrimID); err != nil || balance != 0 {
		t.Fatalf("opening balance = %d (%v), want 0", balance, err)
	}
	// A refund is what puts money into a pilgrim's balance.
	if err := ledger.AppendBalance(ctx, BalanceEntry{
		OperatorID: operatorID, PilgrimID: pilgrimID, AmountIDR: 2_000_000, Kind: "REFUND",
		Note: "Pembatalan paket", IdempotencyKey: "refund-" + uuid.NewString(),
	}); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if balance, err := ledger.PilgrimBalance(ctx, pilgrimID); err != nil || balance != 2_000_000 {
		t.Fatalf("balance after refund = %d (%v)", balance, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE pilgrim_balance_entries SET amount_idr = 9 WHERE pilgrim_id = $1`, pilgrimID); err == nil {
		t.Fatal("a balance entry was edited")
	}
}
