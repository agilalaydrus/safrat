package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Hotel struct {
	ID, OperatorID, SeasonID, Name, City, Address string
	StarRating                                    int32
	CheckInDate, CheckOutDate                     *time.Time
	CreatedAt                                     time.Time
}
type Room struct {
	ID, HotelID, OperatorID, RoomNumber, RoomType, Notes, Gender string
	Capacity, Floor, AllocatedCount                              int32
	CreatedAt                                                    time.Time
}
type Allocation struct {
	ID, RoomID, PilgrimID, OperatorID, AssignedBy string
	AllocatedAt                                   time.Time
}
type AccommodationRepository struct{ queries *db.Queries }

func NewAccommodationRepository(q *db.Queries) *AccommodationRepository {
	return &AccommodationRepository{queries: q}
}
func (r *AccommodationRepository) CreateHotel(ctx context.Context, operatorID, seasonID, name, city string, star int32, address string, checkInDate, checkOutDate *time.Time) (*Hotel, error) {
	op, e := pgUUID(operatorID)
	if e != nil {
		return nil, e
	}
	season, e := pgUUID(seasonID)
	if e != nil {
		return nil, e
	}
	v, e := r.queries.CreateHotel(ctx, db.CreateHotelParams{OperatorID: op, SeasonID: season, Name: name, City: city, StarRating: pgtype.Int4{Int32: star, Valid: star > 0}, Column6: address, Column7: pgDate(checkInDate), Column8: pgDate(checkOutDate)})
	if e != nil {
		return nil, e
	}
	return hotel(v), nil
}
func (r *AccommodationRepository) ListHotels(ctx context.Context, operatorID, seasonID string) ([]*Hotel, error) {
	op, e := pgUUID(operatorID)
	if e != nil {
		return nil, e
	}
	season, e := pgUUID(seasonID)
	if e != nil {
		return nil, e
	}
	vs, e := r.queries.ListHotels(ctx, db.ListHotelsParams{OperatorID: op, SeasonID: season})
	if e != nil {
		return nil, e
	}
	out := make([]*Hotel, 0, len(vs))
	for _, v := range vs {
		out = append(out, hotel(v))
	}
	return out, nil
}
func (r *AccommodationRepository) GetHotel(ctx context.Context, operatorID, hotelID string) (*Hotel, error) {
	op, e := pgUUID(operatorID)
	if e != nil {
		return nil, e
	}
	id, e := pgUUID(hotelID)
	if e != nil {
		return nil, e
	}
	v, e := r.queries.GetHotel(ctx, db.GetHotelParams{ID: id, OperatorID: op})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	return hotel(v), nil
}
func (r *AccommodationRepository) CreateRoom(ctx context.Context, opID, hotelID, number, kind string, capacity, floor int32, notes, gender string) (*Room, error) {
	op, e := pgUUID(opID)
	if e != nil {
		return nil, e
	}
	h, e := pgUUID(hotelID)
	if e != nil {
		return nil, e
	}
	v, e := r.queries.CreateRoom(ctx, db.CreateRoomParams{HotelID: h, OperatorID: op, RoomNumber: number, RoomType: kind, Capacity: capacity, Floor: pgtype.Int4{Int32: floor, Valid: floor != 0}, Column7: notes, Gender: gender})
	if e != nil {
		return nil, e
	}
	return room(v), nil
}
func (r *AccommodationRepository) BulkCreateRooms(ctx context.Context, opID, hotelID string, numbers []string, kind string, capacity, floor int32, gender, notes string) ([]*Room, error) {
	op, e := pgUUID(opID)
	if e != nil {
		return nil, e
	}
	h, e := pgUUID(hotelID)
	if e != nil {
		return nil, e
	}
	values, e := r.queries.BulkCreateRooms(ctx, db.BulkCreateRoomsParams{HotelID: h, OperatorID: op, Column3: numbers, RoomType: kind, Capacity: capacity, Floor: pgtype.Int4{Int32: floor, Valid: floor != 0}, Gender: gender, Column7: notes})
	if e != nil {
		return nil, e
	}
	rooms := make([]*Room, 0, len(values))
	for _, value := range values {
		rooms = append(rooms, room(value))
	}
	return rooms, nil
}
func (r *AccommodationRepository) ListRooms(ctx context.Context, opID, hotelID string) ([]*Room, error) {
	op, e := pgUUID(opID)
	if e != nil {
		return nil, e
	}
	h, e := pgUUID(hotelID)
	if e != nil {
		return nil, e
	}
	vs, e := r.queries.ListRoomsByHotel(ctx, db.ListRoomsByHotelParams{HotelID: h, OperatorID: op})
	if e != nil {
		return nil, e
	}
	out := make([]*Room, 0, len(vs))
	for _, v := range vs {
		out = append(out, &Room{ID: uuidString(v.ID), HotelID: uuidString(v.HotelID), OperatorID: uuidString(v.OperatorID), RoomNumber: v.RoomNumber, RoomType: v.RoomType, Capacity: v.Capacity, Floor: v.Floor.Int32, Notes: v.Notes.String, Gender: v.Gender, CreatedAt: v.CreatedAt.Time, AllocatedCount: v.AllocatedCount})
	}
	return out, nil
}
func (r *AccommodationRepository) GetRoom(ctx context.Context, opID, roomID string) (*Room, error) {
	op, e := pgUUID(opID)
	if e != nil {
		return nil, e
	}
	id, e := pgUUID(roomID)
	if e != nil {
		return nil, e
	}
	v, e := r.queries.GetRoom(ctx, db.GetRoomParams{ID: id, OperatorID: op})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	return room(v), nil
}
func (r *AccommodationRepository) ListAllocations(ctx context.Context, opID, roomID string) ([]*Allocation, error) {
	op, e := pgUUID(opID)
	if e != nil {
		return nil, e
	}
	id, e := pgUUID(roomID)
	if e != nil {
		return nil, e
	}
	scope, e := branchScope(ctx, r.queries, op)
	if e != nil {
		return nil, e
	}
	vs, e := r.queries.ListAllocationsByRoom(ctx, db.ListAllocationsByRoomParams{RoomID: id, OperatorID: op, BranchScope: scope})
	if e != nil {
		return nil, e
	}
	out := make([]*Allocation, 0, len(vs))
	for _, v := range vs {
		out = append(out, allocation(v))
	}
	return out, nil
}
func (r *AccommodationRepository) CountAllocated(ctx context.Context, operatorID, roomID string) (int64, error) {
	op, e := pgUUID(operatorID)
	if e != nil {
		return 0, e
	}
	id, e := pgUUID(roomID)
	if e != nil {
		return 0, e
	}
	return r.queries.CountAllocatedByRoom(ctx, db.CountAllocatedByRoomParams{RoomID: id, OperatorID: op})
}

