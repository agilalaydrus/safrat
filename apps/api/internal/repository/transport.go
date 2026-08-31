package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Movement struct {
	ID, OperatorID, SeasonID, Name, Origin, Destination, Status, Mode, KloterID string
	Airline, FlightNumber, TripLeg                                              string
	ScheduledAt, CreatedAt                                                      time.Time
	VehicleCount, TotalCapacity, AssignedCount                                  int32
}
type Vehicle struct {
	ID, MovementID, OperatorID, PlateNumber, DriverName, DriverPhone, Status string
	Capacity, AssignedCount                                                  int32
	DepartedAt, ArrivedAt                                                    *time.Time
	CreatedAt                                                                time.Time
}
type SeatAssignment struct {
	ID, VehicleID, PilgrimID string
	SeatNumber               int32
	AssignedAt               time.Time
}
type ManifestPilgrim struct {
	ID, FullName, Gender, PassportNumber string
	RequiresWheelchair                   bool
	SeatNumber                           int32
}
type TransportRepository struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewTransportRepository(q *db.Queries, pool *pgxpool.Pool) *TransportRepository {
	return &TransportRepository{q: q, pool: pool}
}
func (r *TransportRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.BeginTx(ctx, pgx.TxOptions{})
}
func (r *TransportRepository) CreateMovement(ctx context.Context, op, season, name, origin, destination, mode, kloterID, airline, flightNumber, tripLeg string, scheduled time.Time) (*Movement, error) {
	o, e := pgUUID(op)
	if e != nil {
		return nil, e
	}
	s, e := pgUUID(season)
	if e != nil {
		return nil, e
	}
	v, e := r.q.CreateMovement(ctx, db.CreateMovementParams{
		OperatorID: o, SeasonID: s, Name: name, Origin: origin, Destination: destination, ScheduledAt: pgTimestamp(scheduled), Mode: mode, Column8: kloterID,
		Airline: airline, FlightNumber: flightNumber, TripLeg: tripLeg,
	})
	if e != nil {
		return nil, databaseError(e)
	}
	return movement(v), nil
}
func (r *TransportRepository) GetMovement(ctx context.Context, op, id string) (*Movement, error) {
	o, e := pgUUID(op)
	if e != nil {
		return nil, e
	}
	i, e := pgUUID(id)
	if e != nil {
		return nil, e
	}
	v, e := r.q.GetMovement(ctx, db.GetMovementParams{ID: i, OperatorID: o})
	if e != nil {
		return nil, databaseError(e)
	}
	return movement(v), nil
}
func (r *TransportRepository) ListMovements(ctx context.Context, op, season string) ([]*Movement, error) {
	o, e := pgUUID(op)
	if e != nil {
		return nil, e
	}
	s, e := pgUUID(season)
	if e != nil {
		return nil, e
	}
	vs, e := r.q.ListMovementsWithStats(ctx, db.ListMovementsWithStatsParams{OperatorID: o, SeasonID: s})
	if e != nil {
		return nil, e
	}
	out := make([]*Movement, 0, len(vs))
	for _, v := range vs {
		out = append(out, &Movement{
			ID: uuidString(v.ID), SeasonID: uuidString(v.SeasonID), OperatorID: uuidString(v.OperatorID), Name: v.Name, Origin: v.Origin, Destination: v.Destination,
			ScheduledAt: v.ScheduledAt.Time, Status: v.Status, Mode: v.Mode, KloterID: nullableUUIDString(v.KloterID), CreatedAt: v.CreatedAt.Time,
			VehicleCount: v.VehicleCount, TotalCapacity: v.TotalCapacity, AssignedCount: v.AssignedCount,
			Airline: v.Airline, FlightNumber: v.FlightNumber, TripLeg: v.TripLeg,
		})
	}
	return out, nil
}
func (r *TransportRepository) UpdateMovementStatus(ctx context.Context, op, id, status string) (*Movement, error) {
	o, e := pgUUID(op)
	if e != nil {
		return nil, e
	}
	i, e := pgUUID(id)
	if e != nil {
		return nil, e
	}
	v, e := r.q.UpdateMovementStatus(ctx, db.UpdateMovementStatusParams{ID: i, OperatorID: o, Status: status})
	if e != nil {
		return nil, databaseError(e)
	}
	return movement(v), nil
}
func (r *TransportRepository) DeleteMovement(ctx context.Context, op, id string) error {
	o, e := pgUUID(op)
	if e != nil {
		return e
	}
	i, e := pgUUID(id)
	if e != nil {
		return e
	}
	return databaseError(r.q.DeleteMovement(ctx, db.DeleteMovementParams{ID: i, OperatorID: o}))
}
func (r *TransportRepository) CreateVehicle(ctx context.Context, op, movement, plate string, capacity int32, driver, phone string) (*Vehicle, error) {
	o, e := pgUUID(op)
	if e != nil {
		return nil, e
	}
	m, e := pgUUID(movement)
	if e != nil {
		return nil, e
	}
	v, e := r.q.CreateVehicle(ctx, db.CreateVehicleParams{OperatorID: o, MovementID: m, PlateNumber: plate, Capacity: capacity, Column5: driver, Column6: phone})
	if e != nil {
		return nil, databaseError(e)
	}
	return vehicle(v), nil
}
func (r *TransportRepository) GetVehicle(ctx context.Context, op, id string) (*Vehicle, error) {
	o, e := pgUUID(op)
	if e != nil {
		return nil, e
	}
	i, e := pgUUID(id)
	if e != nil {
		return nil, e
	}
	v, e := r.q.GetVehicle(ctx, db.GetVehicleParams{ID: i, OperatorID: o})
	if e != nil {
		return nil, databaseError(e)
	}
	return vehicle(v), nil
}
func (r *TransportRepository) ListVehicles(ctx context.Context, op, movement string) ([]*Vehicle, error) {
	o, e := pgUUID(op)
	if e != nil {
		return nil, e
	}
	m, e := pgUUID(movement)
	if e != nil {
		return nil, e
	}
	vs, e := r.q.ListVehiclesByMovement(ctx, db.ListVehiclesByMovementParams{OperatorID: o, MovementID: m})
	if e != nil {
		return nil, e
	}
	out := make([]*Vehicle, 0, len(vs))
	for _, v := range vs {
		out = append(out, &Vehicle{ID: uuidString(v.ID), MovementID: uuidString(v.MovementID), OperatorID: uuidString(v.OperatorID), PlateNumber: v.PlateNumber, Capacity: v.Capacity, DriverName: v.DriverName.String, DriverPhone: v.DriverPhone.String, Status: v.Status, DepartedAt: timestampPtr(v.DepartedAt), ArrivedAt: timestampPtr(v.ArrivedAt), CreatedAt: v.CreatedAt.Time, AssignedCount: v.AssignedCount})
	}
	return out, nil
}
func (r *TransportRepository) UpdateVehicleStatus(ctx context.Context, op, id, status string) (*Vehicle, error) {
	o, e := pgUUID(op)
	if e != nil {
		return nil, e
	}
	i, e := pgUUID(id)
	if e != nil {
		return nil, e
	}
	v, e := r.q.UpdateVehicleStatus(ctx, db.UpdateVehicleStatusParams{ID: i, OperatorID: o, Status: status})
	if e != nil {
		return nil, databaseError(e)
	}
	return vehicle(v), nil
}
func (r *TransportRepository) DeleteVehicle(ctx context.Context, op, id string) error {
	o, e := pgUUID(op)
	if e != nil {
		return e
	}
	i, e := pgUUID(id)
	if e != nil {
		return e
	}
	return databaseError(r.q.DeleteVehicle(ctx, db.DeleteVehicleParams{ID: i, OperatorID: o}))
}
func (r *TransportRepository) LockVehicleTx(ctx context.Context, tx pgx.Tx, id string) (int32, error) {
	i, e := pgUUID(id)
	if e != nil {
		return 0, e
	}
	v, e := r.q.WithTx(tx).LockVehicle(ctx, i)
	if e != nil {
		return 0, databaseError(e)
	}
	return v.Capacity, nil
}
func (r *TransportRepository) CountAssignedByVehicleTx(ctx context.Context, tx pgx.Tx, id string) (int32, error) {
	i, e := pgUUID(id)
	if e != nil {
		return 0, e
	}
	return r.q.WithTx(tx).CountAssignedByVehicle(ctx, i)
}
func (r *TransportRepository) IsSeatNumberTakenTx(ctx context.Context, tx pgx.Tx, vehicle string, seat int32, exclude string) (bool, error) {
	v, e := pgUUID(vehicle)
	if e != nil {
		return false, e
	}
	p, e := pgUUID(exclude)
	if e != nil {
		return false, e
	}
	return r.q.WithTx(tx).IsSeatNumberTaken(ctx, db.IsSeatNumberTakenParams{VehicleID: v, SeatNumber: pgtype.Int4{Int32: seat, Valid: true}, PilgrimID: p})
}
func (r *TransportRepository) AssignSeatTx(ctx context.Context, tx pgx.Tx, op, vehicle, pilgrim, user string, seat int32) (*SeatAssignment, error) {
	o, e := pgUUID(op)
	if e != nil {
		return nil, e
	}
	v, e := pgUUID(vehicle)
	if e != nil {
		return nil, e
	}
	p, e := pgUUID(pilgrim)
	if e != nil {
		return nil, e
	}
	qtx := r.q.WithTx(tx)
	scope, e := branchScope(ctx, qtx, o)
	if e != nil {
		return nil, e
	}
	a, e := qtx.AssignSeat(ctx, db.AssignSeatParams{OperatorID: o, VehicleID: v, PilgrimID: p, SeatNumber: pgtype.Int4{Int32: seat, Valid: seat > 0}, AssignedBy: user, BranchScope: scope})
	if e != nil {
		return nil, databaseError(e)
	}
	return &SeatAssignment{ID: uuidString(a.ID), VehicleID: uuidString(a.VehicleID), PilgrimID: uuidString(a.PilgrimID), SeatNumber: a.SeatNumber.Int32, AssignedAt: a.AssignedAt.Time}, nil
}
func (r *TransportRepository) UnassignSeat(ctx context.Context, op, vehicle, pilgrim string) error {
	o, e := pgUUID(op)
	if e != nil {
		return e
	}
	v, e := pgUUID(vehicle)
	if e != nil {
		return e
	}
	p, e := pgUUID(pilgrim)
	if e != nil {
		return e
	}
	scope, e := branchScope(ctx, r.q, o)
	if e != nil {
		return e
	}
	rows, e := r.q.UnassignSeat(ctx, db.UnassignSeatParams{OperatorID: o, VehicleID: v, PilgrimID: p, BranchScope: scope})
	if e != nil {
		return databaseError(e)
	}
	if rows == 0 {
		return apperror.ErrNotFound
	}
	return nil
}
func (r *TransportRepository) UnassignPilgrimAllSeatsTx(ctx context.Context, tx pgx.Tx, op, pilgrim string) error {
	o, e := pgUUID(op)
	if e != nil {
		return e
	}
	p, e := pgUUID(pilgrim)
	if e != nil {
		return e
	}
	qtx := r.q.WithTx(tx)
	scope, e := branchScope(ctx, qtx, o)
	if e != nil {
		return e
	}
	_, e = qtx.UnassignPilgrimAllSeats(ctx, db.UnassignPilgrimAllSeatsParams{OperatorID: o, PilgrimID: p, BranchScope: scope})
	return e
}
func (r *TransportRepository) Manifest(ctx context.Context, op, vehicleID string) (*Vehicle, []ManifestPilgrim, error) {
	v, e := r.GetVehicle(ctx, op, vehicleID)
	if e != nil {
		return nil, nil, e
	}
	id, e := pgUUID(vehicleID)
	if e != nil {
		return nil, nil, e
	}
	o, e := pgUUID(op)
	if e != nil {
		return nil, nil, e
	}
	scope, e := branchScope(ctx, r.q, o)
	if e != nil {
		return nil, nil, e
	}
	rows, e := r.q.GetVehicleManifest(ctx, db.GetVehicleManifestParams{VehicleID: id, OperatorID: o, BranchScope: scope})
	if e != nil {
		return nil, nil, e
	}
	out := make([]ManifestPilgrim, 0, len(rows))
	for _, row := range rows {
		out = append(out, ManifestPilgrim{ID: uuidString(row.PilgrimID), FullName: row.FullName, Gender: row.Gender, PassportNumber: openKYC(row.PassportNumber), RequiresWheelchair: row.RequiresWheelchair, SeatNumber: row.SeatNumber.Int32})
	}
	return v, out, nil
}
func movement(v db.Movement) *Movement {
	return &Movement{
		ID: uuidString(v.ID), SeasonID: uuidString(v.SeasonID), OperatorID: uuidString(v.OperatorID), Name: v.Name, Origin: v.Origin, Destination: v.Destination,
		ScheduledAt: v.ScheduledAt.Time, Status: v.Status, Mode: v.Mode, KloterID: nullableUUIDString(v.KloterID), CreatedAt: v.CreatedAt.Time,
		Airline: v.Airline, FlightNumber: v.FlightNumber, TripLeg: v.TripLeg,
	}
}
func vehicle(v db.Vehicle) *Vehicle {
	return &Vehicle{ID: uuidString(v.ID), MovementID: uuidString(v.MovementID), OperatorID: uuidString(v.OperatorID), PlateNumber: v.PlateNumber, Capacity: v.Capacity, DriverName: v.DriverName.String, DriverPhone: v.DriverPhone.String, Status: v.Status, DepartedAt: timestampPtr(v.DepartedAt), ArrivedAt: timestampPtr(v.ArrivedAt), CreatedAt: v.CreatedAt.Time}
}
func timestampPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

var _ = errors.Is
