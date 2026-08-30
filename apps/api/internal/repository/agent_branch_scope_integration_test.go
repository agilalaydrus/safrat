package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAgentRepositoryEnforcesBranchScopeIntegration(t *testing.T) {
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
	bandungID, medanID := uuid.NewString(), uuid.NewString()
	bandungHead := "agent-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug)
	      VALUES ($1,$2,'Agent Scope Test','ID',$3,$4)`, operatorID,
		"agent-scope-"+uuid.NewString(), operatorID[:8]+"@example.test", "agent-"+operatorID[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandungID, medanID, operatorID)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, bandungHead, bandungID, operatorID)

	repo := NewAgentRepository(db.New(pool))
	bandungCtx := ContextWithStaffActor(ctx, bandungHead)
	bandungAgent, err := repo.Create(bandungCtx, operatorID, "Agen Bandung", "", "", "", 0)
	if err != nil {
		t.Fatalf("create Bandung: %v", err)
	}
	medanAgentID := uuid.NewString()
	exec(`INSERT INTO agents (id, operator_id, branch_id, name) VALUES ($1,$2,$3,'Agen Medan')`, medanAgentID, operatorID, medanID)

	var storedBranch string
	if err := pool.QueryRow(ctx, `SELECT branch_id::text FROM agents WHERE id=$1`, bandungAgent.ID).Scan(&storedBranch); err != nil || storedBranch != bandungID {
		t.Fatalf("agen baru tidak mewarisi Bandung: branch=%s err=%v", storedBranch, err)
	}
	rows, err := repo.ListByOperatorID(bandungCtx, operatorID)
	if err != nil || len(rows) != 1 || rows[0].ID != bandungAgent.ID {
		t.Fatalf("roster agen Bandung bocor: %#v (%v)", rows, err)
	}
	if _, err := repo.GetByID(bandungCtx, operatorID, medanAgentID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca agen Medan: %v", err)
	}
	if _, err := repo.Update(bandungCtx, operatorID, medanAgentID, "Diubah", "", "", "", 0, true); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung mengubah agen Medan: %v", err)
	}
	if _, err := repo.VerifyKYC(bandungCtx, operatorID, medanAgentID, bandungHead, true, ""); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung memverifikasi KYC Medan: %v", err)
	}
	if _, err := repo.CreateDocument(bandungCtx, operatorID, medanAgentID, "KTP", "/forbidden", "forbidden.pdf", "test"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menambah dokumen agen Medan: %v", err)
	}
	if err := repo.Delete(bandungCtx, operatorID, medanAgentID); err != nil {
		t.Fatalf("delete terlindungi seharusnya no-op: %v", err)
	}
	var medanStillExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agents WHERE id=$1)`, medanAgentID).Scan(&medanStillExists); err != nil || !medanStillExists {
		t.Fatalf("kepala Bandung menghapus agen Medan: exists=%v err=%v", medanStillExists, err)
	}
	if _, err := repo.CreatePayoutRequest(bandungCtx, operatorID, medanAgentID, 100_000, "forbidden"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membuat permintaan payout untuk Medan: %v", err)
	}
	bandungRequestID, medanRequestID := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO agent_payout_requests (id, operator_id, agent_id, amount_idr, note)
	      VALUES ($1,$3,$4,100000,'Bandung'),($2,$3,$5,100000,'Medan')`,
		bandungRequestID, medanRequestID, operatorID, bandungAgent.ID, medanAgentID)
	requests, err := repo.ListPayoutRequests(bandungCtx, operatorID, "")
	if err != nil || len(requests) != 1 || requests[0].ID != bandungRequestID {
		t.Fatalf("inbox payout Bandung bocor: %#v (%v)", requests, err)
	}
	if _, err := repo.GetPayoutRequest(bandungCtx, operatorID, medanRequestID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca request payout Medan: %v", err)
	}
	if _, err := repo.RejectPayoutRequest(bandungCtx, operatorID, medanRequestID, bandungHead, "forbidden"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung menolak payout Medan: %v", err)
	}
	if _, err := repo.GetPayoutSummary(bandungCtx, operatorID, medanAgentID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca saldo agen Medan: %v", err)
	}
	payouts, err := repo.ListPayouts(bandungCtx, operatorID)
	if err != nil || len(payouts) != 1 || payouts[0].AgentID != bandungAgent.ID {
		t.Fatalf("ringkasan payout Bandung bocor: %#v (%v)", payouts, err)
	}

	application, err := repo.CreateApplication(ctx, operatorID, "Calon Agen Bandung", "", "", bandungAgent.ID)
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT branch_id::text FROM agents WHERE id=$1`, application.ID).Scan(&storedBranch); err != nil || storedBranch != bandungID {
		t.Fatalf("aplikasi tidak mewarisi cabang referrer: branch=%s err=%v", storedBranch, err)
	}

	headOfficeCtx := ContextWithStaffActor(ctx, "agent-hq-"+uuid.NewString())
	rows, err = repo.ListByOperatorID(headOfficeCtx, operatorID)
	if err != nil || len(rows) != 3 {
		t.Fatalf("kantor pusat harus melihat semua agen: len=%d err=%v", len(rows), err)
	}
}
