package service

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/events"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Rangkaian must start and end with Transport — a trip cannot begin or end
// anywhere but on a vehicle. This test proves SetItinerary enforces that,
// and that a valid Transport-Hotel-Transport sequence round-trips correctly.
func TestSetKloterItineraryEnforcesTransportBookendsIntegration(t *testing.T) {
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

	operatorID, orgID := uuid.NewString(), "kloter-itin-"+uuid.NewString()
	seasonID, kloterID := uuid.NewString(), uuid.NewString()
	movementInID, movementOutID := uuid.NewString(), uuid.NewString()
	hotelID := uuid.NewString()

	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Rangkaian Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "rgk-"+operatorID[:8])
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Uji','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`,
		seasonID, operatorID)
	exec(`INSERT INTO kloters (id, operator_id, season_id, code, embarkation) VALUES ($1,$2,$3,'SOC-01','CGK')`,
		kloterID, operatorID, seasonID)
	exec(`INSERT INTO movements (id, season_id, operator_id, name, origin, destination, scheduled_at, mode, kloter_id) VALUES ($1,$2,$3,'Bus ke Bandara','Hotel','Bandara',NOW(),'BUS',$4)`,
		movementInID, seasonID, operatorID, kloterID)
	exec(`INSERT INTO movements (id, season_id, operator_id, name, origin, destination, scheduled_at, mode, kloter_id) VALUES ($1,$2,$3,'Bus ke Hotel','Bandara','Hotel',NOW(),'BUS',$4)`,
		movementOutID, seasonID, operatorID, kloterID)
	exec(`INSERT INTO hotels (id, operator_id, season_id, name, city) VALUES ($1,$2,$3,'Hotel Uji','Makkah')`,
		hotelID, operatorID, seasonID)
	t.Cleanup(func() {
		exec(`DELETE FROM kloter_itinerary_segments WHERE kloter_id = $1`, kloterID)
		exec(`DELETE FROM hotels WHERE id = $1`, hotelID)
		exec(`DELETE FROM movements WHERE kloter_id = $1`, kloterID)
		exec(`DELETE FROM kloters WHERE id = $1`, kloterID)
		exec(`DELETE FROM seasons WHERE id = $1`, seasonID)
		exec(`DELETE FROM operators WHERE id = $1`, operatorID)
	})

	queries := db.New(pool)
	kloterService := NewKloterService(
		repository.NewOperatorRepository(queries),
		repository.NewKloterRepository(queries, pool),
		repository.NewAuditRepository(queries),
		repository.NewOutboxRepository(queries),
		pool, events.NewBus(),
	)

	// A sequence that starts on Hotel is refused outright.
	_, err = kloterService.SetItinerary(ctx, orgID, &hajjv1.SetKloterItineraryRequest{
		KloterId: kloterID,
		Segments: []*hajjv1.ItinerarySegmentInput{
			{SegmentType: "HOTEL", HotelId: hotelID},
			{SegmentType: "TRANSPORT", MovementId: movementOutID},
		},
	})
	if err == nil {
		t.Fatalf("mau ditolak: Rangkaian yang dimulai dari Hotel semestinya tidak valid")
	}

	// Transport - Hotel - Transport is the real shape and must save clean.
	response, err := kloterService.SetItinerary(ctx, orgID, &hajjv1.SetKloterItineraryRequest{
		KloterId: kloterID,
		Segments: []*hajjv1.ItinerarySegmentInput{
			{SegmentType: "TRANSPORT", MovementId: movementInID},
			{SegmentType: "HOTEL", HotelId: hotelID, Notes: "check-in 3 malam"},
			{SegmentType: "TRANSPORT", MovementId: movementOutID},
		},
	})
	if err != nil {
		t.Fatalf("SetItinerary valid: %v", err)
	}
	if len(response.Segments) != 3 {
		t.Fatalf("segmen = %d, mau 3", len(response.Segments))
	}
	if response.Segments[0].SegmentType != "TRANSPORT" || response.Segments[2].SegmentType != "TRANSPORT" {
		t.Fatalf("urutan salah: %s ... %s", response.Segments[0].SegmentType, response.Segments[2].SegmentType)
	}
	if response.Segments[1].HotelName != "Hotel Uji" || response.Segments[1].HotelCity != "Makkah" {
		t.Fatalf("info hotel tidak ikut ter-join: %+v", response.Segments[1])
	}
	if response.Segments[0].MovementMode != "BUS" {
		t.Fatalf("info movement tidak ikut ter-join: %+v", response.Segments[0])
	}

	listed, err := kloterService.ListItinerary(ctx, orgID, &hajjv1.ListKloterItineraryRequest{KloterId: kloterID})
	if err != nil {
		t.Fatalf("ListItinerary: %v", err)
	}
	if len(listed.Segments) != 3 {
		t.Fatalf("listed segmen = %d, mau 3", len(listed.Segments))
	}

	// A movement that belongs to a different kloter must be refused, not
	// silently accepted — otherwise one operator's Rangkaian could point at
	// transport nobody in this kloter is actually taking.
	otherKloterID := uuid.NewString()
	otherMovementID := uuid.NewString()
	exec(`INSERT INTO kloters (id, operator_id, season_id, code, embarkation) VALUES ($1,$2,$3,'SOC-02','CGK')`,
		otherKloterID, operatorID, seasonID)
	exec(`INSERT INTO movements (id, season_id, operator_id, name, origin, destination, scheduled_at, mode, kloter_id) VALUES ($1,$2,$3,'Bus Lain','A','B',NOW(),'BUS',$4)`,
		otherMovementID, seasonID, operatorID, otherKloterID)
	t.Cleanup(func() {
		exec(`DELETE FROM movements WHERE id = $1`, otherMovementID)
		exec(`DELETE FROM kloters WHERE id = $1`, otherKloterID)
	})
	_, err = kloterService.SetItinerary(ctx, orgID, &hajjv1.SetKloterItineraryRequest{
		KloterId: kloterID,
		Segments: []*hajjv1.ItinerarySegmentInput{
			{SegmentType: "TRANSPORT", MovementId: otherMovementID},
		},
	})
	if err == nil {
		t.Fatalf("mau ditolak: movement milik kloter lain semestinya tidak boleh dipakai")
	}
	if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "InvalidArgument") {
		t.Logf("info: pesan error = %v", err)
	}
}
