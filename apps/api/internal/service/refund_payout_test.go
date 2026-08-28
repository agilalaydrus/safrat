package service

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type refundPayoutFixture struct {
	pool                         *pgxpool.Pool
	service                      *RefundPayoutService
	ledger                       *repository.LedgerRepository
	operatorID, orgID, pilgrimID string
	pilgrimCtx, ownerCtx         context.Context
}

func newRefundPayoutFixture(t *testing.T) *refundPayoutFixture {
	t.Helper()
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

	operatorID, orgID := uuid.NewString(), "refund-payout-"+uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Travel Refund','ID',$3,$4)`, operatorID, orgID, operatorID[:8]+"@example.com", "refund-payout-"+operatorID[:8]); err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	userID := "pilgrim-" + uuid.NewString()
	ownerID := "owner-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, name, email, "emailVerified", "twoFactorEnabled") VALUES ($1,'Jamaah Refund',$2,true,true),($3,'Owner Refund',$4,true,true)`, userID, userID+"@example.com", ownerID, ownerID+"@example.com"); err != nil {
		t.Fatalf("insert users: %v", err)
	}

	seasonID, pilgrimID, accessCode := uuid.NewString(), uuid.NewString(), "refund-"+uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Refund Payout','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID); err != nil {
		t.Fatalf("insert season: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender, phone, app_access_code, linked_user_id) VALUES ($1,$2,$3,'Jamaah Refund','P-PAYOUT','ID','1990-01-01'::timestamptz,'MALE','08123456789',$4,$5)`, pilgrimID, seasonID, operatorID, accessCode, userID); err != nil {
		t.Fatalf("insert pilgrim: %v", err)
	}

	t.Cleanup(func() {
		cleanup, err := pool.Begin(context.Background())
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Rollback(context.Background()) }()
		if _, err := cleanup.Exec(context.Background(), `SELECT set_config('app.allow_ledger_purge', 'on', true)`); err != nil {
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID); err != nil {
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM "user" WHERE id IN ($1,$2)`, userID, ownerID); err != nil {
			return
		}
		_ = cleanup.Commit(context.Background())
	})

	queries := db.New(pool)
	ledger := repository.NewLedgerRepository(pool)
	if err := ledger.AppendBalance(ctx, repository.BalanceEntry{
		OperatorID: operatorID, PilgrimID: pilgrimID, AmountIDR: 1_000_000,
		Kind: "REFUND", Note: "Refund untuk uji payout", IdempotencyKey: "refund-payout-fixture-" + pilgrimID,
	}); err != nil {
		t.Fatalf("credit balance: %v", err)
	}
	identity := repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries))
	payouts := repository.NewRefundPayoutRepository(pool)
	service := NewRefundPayoutService(
		repository.NewOperatorRepository(queries), identity, payouts,
		ledger, repository.NewAuditRepository(queries), pool,
	)
	return &refundPayoutFixture{
		pool: pool, service: service, ledger: ledger, operatorID: operatorID, orgID: orgID, pilgrimID: pilgrimID,
		pilgrimCtx: middleware.ContextWithIdentity(ctx, userID, ""),
		ownerCtx:   middleware.ContextWithStaffIdentity(ctx, ownerID, orgID, "owner"),
	}
}

func TestRefundPayoutLifecycleIsReservedIdempotentAndLedgerBackedIntegration(t *testing.T) {
	f := newRefundPayoutFixture(t)
	ctx := f.pilgrimCtx
	code := ""
	if err := f.pool.QueryRow(ctx, `SELECT app_access_code FROM pilgrims WHERE id = $1`, f.pilgrimID).Scan(&code); err != nil {
		t.Fatalf("read access code: %v", err)
	}
	key := uuid.NewString()
	created, err := f.service.RequestRefundPayout(ctx, &hajjv1.RequestRefundPayoutRequest{
		AppAccessCode: code, AmountIdr: 800_000,
		Method:         hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_BANK_TRANSFER,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("request payout: %v", err)
	}
	replayed, err := f.service.RequestRefundPayout(ctx, &hajjv1.RequestRefundPayoutRequest{
		AppAccessCode: code, AmountIdr: 1,
		Method:         hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_CASH,
		IdempotencyKey: key,
	})
	if err != nil || replayed.Id != created.Id || replayed.AmountIdr != 800_000 {
		t.Fatalf("replay = %+v, %v; want original %s", replayed, err, created.Id)
	}

	wallet, err := f.service.GetMyRefundWallet(ctx, &hajjv1.GetMyRefundWalletRequest{AppAccessCode: code})
	if err != nil {
		t.Fatalf("wallet after request: %v", err)
	}
	if wallet.BalanceIdr != 1_000_000 || wallet.ReservedIdr != 800_000 || wallet.AvailableIdr != 200_000 {
		t.Fatalf("wallet = balance %d reserved %d available %d", wallet.BalanceIdr, wallet.ReservedIdr, wallet.AvailableIdr)
	}
	processing, err := f.service.TransitionRefundPayout(f.ownerCtx, f.orgID, "owner", &hajjv1.TransitionRefundPayoutRequest{
		RequestId: created.Id, Action: hajjv1.RefundPayoutAction_REFUND_PAYOUT_ACTION_START_PROCESSING,
	})
	if err != nil || processing.Status != hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PROCESSING {
		t.Fatalf("start processing = %+v, %v", processing, err)
	}
	paid, err := f.service.TransitionRefundPayout(f.ownerCtx, f.orgID, "owner", &hajjv1.TransitionRefundPayoutRequest{
		RequestId: created.Id, Action: hajjv1.RefundPayoutAction_REFUND_PAYOUT_ACTION_MARK_PAID,
		PaymentReference: "TRX-REFUND-001", Note: "ditransfer",
	})
	if err != nil || paid.Status != hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PAID {
		t.Fatalf("mark paid = %+v, %v", paid, err)
	}
	if _, err := f.service.TransitionRefundPayout(f.ownerCtx, f.orgID, "owner", &hajjv1.TransitionRefundPayoutRequest{
		RequestId: created.Id, Action: hajjv1.RefundPayoutAction_REFUND_PAYOUT_ACTION_MARK_PAID,
		PaymentReference: "TRX-REFUND-001",
	}); err != nil {
		t.Fatalf("replay paid: %v", err)
	}
	balance, err := f.ledger.PilgrimBalance(ctx, f.pilgrimID)
	if err != nil || balance != 200_000 {
		t.Fatalf("balance after payout = %d, %v", balance, err)
	}
	var withdrawals int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM pilgrim_balance_entries WHERE pilgrim_id = $1 AND kind = 'WITHDRAWAL'`, f.pilgrimID).Scan(&withdrawals); err != nil || withdrawals != 1 {
		t.Fatalf("withdrawals = %d, %v; want 1", withdrawals, err)
	}
}

func TestConcurrentRefundPayoutRequestsCannotOverReserveIntegration(t *testing.T) {
	f := newRefundPayoutFixture(t)
	var code string
	if err := f.pool.QueryRow(f.pilgrimCtx, `SELECT app_access_code FROM pilgrims WHERE id = $1`, f.pilgrimID).Scan(&code); err != nil {
		t.Fatal(err)
	}
	const attempts = 6
	accepted := make([]bool, attempts)
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := f.service.RequestRefundPayout(f.pilgrimCtx, &hajjv1.RequestRefundPayoutRequest{
				AppAccessCode: code, AmountIdr: 700_000,
				Method:         hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_EWALLET,
				IdempotencyKey: uuid.NewString(),
			})
			accepted[index] = err == nil
		}(i)
	}
	wg.Wait()
	count := 0
	for _, ok := range accepted {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("%d concurrent requests accepted, want exactly 1", count)
	}
	var reserved int64
	if err := f.pool.QueryRow(f.pilgrimCtx, `SELECT COALESCE(SUM(amount_idr),0) FROM pilgrim_refund_payout_requests WHERE pilgrim_id = $1 AND status IN ('REQUESTED','PROCESSING')`, f.pilgrimID).Scan(&reserved); err != nil || reserved != 700_000 {
		t.Fatalf("reserved = %d, %v; want 700000", reserved, err)
	}
}