// GetAllocationForHotel checks whether the pilgrim already holds a room in
// THIS hotel specifically — not whether they hold a room anywhere, since
// one per hotel (Makkah + Madinah) is the valid, intended case.
func (r *AccommodationRepository) GetAllocationForHotel(ctx context.Context, opID, pilgrimID, hotelID string) (*Allocation, error) {
	op, e := pgUUID(opID)
	if e != nil {
		return nil, e
	}
	p, e := pgUUID(pilgrimID)
	if e != nil {
		return nil, e
	}
	h, e := pgUUID(hotelID)
	if e != nil {
		return nil, e
	}
	scope, e := branchScope(ctx, r.queries, op)
	if e != nil {
		return nil, e
	}
	v, e := r.queries.GetAllocationForHotel(ctx, db.GetAllocationForHotelParams{OperatorID: op, PilgrimID: p, HotelID: h, BranchScope: scope})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	return allocation(v), nil
}
func (r *AccommodationRepository) Allocate(ctx context.Context, opID, roomID, hotelID, pilgrimID, assignedBy string) (*Allocation, error) {
	op, e := pgUUID(opID)
	if e != nil {
		return nil, e
	}
	rID, e := pgUUID(roomID)
	if e != nil {
		return nil, e
	}
	hID, e := pgUUID(hotelID)
	if e != nil {
		return nil, e
	}
	p, e := pgUUID(pilgrimID)
	if e != nil {
		return nil, e
	}
	scope, e := branchScope(ctx, r.queries, op)
	if e != nil {
		return nil, e
	}
	v, e := r.queries.AllocatePilgrimTx(ctx, db.AllocatePilgrimTxParams{OperatorID: op, RoomID: rID, HotelID: hID, PilgrimID: p, AssignedBy: assignedBy, BranchScope: scope})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	return allocation(v), nil
}
func (r *AccommodationRepository) Deallocate(ctx context.Context, opID, pilgrimID, roomID string) error {
	op, e := pgUUID(opID)
	if e != nil {
		return e
	}
	p, e := pgUUID(pilgrimID)
	if e != nil {
		return e
	}
	rID, e := pgUUID(roomID)
	if e != nil {
		return e
	}
	scope, e := branchScope(ctx, r.queries, op)
	if e != nil {
		return e
	}
	rows, e := r.queries.DeallocatePilgrim(ctx, db.DeallocatePilgrimParams{OperatorID: op, PilgrimID: p, RoomID: rID, BranchScope: scope})
	if e != nil {
		return databaseError(e)
	}
	if rows == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *AccommodationRepository) TransferAllocationTx(ctx context.Context, tx pgx.Tx, originalID, replacementID, operatorID string) error {
	originalUUID, err := pgUUID(originalID)
	if err != nil {
		return err
	}
	replacementUUID, err := pgUUID(replacementID)
	if err != nil {
		return err
	}
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	queries := r.queries.WithTx(tx)
	scope, err := branchScope(ctx, queries, operatorUUID)
	if err != nil {
		return err
	}
	rows, err := queries.TransferAllocation(ctx, db.TransferAllocationParams{OriginalID: originalUUID, ReplacementID: replacementUUID, OperatorID: operatorUUID, BranchScope: scope})
	if err != nil {
		return databaseError(err)
	}
	if rows == 0 {
		return apperror.ErrNotFound
	}
	return nil
}
func hotel(v db.Hotel) *Hotel {
	return &Hotel{ID: uuidString(v.ID), OperatorID: uuidString(v.OperatorID), SeasonID: uuidString(v.SeasonID), Name: v.Name, City: v.City, Address: v.Address.String, StarRating: v.StarRating.Int32, CheckInDate: timePtr(v.CheckInDate), CheckOutDate: timePtr(v.CheckOutDate), CreatedAt: v.CreatedAt.Time}
}
func room(v db.Room) *Room {
	return &Room{ID: uuidString(v.ID), HotelID: uuidString(v.HotelID), OperatorID: uuidString(v.OperatorID), RoomNumber: v.RoomNumber, RoomType: v.RoomType, Capacity: v.Capacity, Floor: v.Floor.Int32, Notes: v.Notes.String, Gender: v.Gender, CreatedAt: v.CreatedAt.Time}
}

