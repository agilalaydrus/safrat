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

// A pilgrim's add-on price is a snapshot taken at assignment time. This
// proves a later catalog price change does not reach back and rewrite what
// an already-assigned pilgrim owes, and that the group filter on
// ListPilgrimAddons narrows to one group's roster without hiding assignments
// that belong to pilgrims outside it from the unfiltered call.
func TestAddonPriceIsSnapshottedAndGroupFilterScopesIntegration(t *testing.T) {
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

	operatorID, orgID := uuid.NewString(), "addon-"+uuid.NewString()
	seasonID := uuid.NewString()
	groupID := uuid.NewString()
	pilgrimInGroupID, pilgrimOutsideGroupID := uuid.NewString(), uuid.NewString()

	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Addon Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "addon-"+operatorID[:8])
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Uji','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`,
		seasonID, operatorID)
	exec(`INSERT INTO groups (id, operator_id, season_id, name) VALUES ($1,$2,$3,'Grup A')`, groupID, operatorID, seasonID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, group_id, full_name, passport_number, nationality, date_of_birth, gender) VALUES ($1,$2,$3,$4,'Jamaah Dalam Grup','A1111111','ID','1990-01-01'::timestamptz,'MALE')`,
		pilgrimInGroupID, seasonID, operatorID, groupID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender) VALUES ($1,$2,$3,'Jamaah Luar Grup','A2222222','ID','1990-01-01'::timestamptz,'MALE')`,
		pilgrimOutsideGroupID, seasonID, operatorID)

	t.Cleanup(func() {
		exec(`DELETE FROM pilgrim_addons WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM addon_items WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM pilgrims WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM groups WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM seasons WHERE id = $1`, seasonID)
		exec(`DELETE FROM operators WHERE id = $1`, operatorID)
	})

	queries := db.New(pool)
	addonService := NewAddonService(repository.NewOperatorRepository(queries), repository.NewAddonRepository(queries))

	item, err := addonService.CreateAddonItem(ctx, orgID, &hajjv1.CreateAddonItemRequest{SeasonId: seasonID, Name: "Kursi Eksekutif", UnitPriceIdr: 500000})
	if err != nil {
		t.Fatalf("CreateAddonItem: %v", err)
	}

	assigned, err := addonService.AssignPilgrimAddon(ctx, orgID, &hajjv1.AssignPilgrimAddonRequest{
		PilgrimId: pilgrimInGroupID, AddonItemId: item.Id, Quantity: 1, UnitPriceIdr: item.UnitPriceIdr,
	})
	if err != nil {
		t.Fatalf("AssignPilgrimAddon (grup): %v", err)
	}
	if assigned.TotalIdr != 500000 {
		t.Fatalf("total awal = %d, mau 500000", assigned.TotalIdr)
	}

	if _, err := addonService.AssignPilgrimAddon(ctx, orgID, &hajjv1.AssignPilgrimAddonRequest{
		PilgrimId: pilgrimOutsideGroupID, AddonItemId: item.Id, Quantity: 2, UnitPriceIdr: item.UnitPriceIdr,
	}); err != nil {
		t.Fatalf("AssignPilgrimAddon (luar grup): %v", err)
	}

	// Catalog price changes after both pilgrims already hold the add-on.
	if _, err := addonService.UpdateAddonItem(ctx, orgID, &hajjv1.UpdateAddonItemRequest{
		ItemId: item.Id, Name: item.Name, UnitPriceIdr: 900000, IsActive: true,
	}); err != nil {
		t.Fatalf("UpdateAddonItem: %v", err)
	}

	all, err := addonService.ListPilgrimAddons(ctx, orgID, &hajjv1.ListPilgrimAddonsRequest{SeasonId: seasonID})
	if err != nil {
		t.Fatalf("ListPilgrimAddons (semua): %v", err)
	}
	if len(all.Addons) != 2 {
		t.Fatalf("jumlah tanpa saringan grup = %d, mau 2", len(all.Addons))
	}
	for _, a := range all.Addons {
		if a.PilgrimId == pilgrimInGroupID && a.UnitPriceIdr != 500000 {
			t.Fatalf("harga jamaah dalam grup berubah jadi %d setelah katalog diubah — semestinya tetap snapshot 500000", a.UnitPriceIdr)
		}
	}

	scoped, err := addonService.ListPilgrimAddons(ctx, orgID, &hajjv1.ListPilgrimAddonsRequest{SeasonId: seasonID, GroupId: groupID})
	if err != nil {
		t.Fatalf("ListPilgrimAddons (grup): %v", err)
	}
	if len(scoped.Addons) != 1 || scoped.Addons[0].PilgrimId != pilgrimInGroupID {
		t.Fatalf("saringan grup tidak tepat: %+v", scoped.Addons)
	}

	// Marking paid should not disturb the price snapshot or the join fields.
	updated, err := addonService.SetPilgrimAddonPaid(ctx, orgID, &hajjv1.SetPilgrimAddonPaidRequest{PilgrimAddonId: assigned.Id, Paid: true})
	if err != nil {
		t.Fatalf("SetPilgrimAddonPaid: %v", err)
	}
	if !updated.Paid || updated.TotalIdr != 500000 || updated.GroupName != "Grup A" {
		t.Fatalf("hasil SetPilgrimAddonPaid tidak sesuai: %+v", updated)
	}
}
