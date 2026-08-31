package service

import (
	"context"
	"errors"
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

type AccommodationService struct {
	operators *repository.OperatorRepository
	pilgrims  *repository.PilgrimRepository
	repo      *repository.AccommodationRepository
	audit     *repository.AuditRepository
}

func NewAccommodationService(o *repository.OperatorRepository, p *repository.PilgrimRepository, r *repository.AccommodationRepository, audit *repository.AuditRepository) *AccommodationService {
	return &AccommodationService{operators: o, pilgrims: p, repo: r, audit: audit}
}

func (s *AccommodationService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.audit.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "accommodation", entityID, message)
}
func (s *AccommodationService) operator(ctx context.Context, org string) (string, error) {
	v, e := s.operators.GetByBetterAuthOrgID(ctx, org)
	if e != nil {
		return "", e
	}
	return v.ID, nil
}
func (s *AccommodationService) CreateHotel(ctx context.Context, org string, r *hajjv1.CreateHotelRequest) (*hajjv1.Hotel, error) {
	if r == nil || r.SeasonId == "" || r.Name == "" || r.City == "" {
		return nil, serviceError("AccommodationService.CreateHotel", apperror.ErrValidation)
	}
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.CreateHotel", e)
	}
	v, e := s.repo.CreateHotel(ctx, op, r.SeasonId, r.Name, r.City, r.StarRating, r.Address, timestampPtr(r.CheckInDate), timestampPtr(r.CheckOutDate))
	if e != nil {
		return nil, serviceError("AccommodationService.CreateHotel", e)
	}
	return hotelMessage(v), nil
}
func (s *AccommodationService) GetHotel(ctx context.Context, org string, r *hajjv1.GetHotelRequest) (*hajjv1.Hotel, error) {
	if r == nil || r.HotelId == "" {
		return nil, serviceError("AccommodationService.GetHotel", apperror.ErrValidation)
	}
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.GetHotel", e)
	}
	v, e := s.repo.GetHotel(ctx, op, r.HotelId)
	if e != nil {
		return nil, serviceError("AccommodationService.GetHotel", e)
	}
	return hotelMessage(v), nil
}
func (s *AccommodationService) ListHotels(ctx context.Context, org string, r *hajjv1.ListHotelsRequest) (*hajjv1.ListHotelsResponse, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.ListHotels", e)
	}
	vs, e := s.repo.ListHotels(ctx, op, r.SeasonId)
	if e != nil {
		return nil, serviceError("AccommodationService.ListHotels", e)
	}
	out := &hajjv1.ListHotelsResponse{}
	for _, v := range vs {
		out.Hotels = append(out.Hotels, hotelMessage(v))
	}
	return out, nil
}
func (s *AccommodationService) CreateRoom(ctx context.Context, org string, r *hajjv1.CreateRoomRequest) (*hajjv1.Room, error) {
	if r == nil || r.HotelId == "" || r.RoomNumber == "" || r.RoomType == "" || r.Capacity < 1 || !validRoomGender(r.Gender) {
		return nil, serviceError("AccommodationService.CreateRoom", apperror.ErrValidation)
	}
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.CreateRoom", e)
	}
	v, e := s.repo.CreateRoom(ctx, op, r.HotelId, r.RoomNumber, r.RoomType, r.Capacity, r.Floor, r.Notes, strings.ToLower(r.Gender))
	if e != nil {
		return nil, serviceError("AccommodationService.CreateRoom", e)
	}
	return roomMessage(v), nil
}
func (s *AccommodationService) BulkCreateRooms(ctx context.Context, org string, r *hajjv1.BulkCreateRoomsRequest) (*hajjv1.BulkCreateRoomsResponse, error) {
	if r == nil || r.HotelId == "" || len(r.RoomNumbers) == 0 || r.RoomType == "" || r.Capacity < 1 || !validRoomGender(r.Gender) {
		return nil, serviceError("AccommodationService.BulkCreateRooms", apperror.ErrValidation)
	}
	numbers := make([]string, 0, len(r.RoomNumbers))
	seen := make(map[string]struct{}, len(r.RoomNumbers))
	for _, number := range r.RoomNumbers {
		number = strings.ToUpper(strings.TrimSpace(number))
		if number == "" {
			return nil, serviceError("AccommodationService.BulkCreateRooms", apperror.ErrValidation)
		}
		if _, exists := seen[number]; exists {
			return nil, serviceError("AccommodationService.BulkCreateRooms", apperror.ErrAlreadyExists)
		}
		seen[number] = struct{}{}
		numbers = append(numbers, number)
	}
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.BulkCreateRooms", e)
	}
	values, e := s.repo.BulkCreateRooms(ctx, op, r.HotelId, numbers, r.RoomType, r.Capacity, r.Floor, strings.ToLower(r.Gender), r.Notes)
	if e != nil {
		return nil, serviceError("AccommodationService.BulkCreateRooms", e)
	}
	out := &hajjv1.BulkCreateRoomsResponse{Rooms: make([]*hajjv1.Room, 0, len(values))}
	for _, value := range values {
		out.Rooms = append(out.Rooms, roomMessage(value))
	}
	return out, nil
}
func (s *AccommodationService) ListRooms(ctx context.Context, org string, r *hajjv1.ListRoomsRequest) (*hajjv1.ListRoomsResponse, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.ListRooms", e)
	}
	vs, e := s.repo.ListRooms(ctx, op, r.HotelId)
	if e != nil {
		return nil, serviceError("AccommodationService.ListRooms", e)
	}
	out := &hajjv1.ListRoomsResponse{}
	for _, v := range vs {
		out.Rooms = append(out.Rooms, roomMessage(v))
	}
	return out, nil
}
func (s *AccommodationService) AllocatePilgrim(ctx context.Context, org string, r *hajjv1.AllocatePilgrimRequest) (*hajjv1.RoomAllocation, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.AllocatePilgrim", e)
	}
	room, e := s.repo.GetRoom(ctx, op, r.RoomId)
	if e != nil {
		return nil, serviceError("AccommodationService.AllocatePilgrim", e)
	}
	p, e := s.pilgrims.Get(ctx, op, r.PilgrimId)
	if e != nil {
		return nil, serviceError("AccommodationService.AllocatePilgrim", e)
	}
	count, e := s.repo.CountAllocated(ctx, op, room.ID)
	if e != nil {
		return nil, serviceError("AccommodationService.AllocatePilgrim", e)
	}
	if count >= int64(room.Capacity) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("room capacity exceeded"))
	}
	_, e = s.repo.GetAllocationForHotel(ctx, op, p.ID, room.HotelID)
	if e == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("pilgrim already has a room in this hotel"))
	}
	if !errors.Is(e, apperror.ErrNotFound) {
		return nil, serviceError("AccommodationService.AllocatePilgrim", e)
	}
	if room.Gender != "family" && strings.ToLower(p.Gender) != room.Gender {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("room is designated %s only", room.Gender))
	}
	v, e := s.repo.Allocate(ctx, op, room.ID, room.HotelID, p.ID, middleware.UserIDFromCtx(ctx))
	if e != nil {
		return nil, serviceError("AccommodationService.AllocatePilgrim", e)
	}
	s.logActivity(ctx, op, "room_allocated", p.ID, fmt.Sprintf("%s ditempatkan di kamar %s", p.FullName, room.RoomNumber))
	return allocationMessage(v), nil
}
func (s *AccommodationService) DeallocatePilgrim(ctx context.Context, org string, r *hajjv1.DeallocatePilgrimRequest) (*emptypb.Empty, error) {
	if r == nil || r.PilgrimId == "" || r.RoomId == "" {
		return nil, serviceError("AccommodationService.DeallocatePilgrim", apperror.ErrValidation)
	}
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.DeallocatePilgrim", e)
	}
	p, e := s.pilgrims.Get(ctx, op, r.PilgrimId)
	if e != nil {
		return nil, serviceError("AccommodationService.DeallocatePilgrim", e)
	}
	if e = s.repo.Deallocate(ctx, op, r.PilgrimId, r.RoomId); e != nil {
		return nil, serviceError("AccommodationService.DeallocatePilgrim", e)
	}
	s.logActivity(ctx, op, "room_deallocated", p.ID, fmt.Sprintf("%s dikeluarkan dari kamar", p.FullName))
	return &emptypb.Empty{}, nil
}
func (s *AccommodationService) ListPilgrimRoomAssignments(ctx context.Context, org string, r *hajjv1.ListPilgrimRoomAssignmentsRequest) (*hajjv1.ListPilgrimRoomAssignmentsResponse, error) {
	if r == nil || r.SeasonId == "" {
		return nil, serviceError("AccommodationService.ListPilgrimRoomAssignments", apperror.ErrValidation)
	}
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.ListPilgrimRoomAssignments", e)
	}
	rows, e := s.repo.ListPilgrimRoomAssignments(ctx, op, r.SeasonId)
	if e != nil {
		return nil, serviceError("AccommodationService.ListPilgrimRoomAssignments", e)
	}
	out := &hajjv1.ListPilgrimRoomAssignmentsResponse{Assignments: make([]*hajjv1.PilgrimRoomAssignment, 0, len(rows))}
	for _, row := range rows {
		out.Assignments = append(out.Assignments, &hajjv1.PilgrimRoomAssignment{PilgrimId: row.PilgrimID, HotelName: row.HotelName, RoomNumber: row.RoomNumber, RoomType: row.RoomType, RoomId: row.RoomID, HotelId: row.HotelID})
	}
	return out, nil
}
func (s *AccommodationService) Manifest(ctx context.Context, org string, r *hajjv1.GetRoomManifestRequest) (*hajjv1.RoomManifest, error) {
	op, e := s.operator(ctx, org)
	if e != nil {
		return nil, serviceError("AccommodationService.Manifest", e)
	}
	room, e := s.repo.GetRoom(ctx, op, r.RoomId)
	if e != nil {
		return nil, serviceError("AccommodationService.Manifest", e)
	}
	as, e := s.repo.ListAllocations(ctx, op, room.ID)
	if e != nil {
		return nil, serviceError("AccommodationService.Manifest", e)
	}
	// The occupant list follows the actor's branch scope, but availability is
	// a physical, operator-wide constraint. Using len(as) here would make every
	// branch believe the other branches' beds were still empty.
	allocated, e := s.repo.CountAllocated(ctx, op, room.ID)
	if e != nil {
		return nil, serviceError("AccommodationService.Manifest", e)
	}
	available := room.Capacity - int32(allocated)
	if available < 0 {
		available = 0
	}
	out := &hajjv1.RoomManifest{Room: roomMessage(room), AvailableCapacity: available}
	for _, a := range as {
		p, err := s.pilgrims.Get(ctx, op, a.PilgrimID)
		if err != nil {
			return nil, serviceError("AccommodationService.Manifest", err)
		}
		out.Pilgrims = append(out.Pilgrims, pilgrimMessage(p))
	}
	return out, nil
}
func hotelMessage(v *repository.Hotel) *hajjv1.Hotel {
	return &hajjv1.Hotel{Id: v.ID, OperatorId: v.OperatorID, SeasonId: v.SeasonID, Name: v.Name, City: v.City, StarRating: v.StarRating, Address: v.Address, CreatedAt: timestamppb.New(v.CreatedAt), CheckInDate: timestampMessage(v.CheckInDate), CheckOutDate: timestampMessage(v.CheckOutDate)}
}
func roomMessage(v *repository.Room) *hajjv1.Room {
	return &hajjv1.Room{Id: v.ID, HotelId: v.HotelID, OperatorId: v.OperatorID, RoomNumber: v.RoomNumber, RoomType: v.RoomType, Capacity: v.Capacity, Floor: v.Floor, Notes: v.Notes, CreatedAt: timestamppb.New(v.CreatedAt), AllocatedCount: v.AllocatedCount, Gender: v.Gender}
}
func allocationMessage(v *repository.Allocation) *hajjv1.RoomAllocation {
	return &hajjv1.RoomAllocation{Id: v.ID, RoomId: v.RoomID, PilgrimId: v.PilgrimID, OperatorId: v.OperatorID, AllocatedAt: timestamppb.New(v.AllocatedAt), AssignedBy: v.AssignedBy}
}

func validRoomGender(value string) bool {
	value = strings.ToLower(value)
	return value == "male" || value == "female" || value == "family"
}

func timestampPtr(value *timestamppb.Timestamp) *time.Time {
	if value == nil {
		return nil
	}
	timestamp := value.AsTime()
	return &timestamp
}

func timestampMessage(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}
