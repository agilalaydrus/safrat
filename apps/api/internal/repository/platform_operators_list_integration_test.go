package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestListOperatorsComputesCancelledAtSeasonCountAndReturnedSinceSignupIntegration
// covers three fields added across D4/D5 (TUGAS-PANEL-SAAS.md) that never
// had a dedicated test: cancelled_at surviving the row-to-struct scan,
// season_count as a real join (not left as zero), and
// has_returned_since_signup — the one genuinely tricky field, since there is
// no login-history table to count against (every sign-in deletes every
// other session for that user; see the SQL comment in ListOperators).
func TestListOperatorsComputesCancelledAtSeasonCountAndReturnedSinceSignupIntegration(t *testing.T) {
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

	// Operator A: cancelled subscription, two seasons, and a member whose
	// live session is old — never came back since signup.
	opA, orgA := uuid.NewString(), "list-ops-a-"+uuid.NewString()
	userA := "list-ops-user-a-" + uuid.NewString()
	// Operator B: no subscription row at all (a brand new signup that has
	// not reached OperatorService.Create's subscription step), one season,
	// and a member whose live session is clearly newer than the operator —
	// came back and signed in again.
	opB, orgB := uuid.NewString(), "list-ops-b-"+uuid.NewString()
	userB := "list-ops-user-b-" + uuid.NewString()

	exec(`INSERT INTO organization (id,name,slug,"createdAt") VALUES ($1,'List Ops A',$2,NOW()),($3,'List Ops B',$4,NOW())`,
		orgA, "list-ops-a-"+opA[:8], orgB, "list-ops-b-"+opB[:8])
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'List Ops Travel A','ID',$3,$4),($5,$6,'List Ops Travel B','ID',$7,$8)`,
		opA, orgA, opA[:8]+"@example.test", "list-ops-a-"+opA[:8],
		opB, orgB, opB[:8]+"@example.test", "list-ops-b-"+opB[:8])
	exec(`INSERT INTO subscriptions (operator_id,plan,status,access_until,cancelled_at)
		VALUES ($1,'STARTER','ACTIVE',NOW()+INTERVAL '10 days',NOW())`, opA)
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES
		($1,$2,'Musim 1','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10),
		($3,$2,'Musim 2','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`,
		uuid.NewString(), opA, uuid.NewString())
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES
		($1,$2,'Musim 1','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, uuid.NewString(), opB)

	exec(`INSERT INTO "user" (id,name,email,"emailVerified") VALUES ($1,'Staf A',$2,true),($3,'Staf B',$4,true)`,
		userA, userA+"@example.test", userB, userB+"@example.test")
	exec(`INSERT INTO "member" (id,"organizationId","userId",role,"createdAt") VALUES ($1,$2,$3,'owner',NOW()),($4,$5,$6,'owner',NOW())`,
		uuid.NewString(), orgA, userA, uuid.NewString(), orgB, userB)

	// A's only session is from the same moment as signup — never returned.
	exec(`SELECT created_at FROM operators WHERE id = $1`, opA) // sanity: row exists
	var opACreatedAt, opBCreatedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT created_at FROM operators WHERE id=$1`, opA).Scan(&opACreatedAt); err != nil {
		t.Fatalf("read opA created_at: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT created_at FROM operators WHERE id=$1`, opB).Scan(&opBCreatedAt); err != nil {
		t.Fatalf("read opB created_at: %v", err)
	}
	exec(`INSERT INTO "session" (id,"expiresAt",token,"createdAt","updatedAt","userId")
		VALUES ($1,NOW()+INTERVAL '1 day',$2,$3,NOW(),$4)`,
		uuid.NewString(), "list-ops-a-token-"+uuid.NewString(), opACreatedAt.Add(1*time.Minute), userA)
	// B's live session is 3 hours after signup — came back.
	exec(`INSERT INTO "session" (id,"expiresAt",token,"createdAt","updatedAt","userId")
		VALUES ($1,NOW()+INTERVAL '1 day',$2,$3,NOW(),$4)`,
		uuid.NewString(), "list-ops-b-token-"+uuid.NewString(), opBCreatedAt.Add(3*time.Hour), userB)

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM "session" WHERE "userId" IN ($1,$2)`, userA, userB)
		_, _ = pool.Exec(bg, `DELETE FROM "member" WHERE "organizationId" IN ($1,$2)`, orgA, orgB)
		_, _ = pool.Exec(bg, `DELETE FROM "user" WHERE id IN ($1,$2)`, userA, userB)
		_, _ = pool.Exec(bg, `DELETE FROM seasons WHERE operator_id IN ($1,$2)`, opA, opB)
		_, _ = pool.Exec(bg, `DELETE FROM subscriptions WHERE operator_id IN ($1,$2)`, opA, opB)
		_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id IN ($1,$2)`, opA, opB)
		_, _ = pool.Exec(bg, `DELETE FROM organization WHERE id IN ($1,$2)`, orgA, orgB)
	})

	repo := NewPlatformRepository(pool)
	operators, err := repo.ListOperators(ctx, 200)
	if err != nil {
		t.Fatalf("ListOperators: %v", err)
	}
	var foundA, foundB *PlatformOperator
	for _, o := range operators {
		if o.ID == opA {
			foundA = o
		}
		if o.ID == opB {
			foundB = o
		}
	}
	if foundA == nil || foundB == nil {
		t.Fatalf("operator uji tidak ditemukan di hasil ListOperators (A=%v B=%v)", foundA != nil, foundB != nil)
	}

	if foundA.CancelledAt == nil {
		t.Fatal("operator A: cancelled_at seharusnya terisi, dapat nil")
	}
	if foundA.SeasonCount != 2 {
		t.Fatalf("operator A: season_count = %d, mau 2", foundA.SeasonCount)
	}
	if foundA.HasReturnedSinceSignup {
		t.Fatal("operator A: has_returned_since_signup seharusnya false (sesi satu-satunya sezaman dengan pendaftaran)")
	}

	if foundB.CancelledAt != nil {
		t.Fatal("operator B: cancelled_at seharusnya nil (tidak pernah dibatalkan)")
	}
	if foundB.SeasonCount != 1 {
		t.Fatalf("operator B: season_count = %d, mau 1", foundB.SeasonCount)
	}
	if !foundB.HasReturnedSinceSignup {
		t.Fatal("operator B: has_returned_since_signup seharusnya true (sesi hidup 3 jam setelah pendaftaran)")
	}
}
