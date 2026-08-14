package service

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// These are integration tests. They only run against an explicitly designated
// disposable database, so `go test ./...` can never alter a developer database.
type transportHarness struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	service  *TransportService
	queries  *db.Queries
	orgID    string
	userID   string
	operator string
	seasonID string
}

func newTransportHarness(t *testing.T) *transportHarness {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run transport integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	orgID, userID := "test-org-"+uuid.NewString(), "test-user-"+uuid.NewString()
	var operatorID, seasonID string
	if err := pool.QueryRow(ctx, `INSERT INTO operators (better_auth_org_id,name,country,email) VALUES ($1,'Transport test','ID',$2) RETURNING id::text`, orgID, userID+"@example.test").Scan(&operatorID); err != nil {
		pool.Close()
		t.Fatalf("create operator: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO seasons (operator_id,name,type,start_date,end_date) VALUES ($1,'Test season','HAJJ',NOW(),NOW()+INTERVAL '30 days') RETURNING id::text`, operatorID).Scan(&seasonID); err != nil {
		pool.Close()
		t.Fatalf("create season: %v", err)
	}
	queries := db.New(pool)
	transportRepo := repository.NewTransportRepository(queries, pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, operatorID)
		pool.Close()
	})
	return &transportHarness{ctx: middleware.ContextWithIdentity(ctx, userID, orgID), pool: pool, queries: queries, service: NewTransportService(repository.NewOperatorRepository(queries), transportRepo), orgID: orgID, userID: userID, operator: operatorID, seasonID: seasonID}
}

func (h *transportHarness) movement(t *testing.T, status string) string {
	t.Helper()
	v, err := h.service.CreateMovement(h.ctx, h.orgID, &hajjv1.CreateMovementRequest{SeasonId: h.seasonID, Name: "Airport shuttle", Origin: "Jeddah", Destination: "Makkah", ScheduledAt: timestamppb.New(time.Now().Add(time.Hour))})
	if err != nil {
		t.Fatalf("create movement: %v", err)
	}
	if status != "scheduled" {
		if _, err := h.service.UpdateMovementStatus(h.ctx, h.orgID, &hajjv1.UpdateMovementStatusRequest{MovementId: v.Id, Status: status}); err != nil {
			t.Fatalf("set movement status: %v", err)
		}
	}
	return v.Id
}

func (h *transportHarness) vehicle(t *testing.T, movementID string, capacity int32) string {
	t.Helper()
	v, err := h.service.CreateVehicle(h.ctx, h.orgID, &hajjv1.CreateVehicleRequest{MovementId: movementID, PlateNumber: uuid.NewString()[:8], Capacity: capacity})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}
	return v.Id
}

func (h *transportHarness) pilgrim(t *testing.T, suffix string) string {
	t.Helper()
	var id string
	err := h.pool.QueryRow(h.ctx, `INSERT INTO pilgrims (season_id,operator_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$2,$3,$4,'ID',NOW()-INTERVAL '30 years','MALE') RETURNING id::text`, h.seasonID, h.operator, "Pilgrim "+suffix, "P"+uuid.NewString()).Scan(&id)
	if err != nil {
		t.Fatalf("create pilgrim: %v", err)
	}
	return id
}