func TestDatabaseRejectsOverReservationAndPaidWithoutWithdrawalIntegration(t *testing.T) {
	f := newRefundPayoutFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `INSERT INTO pilgrim_refund_payout_requests (operator_id,pilgrim_id,amount_idr,method,idempotency_key,requested_by_user_id) VALUES ($1,$2,1000001,'CASH',$3,'direct')`, f.operatorID, f.pilgrimID, uuid.NewString()); err == nil {
		t.Fatal("database accepted a payout larger than the ledger balance")
	}
	var requestID string
	if err := f.pool.QueryRow(ctx, `INSERT INTO pilgrim_refund_payout_requests (operator_id,pilgrim_id,amount_idr,method,idempotency_key,requested_by_user_id) VALUES ($1,$2,100000,'CASH',$3,'direct') RETURNING id::text`, f.operatorID, f.pilgrimID, uuid.NewString()).Scan(&requestID); err != nil {
		t.Fatalf("insert valid request: %v", err)
	}
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE pilgrim_refund_payout_requests SET status='PROCESSING' WHERE id=$1`, requestID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE pilgrim_refund_payout_requests SET status='PAID' WHERE id=$1`, requestID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err == nil {
		t.Fatal("database committed PAID without a matching withdrawal ledger entry")
	}
}
