package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestSupportAdminInboxIsCrossTenantAndCannotCloseIntegration is C5
// (TUGAS-PANEL-SAAS.md): a platform admin — a member of their own operator's
// organisation, nothing else — must be able to see, reply to, and change the
// status of a ticket belonging to a COMPLETELY DIFFERENT tenant, and must
// never be able to reach CLOSED, which belongs only to the operator that
// owns the ticket.
func TestSupportAdminInboxIsCrossTenantAndCannotCloseIntegration(t *testing.T) {
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

	// The admin's own tenant — membership here must grant nothing about the
	// other tenant below.
	fixture := newHTTPFixture(t, pool)
	var adminUserID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&adminUserID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id, note) VALUES ($1, 'uji kotak masuk support')`, adminUserID); err != nil {
		t.Fatalf("grant platform admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, adminUserID); err != nil {
		t.Fatalf("enrol 2fa: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins WHERE user_id = $1`, adminUserID)
	})

	// A second, entirely unrelated tenant with its own ticket.
	otherOperatorID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug)
		VALUES ($1, $2, 'Travel Lain', 'ID', $3, $4)`,
		otherOperatorID, "other-org-"+uuid.NewString(), otherOperatorID[:8]+"@example.test", "other-"+otherOperatorID[:8]); err != nil {
		t.Fatalf("fixture other operator: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, otherOperatorID)
	})
	queries := db.New(pool)
	supportRepository := repository.NewSupportRepository(queries)
	ticket, err := supportRepository.Create(ctx, otherOperatorID, "Pembayaran tidak masuk", "URGENT", "staff-other", "Uang sudah ditransfer tapi status masih pending.", "Staf Travel Lain")
	if err != nil {
		t.Fatalf("fixture ticket: %v", err)
	}

	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool),
		repository.NewKYCRepository(pool), repository.NewAuditRepository(queries),
		repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool),
		repository.NewPersonalDataReadRepository(pool), nil, supportRepository, repository.NewDataExportRepository(pool), repository.NewAnnouncementRepository(pool))
	path, serviceHandler := hajjv1connect.NewPlatformServiceHandler(
		handler.NewPlatformHandler(platform),
		connect.WithInterceptors(middleware.NewAuthInterceptor(pool,
			repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)),
			repository.NewSubscriptionRepository(pool))),
	)
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := hajjv1connect.NewPlatformServiceClient(server.Client(), server.URL)

	authed := func() *connect.Request[hajjv1.ListAllSupportTicketsRequest] {
		req := connect.NewRequest(&hajjv1.ListAllSupportTicketsRequest{})
		req.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
		return req
	}

	// Cross-tenant visibility: the admin, a member of a different
	// organisation entirely, sees the other tenant's ticket.
	list, err := client.ListAllSupportTickets(ctx, authed())
	if err != nil {
		t.Fatalf("ListAllSupportTickets: %v", err)
	}
	found := false
	for _, item := range list.Msg.Tickets {
		if item.Id == ticket.ID {
			found = true
			if item.OperatorName != "Travel Lain" {
				t.Fatalf("nama travel di daftar = %q, mau %q", item.OperatorName, "Travel Lain")
			}
		}
	}
	if !found {
		t.Fatal("tiket travel lain tidak muncul di kotak masuk platform")
	}

	// Reply as platform staff, and the operator's own read path must show it
	// without any change on that side — same rows, same query.
	replyReq := connect.NewRequest(&hajjv1.ReplyToSupportTicketAsPlatformRequest{TicketId: ticket.ID, Body: "Sudah kami cek, mohon tunggu 1x24 jam."})
	replyReq.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	reply, err := client.ReplyToSupportTicketAsPlatform(ctx, replyReq)
	if err != nil {
		t.Fatalf("ReplyToSupportTicketAsPlatform: %v", err)
	}
	if !reply.Msg.AuthorIsPlatform {
		t.Fatal("balasan staf platform seharusnya author_is_platform=true")
	}
	_, messages, err := supportRepository.GetAsPlatform(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("GetAsPlatform: %v", err)
	}
	sawReply := false
	for _, m := range messages {
		if m.AuthorIsPlatform && m.Body == "Sudah kami cek, mohon tunggu 1x24 jam." {
			sawReply = true
		}
	}
	if !sawReply {
		t.Fatal("balasan platform tidak muncul di thread yang sama")
	}

	// F5 (TUGAS-PANEL-SAAS.md): a platform admin writing into another
	// tenant's own support thread must leave a trace on that tenant's audit
	// trail, scoped to them — not blank, and not on the admin's own tenant.
	var replyAuditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs
		WHERE operator_id = $1 AND user_id = $2 AND action = 'support_ticket_replied_as_platform' AND entity_id = $3`,
		otherOperatorID, adminUserID, ticket.ID).Scan(&replyAuditCount); err != nil {
		t.Fatal(err)
	}
	if replyAuditCount != 1 {
		t.Fatalf("%d jejak audit balasan platform pada travel lain, mau 1", replyAuditCount)
	}

	// Status: OPEN -> IN_PROGRESS is allowed.
	statusReq := connect.NewRequest(&hajjv1.SetSupportTicketStatusRequest{TicketId: ticket.ID, Status: "IN_PROGRESS"})
	statusReq.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	updated, err := client.SetSupportTicketStatus(ctx, statusReq)
	if err != nil {
		t.Fatalf("SetSupportTicketStatus: %v", err)
	}
	if updated.Msg.Status != "IN_PROGRESS" {
		t.Fatalf("status = %q, mau IN_PROGRESS", updated.Msg.Status)
	}
	var statusAuditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs
		WHERE operator_id = $1 AND user_id = $2 AND action = 'support_ticket_status_set_as_platform' AND entity_id = $3`,
		otherOperatorID, adminUserID, ticket.ID).Scan(&statusAuditCount); err != nil {
		t.Fatal(err)
	}
	if statusAuditCount != 1 {
		t.Fatalf("%d jejak audit perubahan status platform pada travel lain, mau 1", statusAuditCount)
	}

	// CLOSED is unreachable from this RPC — buf.validate rejects it before
	// the request even reaches the service.
	closeReq := connect.NewRequest(&hajjv1.SetSupportTicketStatusRequest{TicketId: ticket.ID, Status: "CLOSED"})
	closeReq.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
	if _, err := client.SetSupportTicketStatus(ctx, closeReq); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("mengatur status ke CLOSED lewat RPC platform seharusnya ditolak, dapat %v (%s)", err, connect.CodeOf(err))
	}

	// Defence in depth, checked directly at the repository — proto validation
	// on the request already blocks "CLOSED" above, but the SQL guard must
	// also refuse the value itself, not just an already-closed row, or a
	// caller bypassing the proto layer could still set a ticket straight to
	// CLOSED through this same query. The query is :one, so a WHERE clause
	// that matches nothing surfaces as ErrNotFound, not a returned row.
	if _, err := supportRepository.SetStatus(ctx, ticket.ID, "CLOSED"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("SetStatus(CLOSED) di level repository seharusnya ditolak WHERE guard (not found), dapat %v", err)
	}

	// And the operator's own CloseSupportTicket still works afterward — the
	// repository-level WHERE guard did not somehow lock the ticket out of its
	// real closing path.
	if _, err := supportRepository.Close(ctx, otherOperatorID, ticket.ID); err != nil {
		t.Fatalf("operator sendiri seharusnya tetap bisa menutup tiketnya: %v", err)
	}
}
