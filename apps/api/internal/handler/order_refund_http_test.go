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

// Everything below the service was previously unexercised: the Connect
// handler, protovalidate, the auth interceptor's session lookup, and the wire
// contract itself. A service-level test would still pass if the RPC were
// unreachable, unauthenticated, or rejected by validation.
//
// This drives a real HTTP server with a real Connect client. The session it
// authenticates with is one it creates itself, in the test database, and
// removes afterwards.
func TestRefundOrderOverHTTPIntegration(t *testing.T) {
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
	queries := db.New(pool)
	ledger := repository.NewLedgerRepository(pool)
	orderService := service.NewOrderService(
		repository.NewOperatorRepository(queries), repository.NewPilgrimRepository(queries),
		repository.NewProductRepository(queries, pool), repository.NewOrderRepository(queries, pool),
		repository.NewAuditRepository(queries), ledger, repository.NewRefundRepository(pool), repository.NewAgentRepository(queries), repository.NewSeasonRepository(queries),
		pool, nil, "http://localhost:3000")

	path, serviceHandler := hajjv1connect.NewOrderServiceHandler(
		handler.NewOrderHandler(orderService),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool,
			repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)), repository.NewSubscriptionRepository(pool))),
	)
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := hajjv1connect.NewOrderServiceClient(server.Client(), server.URL)

	authorised := func(req *hajjv1.RefundOrderRequest) *connect.Request[hajjv1.RefundOrderRequest] {
		request := connect.NewRequest(req)
		request.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
		return request
	}

	// Unauthenticated callers must not reach the refund path at all.
	if _, err := client.RefundOrder(ctx, connect.NewRequest(&hajjv1.RefundOrderRequest{
		OrderId: fixture.orderID, IdempotencyKey: uuid.NewString(),
	})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("unauthenticated refund returned %v (%v), want unauthenticated", connect.CodeOf(err), err)
	}

	// protovalidate runs in the handler, before any business logic: the
	// idempotency key is required by the schema, not by a service check.
	if _, err := client.RefundOrder(ctx, authorised(&hajjv1.RefundOrderRequest{
		OrderId: fixture.orderID,
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("missing idempotency key returned %v (%v), want invalid_argument", connect.CodeOf(err), err)
	}
	if _, err := client.RefundOrder(ctx, authorised(&hajjv1.RefundOrderRequest{
		OrderId: "not-a-uuid", IdempotencyKey: uuid.NewString(),
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("malformed order id returned %v, want invalid_argument", connect.CodeOf(err))
	}

	key := uuid.NewString()
	response, err := client.RefundOrder(ctx, authorised(&hajjv1.RefundOrderRequest{
		OrderId: fixture.orderID, Reason: "uji http", IdempotencyKey: key,
	}))
	if err != nil {
		t.Fatalf("refund over HTTP: %v", err)
	}
	if !response.Msg.Created {
		t.Fatal("the first refund reported as a replay")
	}
	if response.Msg.Order.Status != "REFUNDED" || response.Msg.Refund.AmountIdr != httpOrderTotal {
		t.Fatalf("status=%s amount=%d, want REFUNDED and %d", response.Msg.Order.Status, response.Msg.Refund.AmountIdr, httpOrderTotal)
	}
	if response.Msg.PilgrimBalanceIdr != httpOrderTotal {
		t.Fatalf("pilgrim balance = %d, want %d", response.Msg.PilgrimBalanceIdr, httpOrderTotal)
	}

	// The replay contract, exercised across the wire rather than in-process.
	replay, err := client.RefundOrder(ctx, authorised(&hajjv1.RefundOrderRequest{
		OrderId: fixture.orderID, Reason: "uji http", IdempotencyKey: key,
	}))
	if err != nil {
		t.Fatalf("replay over HTTP: %v", err)
	}
	if replay.Msg.Created || replay.Msg.Refund.Id != response.Msg.Refund.Id {
		t.Fatalf("replay created=%v id=%s, want false and %s", replay.Msg.Created, replay.Msg.Refund.Id, response.Msg.Refund.Id)
	}

	listRequest := connect.NewRequest(&hajjv1.ListOrderRefundsRequest{OrderId: fixture.orderID})
	listRequest.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	refunds, err := client.ListOrderRefunds(ctx, listRequest)
	if err != nil {
		t.Fatalf("list refunds: %v", err)
	}
	if len(refunds.Msg.Refunds) != 1 || refunds.Msg.TotalRefundedIdr != httpOrderTotal {
		t.Fatalf("%d refunds totalling %d, want 1 totalling %d",
			len(refunds.Msg.Refunds), refunds.Msg.TotalRefundedIdr, httpOrderTotal)
	}

	// A lapsed subscription must lock a money endpoint, not just the UI that
	// calls it. The gate lives in the interceptor, so only a test that goes
	// through real HTTP can show it applies here.
	if _, err := pool.Exec(ctx, `INSERT INTO subscriptions (operator_id, plan, status, access_until)
		VALUES ($1, 'STARTER', 'PAST_DUE', NOW() - INTERVAL '1 day')`, fixture.operatorID); err != nil {
		t.Fatalf("lapse subscription: %v", err)
	}
	if _, err := client.RefundOrder(ctx, authorised(&hajjv1.RefundOrderRequest{
		OrderId: fixture.orderID, IdempotencyKey: uuid.NewString(),
	})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("refund with a lapsed subscription returned %v (%v), want failed_precondition",
			connect.CodeOf(err), err)
	}
}

const httpOrderTotal = int64(2_500_000)

type httpFixture struct {
	sessionToken string
	orderID      string
	operatorID   string
}

// Builds a complete authenticated tenant: a Better Auth user, organization,
// membership and session, plus the operator they belong to and one paid order.
// The session token is generated here, never read from anyone's real session.
func newHTTPFixture(t *testing.T, pool *pgxpool.Pool) *httpFixture {
	t.Helper()
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture %q: %v", query, err)
		}
	}

	userID := "http-user-" + uuid.NewString()
	orgID := "http-org-" + uuid.NewString()
	operatorID := uuid.NewString()
	token := "http-session-" + uuid.NewString()

	exec(`INSERT INTO "user" (id, name, email, "emailVerified") VALUES ($1, 'Staf Uji', $2, true)`, userID, userID+"@example.test")
	exec(`INSERT INTO organization (id, name, slug, "createdAt") VALUES ($1, 'Org Uji', $2, NOW())`, orgID, "org-"+uuid.NewString()[:8])
	exec(`INSERT INTO member (id, "organizationId", "userId", role, "createdAt") VALUES ($1, $2, $3, 'owner', NOW())`,
		uuid.NewString(), orgID, userID)
	exec(`INSERT INTO session (id, "expiresAt", token, "updatedAt", "userId", "activeOrganizationId")
	      VALUES ($1, NOW() + INTERVAL '1 day', $2, NOW(), $3, $4)`, uuid.NewString(), token, userID, orgID)
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug)
	      VALUES ($1, $2, 'Operator HTTP', 'ID', $3, $4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "http-"+operatorID[:8])

	seasonID, pilgrimID, productID, agentID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1, $2, 'Musim HTTP', 'UMRAH_REGULER', NOW(), NOW() + INTERVAL '30 days', 10)`, seasonID, operatorID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1, $2, $3, 'Jamaah HTTP', 'P-HTTP', 'ID', '1990-01-01'::timestamptz, 'MALE')`, pilgrimID, seasonID, operatorID)
	exec(`INSERT INTO products (id, operator_id, season_id, name, price_idr) VALUES ($1, $2, $3, 'Produk HTTP', $4)`,
		productID, operatorID, seasonID, httpOrderTotal)
	exec(`INSERT INTO agents (id, operator_id, name) VALUES ($1, $2, 'Agen HTTP')`, agentID, operatorID)

	var orderID string
	if err := pool.QueryRow(ctx, `INSERT INTO orders
		(operator_id, season_id, pilgrim_id, product_id, agent_id, unit_price_idr, total_price_idr, agent_commission_idr, status, paid_at)
		VALUES ($1,$2,$3,$4,$5,$6,$6,100000,'PAID',NOW()) RETURNING id::text`,
		operatorID, seasonID, pilgrimID, productID, agentID, httpOrderTotal).Scan(&orderID); err != nil {
		t.Fatalf("fixture order: %v", err)
	}
	if err := repository.NewLedgerRepository(pool).AppendCommission(ctx, repository.CommissionEntry{
		OperatorID: operatorID, AgentID: agentID, AmountIDR: 100_000, Kind: "EARNED",
		OrderID: orderID, IdempotencyKey: "order-earned-" + orderID,
	}); err != nil {
		t.Fatalf("fixture commission: %v", err)
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
		for _, statement := range []string{
			`DELETE FROM order_refunds WHERE operator_id = $1`,
			`DELETE FROM operators WHERE id = $1`,
		} {
			if _, err := cleanup.Exec(context.Background(), statement, operatorID); err != nil {
				return
			}
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID); err != nil {
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DELETE FROM organization WHERE id = $1`, orgID); err != nil {
			return
		}
		_ = cleanup.Commit(context.Background())
	})

	return &httpFixture{sessionToken: token, orderID: orderID, operatorID: operatorID}
}
