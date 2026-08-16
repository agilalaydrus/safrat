package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TransportService struct {
	operatorRepo  *repository.OperatorRepository
	transportRepo *repository.TransportRepository
	auditRepo     *repository.AuditRepository
}

func NewTransportService(o *repository.OperatorRepository, t *repository.TransportRepository, audit *repository.AuditRepository) *TransportService {
	return &TransportService{operatorRepo: o, transportRepo: t, auditRepo: audit}
}

func (s *TransportService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepo.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "movement", entityID, message)
}
func (s *TransportService) operator(ctx context.Context, org string) (string, error) {
	o, e := s.operatorRepo.GetByBetterAuthOrgID(ctx, org)
	if e != nil {
		return "", e
	}
	return o.ID, nil
}
var validMovementModes = map[string]bool{"BUS": true, "FLIGHT": true, "TRAIN": true}

func (s *TransportService) CreateMovement(ctx context.Context, org string, r *hajjv1.CreateMovementRequest) (*hajjv1.Movement, error) {
	if r == nil || r.SeasonId == "" || r.Name == "" || r.Origin == "" || r.Destination == "" || r.ScheduledAt == nil {
		return nil, serviceError("TransportService.CreateMovement", apperror.ErrValidation)
	}
	mode := r.Mode
	if mode == "" {
		mode = "BUS"
	}
	if !validMovementModes[mode] {
		return nil, serviceError("TransportService.CreateMovement", apperror.ErrValidation)
	}
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.CreateMovement", e)
	}
	v, e := s.transportRepo.CreateMovement(ctx, op, r.SeasonId, r.Name, r.Origin, r.Destination, mode, r.KloterId, r.ScheduledAt.AsTime())
	if e != nil {
		return nil, serviceError("TransportService.CreateMovement", e)
	}
	s.logActivity(ctx, op, "movement_created", v.ID, fmt.Sprintf("Pergerakan %s dijadwalkan (%s → %s)", v.Name, v.Origin, v.Destination))
	return movementMessage(v), nil
}
func (s *TransportService) GetMovement(ctx context.Context, org string, r *hajjv1.GetMovementRequest) (*hajjv1.Movement, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.GetMovement", e)
	}
	v, e := s.transportRepo.GetMovement(ctx, op, r.MovementId)
	if e != nil {
		return nil, serviceError("TransportService.GetMovement", e)
	}
	return movementMessage(v), nil
}
func (s *TransportService) ListMovements(ctx context.Context, org string, r *hajjv1.ListMovementsRequest) (*hajjv1.ListMovementsResponse, error) {
	if r == nil || r.SeasonId == "" {
		return nil, serviceError("TransportService.ListMovements", apperror.ErrValidation)
	}
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.ListMovements", e)
	}
	vs, e := s.transportRepo.ListMovements(ctx, op, r.SeasonId)
	if e != nil {
		return nil, serviceError("TransportService.ListMovements", e)
	}
	out := &hajjv1.ListMovementsResponse{}
	for _, v := range vs {
		out.Movements = append(out.Movements, movementMessage(v))
	}
	return out, nil
}
func (s *TransportService) UpdateMovementStatus(ctx context.Context, org string, r *hajjv1.UpdateMovementStatusRequest) (*hajjv1.Movement, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.UpdateMovementStatus", e)
	}
	v, e := s.transportRepo.GetMovement(ctx, op, r.MovementId)
	if e != nil {
		return nil, serviceError("TransportService.UpdateMovementStatus", e)
	}
	if !transitionAllowed(v.Status, r.Status) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("cannot transition from %s to %s", v.Status, r.Status))
	}
	v, e = s.transportRepo.UpdateMovementStatus(ctx, op, r.MovementId, r.Status)
	if e != nil {
		return nil, serviceError("TransportService.UpdateMovementStatus", e)
	}
	return movementMessage(v), nil
}
func (s *TransportService) DeleteMovement(ctx context.Context, org string, r *hajjv1.DeleteMovementRequest) (*emptypb.Empty, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.DeleteMovement", e)
	}
	v, e := s.transportRepo.GetMovement(ctx, op, r.MovementId)
	if e != nil {
		return nil, serviceError("TransportService.DeleteMovement", e)
	}
	if v.Status != "scheduled" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("cannot delete a movement that has departed or arrived"))
	}
	if e = s.transportRepo.DeleteMovement(ctx, op, r.MovementId); e != nil {
		return nil, serviceError("TransportService.DeleteMovement", e)
	}
	return &emptypb.Empty{}, nil
}
func (s *TransportService) CreateVehicle(ctx context.Context, org string, r *hajjv1.CreateVehicleRequest) (*hajjv1.Vehicle, error) {
	if r == nil || r.MovementId == "" || r.PlateNumber == "" || r.Capacity < 1 || r.Capacity > 100 {
		return nil, serviceError("TransportService.CreateVehicle", apperror.ErrValidation)
	}
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.CreateVehicle", e)
	}
	if _, e = s.transportRepo.GetMovement(ctx, op, r.MovementId); e != nil {
		return nil, serviceError("TransportService.CreateVehicle", e)
	}
	v, e := s.transportRepo.CreateVehicle(ctx, op, r.MovementId, strings.ToUpper(r.PlateNumber), r.Capacity, r.DriverName, r.DriverPhone)
	if e != nil {
		return nil, serviceError("TransportService.CreateVehicle", e)
	}
	return vehicleMessage(v), nil
}
func (s *TransportService) ListVehicles(ctx context.Context, org string, r *hajjv1.ListVehiclesRequest) (*hajjv1.ListVehiclesResponse, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.ListVehicles", e)
	}
	vs, e := s.transportRepo.ListVehicles(ctx, op, r.MovementId)
	if e != nil {
		return nil, serviceError("TransportService.ListVehicles", e)
	}
	out := &hajjv1.ListVehiclesResponse{}
	for _, v := range vs {
		out.Vehicles = append(out.Vehicles, vehicleMessage(v))
	}
	return out, nil
}
func (s *TransportService) UpdateVehicleStatus(ctx context.Context, org string, r *hajjv1.UpdateVehicleStatusRequest) (*hajjv1.Vehicle, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.UpdateVehicleStatus", e)
	}
	v, e := s.transportRepo.GetVehicle(ctx, op, r.VehicleId)
	if e != nil {
		return nil, serviceError("TransportService.UpdateVehicleStatus", e)
	}
	if !transitionAllowed(v.Status, r.Status) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("cannot transition from %s to %s", v.Status, r.Status))
	}
	v, e = s.transportRepo.UpdateVehicleStatus(ctx, op, r.VehicleId, r.Status)
	if e != nil {
		return nil, serviceError("TransportService.UpdateVehicleStatus", e)
	}
	return vehicleMessage(v), nil
}
func (s *TransportService) DeleteVehicle(ctx context.Context, org string, r *hajjv1.DeleteVehicleRequest) (*emptypb.Empty, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.DeleteVehicle", e)
	}
	if e = s.transportRepo.DeleteVehicle(ctx, op, r.VehicleId); e != nil {
		return nil, serviceError("TransportService.DeleteVehicle", e)
	}
	return &emptypb.Empty{}, nil
}
func (s *TransportService) AssignSeat(ctx context.Context, org string, r *hajjv1.AssignSeatRequest) (*hajjv1.SeatAssignment, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.AssignSeat", e)
	}
	v, e := s.transportRepo.GetVehicle(ctx, op, r.VehicleId)
	if e != nil {
		return nil, serviceError("TransportService.AssignSeat", e)
	}
	tx, e := s.transportRepo.BeginTx(ctx)
	if e != nil {
		return nil, serviceError("TransportService.AssignSeat", e)
	}
	defer tx.Rollback(ctx)
	cap, e := s.transportRepo.LockVehicleTx(ctx, tx, r.VehicleId)
	if e != nil {
		return nil, serviceError("TransportService.AssignSeat", e)
	}
	count, e := s.transportRepo.CountAssignedByVehicleTx(ctx, tx, r.VehicleId)
	if e != nil {
		return nil, serviceError("TransportService.AssignSeat", e)
	}
	if count >= cap {
		return nil, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("vehicle %s is full (%d/%d)", v.PlateNumber, count, cap))
	}
	if r.SeatNumber > 0 {
		taken, e := s.transportRepo.IsSeatNumberTakenTx(ctx, tx, r.VehicleId, r.SeatNumber, r.PilgrimId)
		if e != nil {
			return nil, serviceError("TransportService.AssignSeat", e)
		}
		if taken {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("seat %d is already taken", r.SeatNumber))
		}
	}
	a, e := s.transportRepo.AssignSeatTx(ctx, tx, op, r.VehicleId, r.PilgrimId, middleware.UserIDFromCtx(ctx), r.SeatNumber)
	if e != nil {
		return nil, serviceError("TransportService.AssignSeat", e)
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, serviceError("TransportService.AssignSeat", e)
	}
	return &hajjv1.SeatAssignment{Id: a.ID, VehicleId: a.VehicleID, PilgrimId: a.PilgrimID, SeatNumber: a.SeatNumber, AssignedAt: timestamppb.New(a.AssignedAt)}, nil
}
func (s *TransportService) UnassignSeat(ctx context.Context, org string, r *hajjv1.UnassignSeatRequest) (*emptypb.Empty, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.UnassignSeat", e)
	}
	if e = s.transportRepo.UnassignSeat(ctx, op, r.VehicleId, r.PilgrimId); e != nil {
		return nil, serviceError("TransportService.UnassignSeat", e)
	}
	return &emptypb.Empty{}, nil
}
func (s *TransportService) Manifest(ctx context.Context, org string, r *hajjv1.GetVehicleManifestRequest) (*hajjv1.VehicleManifest, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("TransportService.Manifest", e)
	}
	v, ps, e := s.transportRepo.Manifest(ctx, op, r.VehicleId)
	if e != nil {
		return nil, serviceError("TransportService.Manifest", e)
	}
	out := &hajjv1.VehicleManifest{Vehicle: vehicleMessage(v)}
	for _, p := range ps {
		out.Pilgrims = append(out.Pilgrims, &hajjv1.PilgrimOnVehicle{Id: p.ID, FullName: p.FullName, Gender: p.Gender, PassportNumber: p.PassportNumber, RequiresWheelchair: p.RequiresWheelchair, SeatNumber: p.SeatNumber})
	}
	return out, nil
}
func transitionAllowed(from, to string) bool {
	m := map[string]map[string]bool{"scheduled": {"departed": true, "cancelled": true}, "departed": {"arrived": true, "cancelled": true}, "arrived": {}, "cancelled": {}}
	return m[from][to]
}
func movementMessage(v *repository.Movement) *hajjv1.Movement {
	return &hajjv1.Movement{Id: v.ID, OperatorId: v.OperatorID, SeasonId: v.SeasonID, Name: v.Name, Origin: v.Origin, Destination: v.Destination, ScheduledAt: timestamppb.New(v.ScheduledAt), Status: v.Status, Mode: v.Mode, KloterId: v.KloterID, VehicleCount: v.VehicleCount, TotalCapacity: v.TotalCapacity, AssignedCount: v.AssignedCount, CreatedAt: timestamppb.New(v.CreatedAt)}
}
func vehicleMessage(v *repository.Vehicle) *hajjv1.Vehicle {
	return &hajjv1.Vehicle{Id: v.ID, MovementId: v.MovementID, OperatorId: v.OperatorID, PlateNumber: v.PlateNumber, Capacity: v.Capacity, DriverName: v.DriverName, DriverPhone: v.DriverPhone, Status: v.Status, AssignedCount: v.AssignedCount, DepartedAt: timeMessage(v.DepartedAt), ArrivedAt: timeMessage(v.ArrivedAt), CreatedAt: timestamppb.New(v.CreatedAt)}
}
func timeMessage(v *time.Time) *timestamppb.Timestamp {
	if v == nil {
		return nil
	}
	return timestamppb.New(*v)
}
