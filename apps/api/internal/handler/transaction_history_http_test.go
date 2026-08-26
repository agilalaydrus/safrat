package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A jamaah's payment history is not reachable with app_access_code alone, the
// way the rest of PilgrimAppService is. It needs a real session, and the code
// presented has to belong to that session's own pilgrim — otherwise a leaked
// code, or any signed-in account, could read somebody else's money.
func TestPilgrimTransactionHistoryRequiresBothSessionAndOwnCodeIntegration(t *testing.T) {
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

	fixture := newHTTPFixture(t, pool)
	// Link the jamaah to their own signed-in account, the way the Better Auth
	// session hook does when a pilgrim signs in.
	pilgrimUserID := "pilgrim-user-" + uuid.NewString()
	pilgrimToken := "pilgrim-session-" + uuid.NewString()
	mustExec(t, pool, `INSERT INTO "user" (id, name, email, "emailVerified") VALUES ($1, 'Jamaah HTTP', $2, true)`,
		pilgrimUserID, pilgrimUserID+"@example.test")
	mustExec(t, pool, `INSERT INTO session (id, "expiresAt", token, "updatedAt", "userId") VALUES ($1, NOW() + INTERVAL '1 day', $2, NOW(), $3)`,
		uuid.NewString(), pilgrimToken, pilgrimUserID)
	mustExec(t, pool, `UPDATE pilgrims SET linked_user_id = $1 WHERE id = (SELECT pilgrim_id FROM orders WHERE id = $2)`,
		pilgrimUserID, fixture.orderID)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, pilgrimUserID) })

	var accessCode string
	if err := pool.QueryRow(ctx, `SELECT app_access_code FROM pilgrims WHERE linked_user_id = $1`, pilgrimUserID).Scan(&accessCode); err != nil {
		t.Fatalf("read access code: %v", err)
	}

	queries := db.New(pool)
	identity := repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries))
	pilgrimApp := service.NewPilgrimAppService(
		repository.NewPilgrimRepository(queries), repository.NewProductRepository(queries, pool),
		repository.NewAuditRepository(queries), identity, repository.NewBroadcastRepository(queries),
		repository.NewJourneyRepository(queries), repository.NewRitualRepository(queries),
		repository.NewNotificationRepository(queries), repository.NewOrderRepository(queries),
		repository.NewLedgerRepository(pool))

	path, serviceHandler := hajjv1connect.NewPilgrimAppServiceHandler(
		handler.NewPilgrimAppHandler(pilgrimApp),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool, identity, repository.NewSubscriptionRepository(pool))),
	)
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := hajjv1connect.NewPilgrimAppServiceClient(server.Client(), server.URL)

	call := func(token, code string) (*connect.Response[hajjv1.ListMyTransactionsResponse], error) {
		request := connect.NewRequest(&hajjv1.ListMyTransactionsRequest{AppAccessCode: code})
		if token != "" {
			request.Header().Set("Authorization", "Bearer "+token)
		}
		return client.ListMyTransactions(ctx, request)
	}

	// The access code alone — which is all the rest of this service needs —
	// must not be enough here.
	if _, err := call("", accessCode); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("code without a session returned %v (%v), want unauthenticated", connect.CodeOf(err), err)
	}

	// A real session belonging to somebody else, presenting this jamaah's code.
	if _, err := call(fixture.sessionToken, accessCode); err == nil {
		t.Fatal("a staff session read a jamaah's transaction history")
	}

	// The jamaah's own session, but somebody else's code.
	if _, err := call(pilgrimToken, "not-this-pilgrims-code"); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("mismatched code returned %v (%v), want permission_denied", connect.CodeOf(err), err)
	}

	// Both correct.
	response, err := call(pilgrimToken, accessCode)
	if err != nil {
		t.Fatalf("own history: %v", err)
	}
	if len(response.Msg.Transactions) != 1 {
		t.Fatalf("%d transactions, want 1", len(response.Msg.Transactions))
	}
	transaction := response.Msg.Transactions[0]
	if transaction.Status != "PAID" || transaction.AmountIdr != httpOrderTotal {
		t.Fatalf("status=%s amount=%d, want PAID and %d", transaction.Status, transaction.AmountIdr, httpOrderTotal)
	}
	if response.Msg.TotalPaidIdr != httpOrderTotal || response.Msg.BalanceIdr != 0 {
		t.Fatalf("paid=%d balance=%d, want %d and 0", response.Msg.TotalPaidIdr, response.Msg.BalanceIdr, httpOrderTotal)
	}

	// After a refund the transaction must still be listed — visibly refunded,
	// not quietly gone — and the balance must show the money being held.
	refundOrder(t, pool, fixture)
	response, err = call(pilgrimToken, accessCode)
	if err != nil {
		t.Fatalf("history after refund: %v", err)
	}
	if len(response.Msg.Transactions) != 1 {
		t.Fatalf("%d transactions after a refund, want the refunded one to remain", len(response.Msg.Transactions))
	}
	transaction = response.Msg.Transactions[0]
	if transaction.Status != "REFUNDED" || transaction.RefundedIdr != httpOrderTotal {
		t.Fatalf("status=%s refunded=%d, want REFUNDED and %d", transaction.Status, transaction.RefundedIdr, httpOrderTotal)
	}
	if response.Msg.TotalPaidIdr != 0 {
		t.Fatalf("total paid = %d after a full refund, want 0", response.Msg.TotalPaidIdr)
	}
	if response.Msg.BalanceIdr != httpOrderTotal {
		t.Fatalf("balance = %d, want the refunded %d held for the jamaah", response.Msg.BalanceIdr, httpOrderTotal)
	}
}

