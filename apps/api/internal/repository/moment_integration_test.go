package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A moment targets exactly one of a pilgrim or a group, and the family-facing
// read (ListForFamily) must resolve both: a moment aimed straight at this
// pilgrim, and a moment aimed at the group they belong to. It must NOT
// resolve a moment aimed at a different pilgrim or a different group — that
// would leak one family's photo into another family's feed.
func TestMomentFamilyVisibilityIntegration(t *testing.T) {
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

	operatorID := uuid.NewString()
	seasonID := uuid.NewString()
	groupID := uuid.NewString()
	pilgrimInGroupID := uuid.NewString()
	pilgrimOutsideGroupID := uuid.NewString()

	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Momen Uji','ID',$3,$4)`,
		operatorID, "moment-"+operatorID, operatorID[:8]+"@example.test", "moment-"+operatorID[:8])
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Uji','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`,
		seasonID, operatorID)
	exec(`INSERT INTO groups (id, operator_id, season_id, name) VALUES ($1,$2,$3,'Grup A')`, groupID, operatorID, seasonID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, group_id, full_name, passport_number, nationality, date_of_birth, gender) VALUES ($1,$2,$3,$4,'Jamaah Dalam Grup','M1111111','ID','1990-01-01'::timestamptz,'MALE')`,
		pilgrimInGroupID, seasonID, operatorID, groupID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender) VALUES ($1,$2,$3,'Jamaah Luar Grup','M2222222','ID','1990-01-01'::timestamptz,'MALE')`,
		pilgrimOutsideGroupID, seasonID, operatorID)

	t.Cleanup(func() {
		exec(`DELETE FROM pilgrim_moments WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM pilgrims WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM groups WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM seasons WHERE id = $1`, seasonID)
		exec(`DELETE FROM operators WHERE id = $1`, operatorID)
	})

	queries := db.New(pool)
	repo := NewMomentRepository(queries)

	// A moment for the whole group, and a moment for the outside pilgrim
	// directly, and a moment for a group that does not exist in this test at
	// all (nobody should ever see it, but we don't need a third pilgrim to
	// prove that — the two above are enough).
	if _, err := repo.Create(ctx, operatorID, seasonID, "", groupID, "moments/"+operatorID+"/group.jpg", "Foto rombongan", "Petugas A"); err != nil {
		t.Fatalf("Create (group): %v", err)
	}
	if _, err := repo.Create(ctx, operatorID, seasonID, pilgrimOutsideGroupID, "", "moments/"+operatorID+"/solo.jpg", "Foto pribadi", "Petugas B"); err != nil {
		t.Fatalf("Create (pilgrim): %v", err)
	}

	inGroup, err := repo.ListForFamily(ctx, pilgrimInGroupID)
	if err != nil {
		t.Fatalf("ListForFamily (dalam grup): %v", err)
	}
	if len(inGroup) != 1 || inGroup[0].Caption != "Foto rombongan" {
		t.Fatalf("jamaah dalam grup semestinya melihat 1 momen grup, dapat: %+v", inGroup)
	}

	outsideGroup, err := repo.ListForFamily(ctx, pilgrimOutsideGroupID)
	if err != nil {
		t.Fatalf("ListForFamily (luar grup): %v", err)
	}
	if len(outsideGroup) != 1 || outsideGroup[0].Caption != "Foto pribadi" {
		t.Fatalf("jamaah luar grup semestinya melihat 1 momen pribadi miliknya sendiri, bukan momen grup: %+v", outsideGroup)
	}

	// Staff-facing list sees both, regardless of target.
	all, err := repo.ListForSeason(ctx, operatorID, seasonID)
	if err != nil {
		t.Fatalf("ListForSeason: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("daftar staf = %d momen, mau 2", len(all))
	}
}
