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

func TestPlatformPlanControlsGrandfatherIsolationAuditAndIdempotencyIntegration(t *testing.T) {
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

	actor := "platform-plan-test-" + uuid.NewString()
	opA, opB := uuid.NewString(), uuid.NewString()
	seasonA, seasonB := uuid.NewString(), uuid.NewString()
	insert := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	insert(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES
		($1,$2,'Plan Test A','ID',$3,$4,'PRO'),($5,$6,'Plan Test B','ID',$7,$8,'PRO')`,
		opA, "plan-a-"+uuid.NewString(), opA[:8]+"@example.test", "plan-a-"+opA[:8],
		opB, "plan-b-"+uuid.NewString(), opB[:8]+"@example.test", "plan-b-"+opB[:8])
	insert(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES
		($1,$2,'Musim A','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20),
		($3,$4,'Musim B','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, seasonA, opA, seasonB, opB)
	addPilgrim := func(operatorID, seasonID, name string) error {
		_, err := pool.Exec(ctx, `INSERT INTO pilgrims
			(id,season_id,operator_id,full_name,passport_number,nationality,date_of_birth,gender)
			VALUES ($1,$2,$3,$4,$5,'ID','1990-01-01','MALE')`,
			uuid.NewString(), seasonID, operatorID, name, "PLAN-"+uuid.NewString())
		return err
	}
	if err := addPilgrim(opA, seasonA, "A Satu"); err != nil {
		t.Fatal(err)
	}
	if err := addPilgrim(opA, seasonA, "A Dua"); err != nil {
		t.Fatal(err)
	}
	if err := addPilgrim(opB, seasonB, "B Satu"); err != nil {
		t.Fatal(err)
	}

	var originalFlags []byte
	if err := pool.QueryRow(ctx, `SELECT feature_flags FROM plan_limits WHERE plan='PRO'`).Scan(&originalFlags); err != nil {
		t.Fatalf("read PRO: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `UPDATE plan_limits SET max_pilgrims=NULL,max_branches=NULL,feature_flags=$1,updated_at=NOW() WHERE plan='PRO'`, originalFlags)
		_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id IN ($1,$2)`, opA, opB)
		_, _ = pool.Exec(bg, `DELETE FROM audit_logs WHERE user_id=$1`, actor)
		_, _ = pool.Exec(bg, `DELETE FROM privileged_actions WHERE requested_by=$1`, actor)
	})

	repo := NewPlatformRepository(pool)
	one := int32(1)
	change := PlanLimitChange{
		Plan: "PRO", MaxPilgrims: &one, FeatureFlags: map[string]bool{"branches": true, "installments": true, "crm": true},
		Reason: "uji penurunan kuota", ActorUserID: actor, IdempotencyKey: "plan-" + uuid.NewString(), GrandfatherAffected: true,
	}
	preview, err := repo.PreviewPlanLimitChange(ctx, change)
	if err != nil || len(preview) != 1 || preview[0].OperatorID != opA || preview[0].Name != "Plan Test A" {
		t.Fatalf("preview harus menyebut tenant A: %#v (%v)", preview, err)
	}
	limit, grandfathered, err := repo.SetPlanLimit(ctx, change)
	if err != nil || limit.MaxPilgrims == nil || *limit.MaxPilgrims != 1 || grandfathered != 1 {
		t.Fatalf("set limit: limit=%#v grandfathered=%d err=%v", limit, grandfathered, err)
	}
	// Exact replay returns the stored result and does not create more evidence.
	_, replayCount, err := repo.SetPlanLimit(ctx, change)
	if err != nil || replayCount != 1 {
		t.Fatalf("replay set limit: count=%d err=%v", replayCount, err)
	}
	var actionCount, auditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM privileged_actions WHERE requested_by=$1`, actor).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE user_id=$1 AND action='plan_limit_changed'`, actor).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if actionCount != 1 || auditCount != 1 {
		t.Fatalf("replay menggandakan bukti: privileged=%d audit=%d", actionCount, auditCount)
	}

	entitlements := NewEntitlementRepository(db.New(pool))
	aEntitlement, err := entitlements.Get(ctx, opA)
	if err != nil || aEntitlement.MaxPilgrims == nil || *aEntitlement.MaxPilgrims != 2 {
		t.Fatalf("tenant A tidak di-grandfather: %#v (%v)", aEntitlement, err)
	}
	if err := addPilgrim(opB, seasonB, "B Dua"); !errors.Is(databaseError(err), apperror.ErrFailedPrecondition) {
		t.Fatalf("tenant B harus terkena batas global: %v", err)
	}

	two := int32(2)
	overrideChange := PlanOverrideChange{
		OperatorID: opB, MaxPilgrims: &two, FeatureFlagOverrides: map[string]bool{}, Note: "tambahan untuk uji",
		ActorUserID: actor, IdempotencyKey: "override-" + uuid.NewString(),
	}
	override, err := repo.SetPlanOverride(ctx, overrideChange)
	if err != nil || override.OperatorID != opB || override.MaxPilgrims == nil || *override.MaxPilgrims != 2 {
		t.Fatalf("set override B: %#v (%v)", override, err)
	}
	if _, err := repo.SetPlanOverride(ctx, overrideChange); err != nil {
		t.Fatalf("replay override: %v", err)
	}
	conflicting := overrideChange
	conflicting.Note = "payload berbeda"
	if _, err := repo.SetPlanOverride(ctx, conflicting); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("kunci sama menerima payload berbeda: %v", err)
	}
	if err := addPilgrim(opB, seasonB, "B Dua"); err != nil {
		t.Fatalf("override B tidak berlaku: %v", err)
	}
	aEntitlement, _ = entitlements.Get(ctx, opA)
	bEntitlement, _ := entitlements.Get(ctx, opB)
	if aEntitlement.MaxPilgrims == nil || *aEntitlement.MaxPilgrims != 2 || bEntitlement.MaxPilgrims == nil || *bEntitlement.MaxPilgrims != 2 {
		t.Fatalf("override bocor atau hilang: A=%#v B=%#v", aEntitlement, bEntitlement)
	}

	// Expiry is enforced by the database-backed worker path and leaves evidence.
	insert(`UPDATE plan_overrides SET expires_at=NOW()-INTERVAL '1 second' WHERE operator_id=$1`, opB)
	expired, err := repo.ExpirePlanOverrides(ctx)
	if err != nil || expired != 1 {
		t.Fatalf("expire override: count=%d err=%v", expired, err)
	}
	bEntitlement, err = entitlements.Get(ctx, opB)
	if err != nil || bEntitlement.MaxPilgrims == nil || *bEntitlement.MaxPilgrims != 1 {
		t.Fatalf("override kedaluwarsa masih aktif: %#v (%v)", bEntitlement, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE operator_id=$1 AND action='plan_override_expired'`, opB).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("expiry tidak diaudit: count=%d err=%v", auditCount, err)
	}

	if err := repo.DeletePlanOverride(ctx, opA, actor, "selesai uji", "delete-"+uuid.NewString()); err != nil {
		t.Fatalf("hapus grandfather override: %v", err)
	}
	if _, err := repo.GetPlanOverride(ctx, opA); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("override A masih ada: %v", err)
	}
}
