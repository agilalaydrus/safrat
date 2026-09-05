package service

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestInquiryServiceIsolationEntitlementAndAttributionIntegration proves
// three things K2.5 (TUGAS-CORONG.md) depends on:
//  1. an operator can only ever list/convert its own inquiries;
//  2. converting still respects the same CRM plan entitlement CreateLead
//     already enforces — a Starter-plan operator receives inquiries but
//     can't file one into a pipeline it doesn't have;
//  3. a converted lead's Source/Campaign come from the visitor's own
//     utm_campaign/utm_source, not from anything staff typed.
func TestInquiryServiceIsolationEntitlementAndAttributionIntegration(t *testing.T) {
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

	growthOp, starterOp := uuid.NewString(), uuid.NewString()
	growthOrg, starterOrg := "inquiry-growth-"+uuid.NewString(), "inquiry-starter-"+uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan) VALUES
		($1,$2,'Travel Growth','ID',$3,$4,'GROWTH'),
		($5,$6,'Travel Starter','ID',$7,$8,'STARTER')`,
		growthOp, growthOrg, growthOp[:8]+"@example.test", "inq-growth-"+growthOp[:8],
		starterOp, starterOrg, starterOp[:8]+"@example.test", "inq-starter-"+starterOp[:8])
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id IN ($1,$2)`, growthOp, starterOp)
	})

	queries := db.New(pool)
	operators := repository.NewOperatorRepository(queries)
	inquiries := repository.NewInquiryRepository(queries)
	crm := repository.NewCRMRepository(pool)
	audit := repository.NewAuditRepository(queries)
	svc := NewInquiryService(operators, inquiries, crm, audit, NewEntitlementChecker(repository.NewEntitlementRepository(queries)))

	growthCtx := middleware.ContextWithIdentity(ctx, "staff-"+uuid.NewString(), growthOrg)
	starterCtx := middleware.ContextWithIdentity(ctx, "staff-"+uuid.NewString(), starterOrg)

	// A visitor lands on Travel Growth's storefront from an Instagram ad.
	submitResp, err := svc.Submit(ctx, &hajjv1.SubmitInquiryRequest{
		OperatorId: growthOp, FullName: "Calon Jamaah Growth", Phone: "081200000001",
		Message: "Tanya paket Umrah Desember", UtmSource: "instagram", UtmCampaign: "umrah-des-2026",
	})
	if err != nil || submitResp.Message == "" {
		t.Fatalf("submit inquiry growth: %v", err)
	}

	// A second visitor lands on Travel Starter's storefront, no UTM at all.
	if _, err := svc.Submit(ctx, &hajjv1.SubmitInquiryRequest{
		OperatorId: starterOp, FullName: "Calon Jamaah Starter", Phone: "081200000002",
	}); err != nil {
		t.Fatalf("submit inquiry starter: %v", err)
	}

	// Isolation: Growth's inbox must show only its own inquiry.
	growthList, err := svc.List(growthCtx, growthOrg, &hajjv1.ListInquiriesRequest{})
	if err != nil {
		t.Fatalf("list growth: %v", err)
	}
	if len(growthList.Inquiries) != 1 || growthList.Inquiries[0].FullName != "Calon Jamaah Growth" {
		t.Fatalf("kebocoran isolasi: growth melihat %d baris, %#v", len(growthList.Inquiries), growthList.Inquiries)
	}
	starterList, err := svc.List(starterCtx, starterOrg, &hajjv1.ListInquiriesRequest{})
	if err != nil {
		t.Fatalf("list starter: %v", err)
	}
	if len(starterList.Inquiries) != 1 || starterList.Inquiries[0].FullName != "Calon Jamaah Starter" {
		t.Fatalf("kebocoran isolasi: starter melihat %d baris, %#v", len(starterList.Inquiries), starterList.Inquiries)
	}

	// Entitlement: Starter has no CRM feature, so converting must fail even
	// though the inquiry itself belongs to Starter.
	_, err = svc.ConvertToLead(starterCtx, starterOrg, &hajjv1.ConvertInquiryToLeadRequest{
		InquiryId: starterList.Inquiries[0].Id, IdempotencyKey: "convert-" + uuid.NewString(),
	})
	if err == nil {
		t.Fatalf("konversi Starter seharusnya ditolak entitlement CRM, malah berhasil")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("konversi Starter: ingin failed_precondition (fitur crm), dapat %v (%s)", err, connect.CodeOf(err))
	}

	// Attribution: Growth converts its inquiry, and the resulting lead's
	// Source/Campaign must reflect the visitor's own link, not a manual guess.
	lead, err := svc.ConvertToLead(growthCtx, growthOrg, &hajjv1.ConvertInquiryToLeadRequest{
		InquiryId: growthList.Inquiries[0].Id, IdempotencyKey: "convert-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("konversi growth: %v", err)
	}
	if lead.Source != hajjv1.CRMLeadSource_CRM_LEAD_SOURCE_WEBSITE {
		t.Fatalf("source lead ingin WEBSITE, dapat %v", lead.Source)
	}
	if lead.Campaign != "umrah-des-2026" {
		t.Fatalf("campaign lead ingin 'umrah-des-2026' (dari utm_campaign pengunjung), dapat %q", lead.Campaign)
	}

	// Converting the same inquiry twice must not be possible — it's already
	// spoken for.
	if _, err := svc.ConvertToLead(growthCtx, growthOrg, &hajjv1.ConvertInquiryToLeadRequest{
		InquiryId: growthList.Inquiries[0].Id, IdempotencyKey: "convert-" + uuid.NewString(),
	}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("konversi ulang pesan yang sudah dikonversi seharusnya ditolak, dapat %v (%s)", err, connect.CodeOf(err))
	}
}