// The agent's recap must report what survived, not what was once transacted.
func TestAgentReferredTransactionRecapIsNetOfRefundsIntegration(t *testing.T) {
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

	fixture := newHTTPFixture(t, pool)
	// The agent portal is self-scoped from the caller's Better Auth identity,
	// so the staff user has to be the linked agent.
	mustExec(t, pool, `UPDATE agents SET linked_user_id = (SELECT "userId" FROM session WHERE token = $1) WHERE operator_id = $2`,
		fixture.sessionToken, fixture.operatorID)

	queries := db.New(pool)
	identity := repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries))
	agentService := service.NewAgentService(repository.NewOperatorRepository(queries),
		repository.NewAgentRepository(queries), repository.NewAuditRepository(queries), pool)

	path, serviceHandler := hajjv1connect.NewAgentServiceHandler(
		handler.NewAgentHandler(agentService),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool, identity, repository.NewSubscriptionRepository(pool))),
	)
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := hajjv1connect.NewAgentServiceClient(server.Client(), server.URL)

	call := func() *hajjv1.ListMyReferredTransactionsResponse {
		t.Helper()
		request := connect.NewRequest(&hajjv1.ListMyReferredTransactionsRequest{})
		request.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
		response, err := client.ListMyReferredTransactions(ctx, request)
		if err != nil {
			t.Fatalf("recap: %v", err)
		}
		return response.Msg
	}

	recap := call()
	if len(recap.Customers) != 1 {
		t.Fatalf("%d customers, want 1", len(recap.Customers))
	}
	if recap.TotalPaidIdr != httpOrderTotal || recap.TotalCommissionIdr != 100_000 {
		t.Fatalf("paid=%d commission=%d, want %d and 100000", recap.TotalPaidIdr, recap.TotalCommissionIdr, httpOrderTotal)
	}

	refundOrder(t, pool, fixture)

	recap = call()
	if len(recap.Customers) != 1 {
		t.Fatalf("the customer disappeared from the recap after a refund")
	}
	customer := recap.Customers[0]
	if customer.RefundedOrderCount != 1 || customer.RefundedIdr != httpOrderTotal {
		t.Fatalf("refunded count=%d amount=%d, want 1 and %d", customer.RefundedOrderCount, customer.RefundedIdr, httpOrderTotal)
	}
	if recap.TotalPaidIdr != 0 {
		t.Fatalf("total paid = %d after a refund, want 0", recap.TotalPaidIdr)
	}
	if recap.TotalCommissionIdr != 0 {
		t.Fatalf("commission = %d after a refund, want 0 — it is earned only on sales that stand", recap.TotalCommissionIdr)
	}
}

func mustExec(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("%q: %v", query, err)
	}
}

// Refunds through the service, so the test exercises the real path rather than
// hand-writing rows the service would have written differently.
func refundOrder(t *testing.T, pool *pgxpool.Pool, fixture *httpFixture) {
	t.Helper()
	queries := db.New(pool)
	orders := service.NewOrderService(
		repository.NewOperatorRepository(queries), repository.NewPilgrimRepository(queries),
		repository.NewProductRepository(queries, pool), repository.NewOrderRepository(queries),
		repository.NewAuditRepository(queries), repository.NewLedgerRepository(pool),
		repository.NewRefundRepository(pool), pool, nil, "http://localhost:3000")
	var orgID string
	if err := pool.QueryRow(context.Background(), `SELECT better_auth_org_id FROM operators WHERE id = $1`, fixture.operatorID).Scan(&orgID); err != nil {
		t.Fatalf("read org: %v", err)
	}
	if _, err := orders.RefundOrder(context.Background(), orgID, "test-user", &hajjv1.RefundOrderRequest{
		OrderId: fixture.orderID, Reason: "uji", IdempotencyKey: uuid.NewString(),
	}); err != nil {
		t.Fatalf("refund: %v", err)
	}
}