type PilgrimRoomAssignment struct {
	PilgrimID, HotelName, RoomNumber, RoomType, RoomID, HotelID string
}

func (r *AccommodationRepository) ListPilgrimRoomAssignments(ctx context.Context, opID, seasonID string) ([]*PilgrimRoomAssignment, error) {
	op, e := pgUUID(opID)
	if e != nil {
		return nil, e
	}
	season, e := pgUUID(seasonID)
	if e != nil {
		return nil, e
	}
	scope, e := branchScope(ctx, r.queries, op)
	if e != nil {
		return nil, e
	}
	rows, e := r.queries.ListPilgrimRoomAssignments(ctx, db.ListPilgrimRoomAssignmentsParams{OperatorID: op, SeasonID: season, BranchScope: scope})
	if e != nil {
		return nil, e
	}
	result := make([]*PilgrimRoomAssignment, 0, len(rows))
	for _, row := range rows {
		result = append(result, &PilgrimRoomAssignment{PilgrimID: uuidString(row.PilgrimID), HotelName: row.HotelName, RoomNumber: row.RoomNumber, RoomType: row.RoomType, RoomID: uuidString(row.RoomID), HotelID: uuidString(row.HotelID)})
	}
	return result, nil
}

func allocation(v db.RoomAllocation) *Allocation {
	return &Allocation{ID: uuidString(v.ID), RoomID: uuidString(v.RoomID), PilgrimID: uuidString(v.PilgrimID), OperatorID: uuidString(v.OperatorID), AssignedBy: v.AssignedBy, AllocatedAt: v.AllocatedAt.Time}
}

func pgDate(value *time.Time) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *value, Valid: true}
}

func timePtr(value pgtype.Date) *time.Time {
	if !value.Valid {
		return nil
	}
	timestamp := value.Time
	return &timestamp
}
