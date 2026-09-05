package service

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AddSupportTicketMessage must refuse to post into a ticket that does not
// belong to the calling operator — a plain insert keyed only by ticket_id
// would let any operator that merely guesses another tenant's ticket id
// write into their support thread. Also proves an URGENT ticket's response
// target is 1 hour, not the MEDIUM default.
func TestSupportTicketTenantIsolationAndResponseTargetIntegration(t *testing.T) {
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

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture %q: %v", query, err)
		}
	}

	operatorAID, orgAID := uuid.NewString(), "sup-a-"+uuid.NewString()
	operatorBID, orgBID := uuid.NewString(), "sup-b-"+uuid.NewString()

	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Sup A','ID',$3,$4)`,
		operatorAID, orgAID, operatorAID[:8]+"@example.test", "sup-a-"+operatorAID[:8])
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Sup B','ID',$3,$4)`,
		operatorBID, orgBID, operatorBID[:8]+"@example.test", "sup-b-"+operatorBID[:8])

	t.Cleanup(func() {
		exec(`DELETE FROM support_tickets WHERE operator_id IN ($1,$2)`, operatorAID, operatorBID)
		exec(`DELETE FROM operators WHERE id IN ($1,$2)`, operatorAID, operatorBID)
	})

	queries := db.New(pool)
	svc := NewSupportService(repository.NewOperatorRepository(queries), repository.NewSupportRepository(queries))

	ticketA, err := svc.CreateSupportTicket(ctx, orgAID, "user-a", "Owner A", &hajjv1.CreateSupportTicketRequest{
		Subject: "Tidak bisa login", Priority: "URGENT", Body: "Sejak pagi ini tidak bisa masuk dashboard.",
	})
	if err != nil {
		t.Fatalf("CreateSupportTicket: %v", err)
	}
	if ticketA.Status != "OPEN" {
		t.Fatalf("status awal = %s, mau OPEN", ticketA.Status)
	}
	wantDue := ticketA.CreatedAt.AsTime().Add(60 * 60 * 1e9) // 1 hour, in nanoseconds
	if got := ticketA.ResponseDueAt.AsTime(); !got.Equal(wantDue) {
		t.Fatalf("response_due_at untuk URGENT = %v, mau tepat 1 jam setelah dibuat (%v)", got, wantDue)
	}

	// Tenant isolation: operator B must not be able to post into operator A's ticket.
	if _, err := svc.AddSupportTicketMessage(ctx, orgBID, "user-b", "Owner B", &hajjv1.AddSupportTicketMessageRequest{
		TicketId: ticketA.Id, Body: "mencoba menyusup",
	}); err == nil {
		t.Fatalf("AddSupportTicketMessage mengizinkan operator B menulis ke tiket operator A")
	}

	// Same operator: a follow-up message succeeds and shows up in the thread.
	if _, err := svc.AddSupportTicketMessage(ctx, orgAID, "user-a", "Owner A", &hajjv1.AddSupportTicketMessageRequest{
		TicketId: ticketA.Id, Body: "Masih belum bisa, sudah coba reset password juga.",
	}); err != nil {
		t.Fatalf("AddSupportTicketMessage (milik sendiri): %v", err)
	}
	detail, err := svc.GetSupportTicket(ctx, orgAID, &hajjv1.GetSupportTicketRequest{TicketId: ticketA.Id})
	if err != nil {
		t.Fatalf("GetSupportTicket: %v", err)
	}
	if len(detail.Messages) != 2 {
		t.Fatalf("jumlah pesan = %d, mau 2 (pesan pembuka + balasan)", len(detail.Messages))
	}

	// Operator B's own ticket list must never show operator A's ticket.
	listB, err := svc.ListMySupportTickets(ctx, orgBID, &hajjv1.ListMySupportTicketsRequest{})
	if err != nil {
		t.Fatalf("ListMySupportTickets (B): %v", err)
	}
	for _, ticket := range listB.Tickets {
		if ticket.Id == ticketA.Id {
			t.Fatalf("tiket operator A bocor ke daftar operator B")
		}
	}
}
