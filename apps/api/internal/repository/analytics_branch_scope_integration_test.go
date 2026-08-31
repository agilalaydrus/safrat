package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Dashboard figures are personal-data aggregates too. This verifies that a
// branch head receives figures only for its own pilgrims, while head office
// retains the operator-wide view.
func TestDashboardAnalyticsEnforcesBranchScopeIntegration(t *testing.T) {
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

	operatorID, seasonID := uuid.NewString(), uuid.NewString()
	bandungID, medanID := uuid.NewString(), uuid.NewString()
	bandungHead := "analytics-head-" + uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan)
	      VALUES ($1,$2,'Analytics Scope Test','ID',$3,$4,'GROWTH')`,
		operatorID, "analytics-scope-"+uuid.NewString(), operatorID[:8]+"@example.test", "analytics-"+operatorID[:8])
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, operatorID) })
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity)
	      VALUES ($1,$2,'Musim Analytics','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',50)`, seasonID, operatorID)
	exec(`INSERT INTO branches (id, operator_id, name, city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandungID, medanID, operatorID)
	exec(`INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id) VALUES ($1,$2,$3)`, bandungHead, bandungID, operatorID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, branch_id, full_name, passport_number, nationality, date_of_birth, gender, payment_status)
	      VALUES ($1,$3,$4,$5,'Jamaah Bandung','ANL-BDG','ID','1990-01-01','MALE','PAID'),
	             ($2,$3,$4,$6,'Jamaah Medan','ANL-MDN','ID','1991-01-01','FEMALE','DP')`,
		uuid.NewString(), uuid.NewString(), seasonID, operatorID, bandungID, medanID)

	seasonRepo := NewSeasonRepository(db.New(pool))
	analyticsRepo := NewAnalyticsRepository(db.New(pool))
	bandungCtx := ContextWithStaffActor(ctx, bandungHead)

	analytics, err := seasonRepo.GetAnalytics(bandungCtx, operatorID, seasonID)
	if err != nil || analytics.TotalPilgrims != 1 || analytics.PaidCount != 1 || analytics.DPCount != 0 {
		t.Fatalf("ringkasan Bandung bocor lintas cabang: %#v (%v)", analytics, err)
	}
	timeline, err := analyticsRepo.ListPaymentTimeline(bandungCtx, operatorID, seasonID)
	if err != nil || len(timeline) != 1 || timeline[0].PaidCount != 1 || timeline[0].DPCount != 0 {
		t.Fatalf("timeline Bandung bocor lintas cabang: %#v (%v)", timeline, err)
	}

	headOffice, err := seasonRepo.GetAnalytics(ContextWithStaffActor(ctx, "analytics-hq-"+uuid.NewString()), operatorID, seasonID)
	if err != nil || headOffice.TotalPilgrims != 2 || headOffice.PaidCount != 1 || headOffice.DPCount != 1 {
		t.Fatalf("kantor pusat harus melihat dua cabang: %#v (%v)", headOffice, err)
	}
}