func TestAssignSeat(t *testing.T) {
	t.Run("records assignment and user", func(t *testing.T) {
		h := newTransportHarness(t)
		vehicle := h.vehicle(t, h.movement(t, "scheduled"), 1)
		pilgrim := h.pilgrim(t, "one")
		result, err := h.service.AssignSeat(h.ctx, h.orgID, &hajjv1.AssignSeatRequest{VehicleId: vehicle, PilgrimId: pilgrim, SeatNumber: 1})
		if err != nil || result.SeatNumber != 1 {
			t.Fatalf("assign seat result=%+v err=%v", result, err)
		}
		var assignedBy string
		if err := h.pool.QueryRow(h.ctx, `SELECT assigned_by FROM seat_assignments WHERE id=$1::uuid`, result.Id).Scan(&assignedBy); err != nil || assignedBy != h.userID {
			t.Fatalf("assigned_by=%q err=%v", assignedBy, err)
		}
	})
	t.Run("rejects capacity and duplicate seats", func(t *testing.T) {
		h := newTransportHarness(t)
		vehicle := h.vehicle(t, h.movement(t, "scheduled"), 1)
		first := h.pilgrim(t, "one")
		second := h.pilgrim(t, "two")
		if _, err := h.service.AssignSeat(h.ctx, h.orgID, &hajjv1.AssignSeatRequest{VehicleId: vehicle, PilgrimId: first, SeatNumber: 1}); err != nil {
			t.Fatal(err)
		}
		_, err := h.service.AssignSeat(h.ctx, h.orgID, &hajjv1.AssignSeatRequest{VehicleId: vehicle, PilgrimId: second, SeatNumber: 2})
		if connect.CodeOf(err) != connect.CodeResourceExhausted {
			t.Fatalf("capacity code=%v err=%v", connect.CodeOf(err), err)
		}
	})
	t.Run("rejects duplicate seat before capacity", func(t *testing.T) {
		h := newTransportHarness(t)
		vehicle := h.vehicle(t, h.movement(t, "scheduled"), 2)
		first := h.pilgrim(t, "one")
		second := h.pilgrim(t, "two")
		_, _ = h.service.AssignSeat(h.ctx, h.orgID, &hajjv1.AssignSeatRequest{VehicleId: vehicle, PilgrimId: first, SeatNumber: 1})
		_, err := h.service.AssignSeat(h.ctx, h.orgID, &hajjv1.AssignSeatRequest{VehicleId: vehicle, PilgrimId: second, SeatNumber: 1})
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("seat code=%v err=%v", connect.CodeOf(err), err)
		}
	})
	t.Run("serializes concurrent capacity checks", func(t *testing.T) {
		h := newTransportHarness(t)
		vehicle := h.vehicle(t, h.movement(t, "scheduled"), 5)
		var wg sync.WaitGroup
		results := make(chan error, 10)
		for i := 0; i < 10; i++ {
			p := h.pilgrim(t, uuid.NewString())
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, err := h.service.AssignSeat(h.ctx, h.orgID, &hajjv1.AssignSeatRequest{VehicleId: vehicle, PilgrimId: p, SeatNumber: int32(i + 1)})
				results <- err
			}(i)
		}
		wg.Wait()
		close(results)
		ok, full := 0, 0
		for err := range results {
			if err == nil {
				ok++
			} else if connect.CodeOf(err) == connect.CodeResourceExhausted {
				full++
			} else {
				t.Fatalf("unexpected err: %v", err)
			}
		}
		if ok != 5 || full != 5 {
			t.Fatalf("success=%d full=%d", ok, full)
		}
	})
}

