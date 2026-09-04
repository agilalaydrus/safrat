package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The combined calendar merges three sources that live in three different
// tables. This proves the merge is complete (all three kinds show up) and
// that the branch filter narrows only internal events — manasik and kloter
// movements are never branch-owned, so a branch view must not hide them.
func TestAgendaMergesThreeSourcesAndScopesBranchIntegration(t *testing.T) {
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

	operatorID, orgID := uuid.NewString(), "agenda-"+uuid.NewString()
	seasonID := uuid.NewString()
	branchID := uuid.NewString()
	kloterID := uuid.NewString()
	departureMovementID, returnMovementID := uuid.NewString(), uuid.NewString()
	hotelID := uuid.NewString()

	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug, plan) VALUES ($1,$2,'Agenda Uji','ID',$3,$4,'PRO')`,
		operatorID, orgID, operatorID[:8]+"@example.test", "agn-"+operatorID[:8])
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Uji','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`,
		seasonID, operatorID)
	exec(`INSERT INTO branches (id, operator_id, name) VALUES ($1,$2,'Cabang Uji')`, branchID, operatorID)
	exec(`INSERT INTO kloters (id, operator_id, season_id, code) VALUES ($1,$2,$3,'K1')`, kloterID, operatorID, seasonID)
	exec(`INSERT INTO movements (id, operator_id, season_id, name, origin, destination, scheduled_at) VALUES ($1,$2,$3,'Keberangkatan','CGK','JED', NOW() + INTERVAL '1 day')`,
		departureMovementID, operatorID, seasonID)
	exec(`INSERT INTO movements (id, operator_id, season_id, name, origin, destination, scheduled_at) VALUES ($1,$2,$3,'Kepulangan','JED','CGK', NOW() + INTERVAL '10 days')`,
		returnMovementID, operatorID, seasonID)
	exec(`INSERT INTO hotels (id, operator_id, season_id, name, city) VALUES ($1,$2,$3,'Hotel Uji','Makkah')`, hotelID, operatorID, seasonID)
	exec(`INSERT INTO kloter_itinerary_segments (operator_id, kloter_id, position, segment_type, movement_id) VALUES ($1,$2,1,'TRANSPORT',$3)`,
		operatorID, kloterID, departureMovementID)
	exec(`INSERT INTO kloter_itinerary_segments (operator_id, kloter_id, position, segment_type, hotel_id) VALUES ($1,$2,2,'HOTEL',$3)`,
		operatorID, kloterID, hotelID)
	exec(`INSERT INTO kloter_itinerary_segments (operator_id, kloter_id, position, segment_type, movement_id) VALUES ($1,$2,3,'TRANSPORT',$3)`,
		operatorID, kloterID, returnMovementID)
	exec(`INSERT INTO manasik_sessions (operator_id, season_id, title, scheduled_at) VALUES ($1,$2,'Manasik Akbar', NOW() + INTERVAL '5 days')`,
		operatorID, seasonID)

	t.Cleanup(func() {
		exec(`DELETE FROM agenda_events WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM manasik_sessions WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM kloter_itinerary_segments WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM kloters WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM hotels WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM movements WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM branches WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM seasons WHERE id = $1`, seasonID)
		exec(`DELETE FROM operators WHERE id = $1`, operatorID)
	})

	queries := db.New(pool)
	agendaService := NewAgendaService(repository.NewOperatorRepository(queries), repository.NewAgendaRepository(queries))

	if _, err := agendaService.CreateAgendaEvent(ctx, orgID, &hajjv1.CreateAgendaEventRequest{
		SeasonId: seasonID, Title: "Rapat Pusat", StartsAt: timestamppb.New(time.Now().Add(48 * time.Hour)),
	}); err != nil {
		t.Fatalf("CreateAgendaEvent (pusat): %v", err)
	}
	if _, err := agendaService.CreateAgendaEvent(ctx, orgID, &hajjv1.CreateAgendaEventRequest{
		SeasonId: seasonID, BranchId: branchID, Title: "Briefing Cabang", StartsAt: timestamppb.New(time.Now().Add(72 * time.Hour)),
	}); err != nil {
		t.Fatalf("CreateAgendaEvent (cabang): %v", err)
	}

	all, err := agendaService.ListAgenda(ctx, orgID, &hajjv1.ListAgendaRequest{SeasonId: seasonID})
	if err != nil {
		t.Fatalf("ListAgenda (semua): %v", err)
	}
	// 2 internal events + 1 manasik + departure + return = 5.
	if len(all.Items) != 5 {
		t.Fatalf("item agenda tanpa saringan cabang = %d, mau 5: %+v", len(all.Items), all.Items)
	}
	kinds := map[string]int{}
	for _, item := range all.Items {
		kinds[item.Kind]++
	}
	if kinds["INTERNAL"] != 2 || kinds["MANASIK"] != 1 || kinds["DEPARTURE"] != 1 || kinds["RETURN"] != 1 {
		t.Fatalf("sebaran kind salah: %+v", kinds)
	}

	scoped, err := agendaService.ListAgenda(ctx, orgID, &hajjv1.ListAgendaRequest{SeasonId: seasonID, BranchId: branchID})
	if err != nil {
		t.Fatalf("ListAgenda (cabang): %v", err)
	}
	// Manasik dan keberangkatan/kepulangan tetap tampil — keduanya bukan
	// milik cabang manapun. Hanya event internal pusat yang tersaring keluar.
	if len(scoped.Items) != 4 {
		t.Fatalf("item agenda dengan saringan cabang = %d, mau 4 (briefing cabang + manasik + keberangkatan + kepulangan): %+v", len(scoped.Items), scoped.Items)
	}
	scopedKinds := map[string]int{}
	for _, item := range scoped.Items {
		scopedKinds[item.Kind]++
		if item.Kind == "INTERNAL" && item.Title != "Briefing Cabang" {
			t.Fatalf("saringan cabang meloloskan event pusat: %s", item.Title)
		}
	}
	if scopedKinds["INTERNAL"] != 1 {
		t.Fatalf("saringan cabang harus menyisakan tepat 1 event internal, dapat %d", scopedKinds["INTERNAL"])
	}
}
