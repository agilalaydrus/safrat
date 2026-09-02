package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCRMRepositoryTimelineEntitlementIdempotencyAndBranchScopeIntegration(t *testing.T) {
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

	op, starter := uuid.NewString(), uuid.NewString()
	org := "crm-org-" + uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString()
	bandungHead, medanHead := "crm-bdg-"+uuid.NewString(), "crm-mdn-"+uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO organization (id,name,slug,"createdAt") VALUES ($1,'CRM Scope',$2,NOW())`, org, "crm-org-"+op[:8])
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan)
		VALUES ($1,$2,'CRM Scope','ID',$3,$4,'GROWTH'),
		       ($5,$6,'CRM Starter','ID',$7,$8,'STARTER')`,
		op, org, op[:8]+"@example.test", "crm-"+op[:8], starter, "crm-starter-"+uuid.NewString(), starter[:8]+"@example.test", "crm-starter-"+starter[:8])
	exec(`INSERT INTO "user" (id,name,email,"emailVerified") VALUES
		($1,'Kepala Bandung',$2,true),($3,'Kepala Medan',$4,true)`, bandungHead, bandungHead+"@example.test", medanHead, medanHead+"@example.test")
	exec(`INSERT INTO "member" (id,"organizationId","userId",role,"createdAt") VALUES
		($1,$2,$3,'admin',NOW()),($4,$2,$5,'admin',NOW())`, uuid.NewString(), org, bandungHead, uuid.NewString(), medanHead)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES
		($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES
		($1,$2,$4),($3,$5,$4)`, bandungHead, bandung, medanHead, op, medan)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM "member" WHERE "organizationId"=$1`, org)
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id IN ($1,$2)`, op, starter)
		_, _ = pool.Exec(context.Background(), `DELETE FROM "user" WHERE id IN ($1,$2)`, bandungHead, medanHead)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organization WHERE id=$1`, org)
	})

	repo := NewCRMRepository(pool)
	next := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	key := "crm-create-" + uuid.NewString()
	draft := domain.CRMLeadDraft{FullName: "Prospek Bandung", Phone: "0812345678", Source: "WHATSAPP", Pax: 2,
		EstimatedValueIDR: 50_000_000, NextAction: "Hubungi kembali", NextFollowUpAt: &next,
		Note: "Masuk dari iklan", IdempotencyKey: key}
	bandungCtx := ContextWithStaffActor(ctx, bandungHead)
	lead, created, err := repo.CreateLead(bandungCtx, op, bandungHead, draft)
	if err != nil || !created || lead.BranchID != bandung {
		t.Fatalf("buat lead Bandung: created=%v lead=%#v err=%v", created, lead, err)
	}
	replay, created, err := repo.CreateLead(bandungCtx, op, bandungHead, draft)
	if err != nil || created || replay.ID != lead.ID {
		t.Fatalf("replay create membuat lead baru: created=%v lead=%#v err=%v", created, replay, err)
	}
	conflicting := draft
	conflicting.FullName = "Payload berbeda"
	if _, _, err := repo.CreateLead(bandungCtx, op, bandungHead, conflicting); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("idempotency create menerima payload berbeda: %v", err)
	}

	medanDraft := draft
	medanDraft.FullName = "Prospek Medan"
	medanDraft.IdempotencyKey = "crm-medan-" + uuid.NewString()
	medanLead, _, err := repo.CreateLead(ContextWithStaffActor(ctx, medanHead), op, medanHead, medanDraft)
	if err != nil || medanLead.BranchID != medan {
		t.Fatalf("buat lead Medan: %#v (%v)", medanLead, err)
	}
	bandungRows, _, err := repo.ListLeads(bandungCtx, op, domain.CRMLeadFilter{})
	if err != nil || len(bandungRows) != 1 || bandungRows[0].ID != lead.ID {
		t.Fatalf("papan Bandung bocor: %#v (%v)", bandungRows, err)
	}
	if _, err := repo.GetLead(bandungCtx, op, medanLead.ID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung membaca lead Medan: %v", err)
	}
	if _, _, err := repo.MoveStage(bandungCtx, op, bandungHead, medanLead.ID, "CONTACT", "Lintas cabang", "cross-"+uuid.NewString()); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("kepala Bandung memindahkan lead Medan: %v", err)
	}
	headOfficeRows, total, err := repo.ListLeads(ctx, op, domain.CRMLeadFilter{})
	if err != nil || len(headOfficeRows) != 2 || total != 2 {
		t.Fatalf("kantor pusat kehilangan lead cabang: %#v total=%d (%v)", headOfficeRows, total, err)
	}

	moveKey := "crm-stage-" + uuid.NewString()
	moved, applied, err := repo.MoveStage(bandungCtx, op, bandungHead, lead.ID, "CONTACT", "Sudah dihubungi", moveKey)
	if err != nil || !applied || moved.Stage != "CONTACT" {
		t.Fatalf("pindah tahap: applied=%v lead=%#v err=%v", applied, moved, err)
	}
	if _, applied, err := repo.MoveStage(bandungCtx, op, bandungHead, lead.ID, "CONTACT", "Sudah dihubungi", moveKey); err != nil || applied {
		t.Fatalf("replay tahap membuat event baru: applied=%v err=%v", applied, err)
	}
	if _, _, err := repo.MoveStage(bandungCtx, op, bandungHead, lead.ID, "HOT", "Payload berbeda", moveKey); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("idempotency tahap menerima payload berbeda: %v", err)
	}

	activityKey := "crm-contact-" + uuid.NewString()
	activity, created, err := repo.AddActivity(bandungCtx, op, bandungHead, lead.ID, "CONTACT", "Telepon tersambung", "Kirim proposal", activityKey, time.Now(), nil)
	if err != nil || !created || activity.Kind != "CONTACT" {
		t.Fatalf("catat kontak: created=%v activity=%#v err=%v", created, activity, err)
	}
	detail, err := repo.GetLeadDetail(bandungCtx, op, lead.ID)
	if err != nil || len(detail.Activities) != 3 || detail.Lead.LastContactAt == nil {
		t.Fatalf("timeline/proyeksi kontak salah: %#v (%v)", detail, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE crm_leads SET stage='HOT' WHERE id=$1`, lead.ID); !crmConstraintIs(err, "crm_update_requires_activity") {
		t.Fatalf("direct UPDATE tanpa event harus ditolak: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM crm_lead_activities WHERE id=$1`, activity.ID); !crmConstraintIs(err, "crm_activities_append_only") {
		t.Fatalf("timeline dapat dihapus: %v", err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO crm_leads
		(operator_id,full_name,phone,source,pax,created_by_user_id,idempotency_key,request_fingerprint)
		VALUES ($1,'Lead Starter','081','WEBSITE',1,'tester',$2,$3)`, starter, uuid.NewString(), crmFingerprint("starter")); !crmConstraintIs(err, "operator_crm_feature") {
		t.Fatalf("CRM STARTER harus ditolak database: %v", err)
	}
}

func crmConstraintIs(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == constraint
}