func TestMovementAndVehicleTransitions(t *testing.T) {
	h := newTransportHarness(t)
	for _, tc := range []struct {
		from, to string
		want     connect.Code
		valid    bool
	}{{"scheduled", "departed", 0, true}, {"scheduled", "arrived", connect.CodeFailedPrecondition, false}, {"scheduled", "cancelled", 0, true}, {"departed", "arrived", 0, true}, {"departed", "scheduled", connect.CodeFailedPrecondition, false}, {"arrived", "departed", connect.CodeFailedPrecondition, false}, {"arrived", "cancelled", connect.CodeFailedPrecondition, false}, {"cancelled", "departed", connect.CodeFailedPrecondition, false}, {"cancelled", "arrived", connect.CodeFailedPrecondition, false}} {
		t.Run("movement "+tc.from+"-"+tc.to, func(t *testing.T) {
			id := h.movement(t, tc.from)
			_, err := h.service.UpdateMovementStatus(h.ctx, h.orgID, &hajjv1.UpdateMovementStatusRequest{MovementId: id, Status: tc.to})
			if (tc.valid && err != nil) || (!tc.valid && connect.CodeOf(err) != tc.want) {
				t.Fatalf("code=%v err=%v", connect.CodeOf(err), err)
			}
		})
	}
	for _, tc := range []struct {
		from, to string
		want     connect.Code
		valid    bool
	}{{"scheduled", "departed", 0, true}, {"scheduled", "arrived", connect.CodeFailedPrecondition, false}, {"scheduled", "cancelled", 0, true}, {"departed", "arrived", 0, true}, {"departed", "scheduled", connect.CodeFailedPrecondition, false}, {"arrived", "departed", connect.CodeFailedPrecondition, false}, {"arrived", "cancelled", connect.CodeFailedPrecondition, false}, {"cancelled", "departed", connect.CodeFailedPrecondition, false}, {"cancelled", "arrived", connect.CodeFailedPrecondition, false}} {
		t.Run("vehicle "+tc.from+"-"+tc.to, func(t *testing.T) {
			mid := h.movement(t, "scheduled")
			id := h.vehicle(t, mid, 3)
			if tc.from != "scheduled" {
				_, err := h.service.UpdateVehicleStatus(h.ctx, h.orgID, &hajjv1.UpdateVehicleStatusRequest{VehicleId: id, Status: tc.from})
				if err != nil {
					t.Fatal(err)
				}
			}
			_, err := h.service.UpdateVehicleStatus(h.ctx, h.orgID, &hajjv1.UpdateVehicleStatusRequest{VehicleId: id, Status: tc.to})
			if (tc.valid && err != nil) || (!tc.valid && connect.CodeOf(err) != tc.want) {
				t.Fatalf("code=%v err=%v", connect.CodeOf(err), err)
			}
		})
	}
}

func TestDeleteMovement(t *testing.T) {
	h := newTransportHarness(t)
	for _, status := range []string{"scheduled", "departed", "arrived"} {
		t.Run(status, func(t *testing.T) {
			id := h.movement(t, status)
			_, err := h.service.DeleteMovement(h.ctx, h.orgID, &hajjv1.DeleteMovementRequest{MovementId: id})
			if (status == "scheduled" && err != nil) || (status != "scheduled" && connect.CodeOf(err) != connect.CodeFailedPrecondition) {
				t.Fatalf("code=%v err=%v", connect.CodeOf(err), err)
			}
		})
	}
}

func TestSubstitutePilgrimUnassignsTransportSeat(t *testing.T) {
	h := newTransportHarness(t)
	movementID := h.movement(t, "scheduled")
	vehicleID := h.vehicle(t, movementID, 2)
	original, replacement := h.pilgrim(t, "original"), h.pilgrim(t, "replacement")
	if _, err := h.service.AssignSeat(h.ctx, h.orgID, &hajjv1.AssignSeatRequest{VehicleId: vehicleID, PilgrimId: original, SeatNumber: 1}); err != nil {
		t.Fatal(err)
	}
	pilgrims := NewPilgrimService(repository.NewOperatorRepository(h.queries), repository.NewPilgrimRepository(h.queries), repository.NewAccommodationRepository(h.queries), repository.NewTransportRepository(h.queries, h.pool), h.pool)
	if _, err := pilgrims.SubstitutePilgrim(h.ctx, h.orgID, original, replacement); err != nil {
		t.Fatalf("substitute: %v", err)
	}
	var originalCount, replacementCount int
	if err := h.pool.QueryRow(h.ctx, `SELECT COUNT(*) FROM seat_assignments WHERE pilgrim_id=$1::uuid`, original).Scan(&originalCount); err != nil {
		t.Fatal(err)
	}
	if err := h.pool.QueryRow(h.ctx, `SELECT COUNT(*) FROM seat_assignments WHERE pilgrim_id=$1::uuid`, replacement).Scan(&replacementCount); err != nil {
		t.Fatal(err)
	}
	if originalCount != 0 || replacementCount != 0 {
		t.Fatalf("original seats=%d replacement seats=%d", originalCount, replacementCount)
	}
}
