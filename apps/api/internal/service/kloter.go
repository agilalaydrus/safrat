package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/events"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type KloterService struct {
	operatorRepository *repository.OperatorRepository
	kloterRepository   *repository.KloterRepository
	auditRepository    *repository.AuditRepository
	outboxRepository   *repository.OutboxRepository
	db                 *pgxpool.Pool
	eventBus           *events.Bus
}

func NewKloterService(operators *repository.OperatorRepository, kloters *repository.KloterRepository, audit *repository.AuditRepository, outbox *repository.OutboxRepository, db *pgxpool.Pool, bus *events.Bus) *KloterService {
	return &KloterService{operatorRepository: operators, kloterRepository: kloters, auditRepository: audit, outboxRepository: outbox, db: db, eventBus: bus}
}

// kloterStatusPushText is the "seluruh jamaah kloter" push copy per the
// cascade map in SPEC_GROUP_KLOTER_MUTTAWWIF.md section 4.
var kloterStatusPushText = map[string]string{
	"DEPARTED":       "Perjalanan ibadah Anda dimulai",
	"IN_SAUDI":       "Selamat tiba di Tanah Suci",
	"DEPARTED_SAUDI": "Dalam perjalanan pulang ke Indonesia",
	"COMPLETED":      "Selamat tiba! Sertifikat Anda siap diunduh",
}

// kloterStatusOrder is forward-only, except the one explicit rollback
// business rule: CONFIRMED -> DRAFT (e.g. an operator undoes an early
// confirmation before departure prep has really started).
var kloterStatusOrder = []string{"DRAFT", "CONFIRMED", "DEPARTED", "IN_SAUDI", "DEPARTED_SAUDI", "COMPLETED"}

func kloterStatusIndex(status string) int {
	for i, s := range kloterStatusOrder {
		if s == status {
			return i
		}
	}
	return -1
}

// kloterToJourneyStatus is the cascade map from section 4 of
// SPEC_GROUP_KLOTER_MUTTAWWIF.md — only kloter transitions with a direct
// 1:1 journey-status equivalent cascade; DRAFT/CONFIRMED don't.
var kloterToJourneyStatus = map[string]string{
	"DEPARTED":       "DEPARTED_INDONESIA",
	"IN_SAUDI":       "ARRIVED_SAUDI",
	"DEPARTED_SAUDI": "DEPARTED_SAUDI",
	"COMPLETED":      "ARRIVED_INDONESIA",
}

func (s *KloterService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "kloter", entityID, message)
}

func (s *KloterService) ListKloters(ctx context.Context, orgID string, req *hajjv1.ListKlotersRequest) (*hajjv1.ListKlotersResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("KloterService.ListKloters", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.ListKloters", err)
	}
	kloters, err := s.kloterRepository.ListForOperator(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("KloterService.ListKloters", err)
	}
	result := &hajjv1.ListKlotersResponse{Kloters: make([]*hajjv1.Kloter, 0, len(kloters))}
	for _, k := range kloters {
		result.Kloters = append(result.Kloters, kloterMessage(k))
	}
	return result, nil
}

func (s *KloterService) CreateKloter(ctx context.Context, orgID string, req *hajjv1.CreateKloterRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("KloterService.CreateKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.CreateKloter", err)
	}
	kloter, err := s.kloterRepository.Create(ctx, op.ID, req.SeasonId, req.Code, req.Embarkation, req.FlightNumber, timestampPtr(req.DepartureDate), req.Capacity)
	if err != nil {
		return nil, serviceError("KloterService.CreateKloter", err)
	}
	s.logActivity(ctx, op.ID, "kloter_created", kloter.ID, fmt.Sprintf("Kloter %s dibuat", kloter.Code))
	return kloterMessage(kloter), nil
}

func (s *KloterService) UpdateKloter(ctx context.Context, orgID string, req *hajjv1.UpdateKloterRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("KloterService.UpdateKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloter", err)
	}
	kloter, err := s.kloterRepository.Update(ctx, op.ID, req.KloterId, req.Code, req.Embarkation, req.FlightNumber, timestampPtr(req.DepartureDate), req.Capacity)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloter", err)
	}
	s.logActivity(ctx, op.ID, "kloter_updated", kloter.ID, fmt.Sprintf("Kloter %s diperbarui", kloter.Code))
	return kloterMessage(kloter), nil
}

func (s *KloterService) UpdateKloterStatus(ctx context.Context, orgID string, req *hajjv1.UpdateKloterStatusRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" || strings.TrimSpace(req.Status) == "" {
		return nil, serviceError("KloterService.UpdateKloterStatus", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloterStatus", err)
	}
	kloter, err := s.updateStatus(ctx, op.ID, req.KloterId, req.Status)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloterStatus", err)
	}
	return kloterMessage(kloter), nil
}

// updateStatus is the operator-independent core, reused by
// TripService.UpdateTripKloterStatus (a Tour Leader assigned to the kloter
// via kloter_staff — already resolved its own operatorID via
// authorizeKloter, no need to re-derive it here).
func (s *KloterService) updateStatus(ctx context.Context, operatorID, kloterID, status string) (*domain.Kloter, error) {
	targetIdx := kloterStatusIndex(status)
	if targetIdx == -1 {
		return nil, apperror.ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := s.kloterRepository.GetForOperatorForUpdateTx(ctx, tx, operatorID, kloterID)
	if err != nil {
		return nil, err
	}
	currentIdx := kloterStatusIndex(current.Status)
	isRollback := current.Status == "CONFIRMED" && status == "DRAFT"
	if !isRollback && targetIdx <= currentIdx {
		return nil, errors.New("status kloter hanya bisa maju")
	}
	kloter, err := s.kloterRepository.UpdateStatusTx(ctx, tx, operatorID, kloterID, status)
	if err != nil {
		return nil, err
	}
	journeyStatus := kloterToJourneyStatus[status]
	if err := s.outboxRepository.EnqueueTx(ctx, tx, operatorID, domain.EventKloterStatusUpdated, kloter.ID, domain.KloterStatusUpdatedPayload{
		KloterID: kloter.ID, KloterCode: kloter.Code, Status: status, JourneyStatus: journeyStatus, UpdatedBy: middleware.UserIDFromCtx(ctx), NotificationBody: kloterStatusPushText[status],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.logActivity(ctx, operatorID, "kloter_status_changed", kloter.ID, fmt.Sprintf("Status kloter %s: %s -> %s", kloter.Code, current.Status, status))
	s.eventBus.Publish(operatorID, "kloter_status", kloter.ID)
	return kloter, nil
}

func (s *KloterService) DeleteKloter(ctx context.Context, orgID string, req *hajjv1.DeleteKloterRequest) (*emptypb.Empty, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.DeleteKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.DeleteKloter", err)
	}
	if err := s.kloterRepository.Delete(ctx, op.ID, req.KloterId); err != nil {
		return nil, serviceError("KloterService.DeleteKloter", err)
	}
	return &emptypb.Empty{}, nil
}

func kloterMessage(k *domain.Kloter) *hajjv1.Kloter {
	msg := &hajjv1.Kloter{Id: k.ID, SeasonId: k.SeasonID, Code: k.Code, Embarkation: k.Embarkation, FlightNumber: k.FlightNumber, Capacity: k.Capacity, PilgrimCount: k.PilgrimCount, Status: k.Status, Notes: k.Notes}
	if k.DepartureDate != nil {
		msg.DepartureDate = timestamppb.New(*k.DepartureDate)
	}
	return msg
}

// GetManifest builds the list that leaves the office.
//
// The readiness figures are computed here rather than in SQL so that "what is
// missing" and "how many are ready" come from one definition of required — the
// same one the row itself reports.
func (s *KloterService) GetManifest(ctx context.Context, orgID string, req *hajjv1.GetKloterManifestRequest) (*hajjv1.GetKloterManifestResponse, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.GetManifest", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.GetManifest", err)
	}
	manifest, err := s.kloterRepository.Manifest(ctx, op.ID, req.KloterId)
	if err != nil {
		return nil, serviceError("KloterService.GetManifest", err)
	}

	response := &hajjv1.GetKloterManifestResponse{
		KloterId: manifest.KloterID, KloterCode: manifest.Code,
		FlightNumber: manifest.FlightNumber, Embarkation: manifest.Embarkation,
		Capacity: manifest.Capacity, MissingByDocument: map[string]int32{},
	}
	if manifest.DepartureDate != nil {
		response.DepartureDate = timestamppb.New(*manifest.DepartureDate)
	}
	for _, row := range manifest.Rows {
		missing := row.MissingDocuments()
		message := &hajjv1.ManifestRow{
			PilgrimId: row.PilgrimID, FullName: row.FullName, PassportNumber: row.PassportNumber,
			Gender: row.Gender, GroupName: row.GroupName, RoomLabel: row.RoomLabel, Phone: row.Phone,
			HasPassport: row.HasPassport, HasVaccine: row.HasVaccine, HasPhoto: row.HasPhoto,
			HasKtp: row.HasKTP, HasKk: row.HasKK, HasMahramProof: row.HasMahramProof,
			MahramProofRequired: row.MahramProofRequired,
			DocumentsComplete:   len(missing) == 0,
			MissingDocuments:    missing,
		}
		if row.DateOfBirth != nil {
			message.DateOfBirth = timestamppb.New(*row.DateOfBirth)
		}
		if len(missing) == 0 {
			response.ReadyCount++
		}
		for _, document := range missing {
			response.MissingByDocument[document]++
		}
		response.Rows = append(response.Rows, message)
	}
	return response, nil
}

// GetRoomlist is the sheet the hotel receives.
func (s *KloterService) GetRoomlist(ctx context.Context, orgID string, req *hajjv1.GetKloterRoomlistRequest) (*hajjv1.GetKloterRoomlistResponse, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.GetRoomlist", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.GetRoomlist", err)
	}
	list, err := s.kloterRepository.Roomlist(ctx, op.ID, req.KloterId)
	if err != nil {
		return nil, serviceError("KloterService.GetRoomlist", err)
	}

	response := &hajjv1.GetKloterRoomlistResponse{
		KloterCode: list.KloterCode, TotalPilgrims: list.Total,
	}
	for _, hotel := range list.Hotels {
		message := &hajjv1.RoomlistHotel{
			HotelId: hotel.HotelID, Name: hotel.Name, City: hotel.City,
		}
		if hotel.CheckInDate != nil {
			message.CheckInDate = timestamppb.New(*hotel.CheckInDate)
		}
		if hotel.CheckOutDate != nil {
			message.CheckOutDate = timestamppb.New(*hotel.CheckOutDate)
		}
		for _, room := range hotel.Rooms {
			// Free beds can go negative only if something wrote past capacity
			// behind the allocation rules; clamping it would hide exactly that.
			free := room.Capacity - int32(len(room.Occupants))
			roomMessage := &hajjv1.RoomlistRoom{
				RoomId: room.RoomID, RoomNumber: room.RoomNumber, RoomType: room.RoomType,
				DesignatedGender: room.DesignatedGender, Capacity: room.Capacity,
				BedsFree: free, MixedWithoutMahram: room.MixedWithoutMahram(),
			}
			response.BedsFree += free
			present := map[string]bool{}
			for _, occupant := range room.Occupants {
				present[occupant.PilgrimID] = true
			}
			for _, occupant := range room.Occupants {
				mahramHere := occupant.MahramID != "" && present[occupant.MahramID]
				if !mahramHere {
					for _, other := range room.Occupants {
						if other.MahramID == occupant.PilgrimID {
							mahramHere = true
							break
						}
					}
				}
				roomMessage.Occupants = append(roomMessage.Occupants, &hajjv1.RoomlistOccupant{
					PilgrimId: occupant.PilgrimID, FullName: occupant.FullName,
					Gender: occupant.Gender, HasMahram: occupant.MahramID != "",
					MahramInRoom: mahramHere,
				})
			}
			message.Rooms = append(message.Rooms, roomMessage)
		}
		response.Hotels = append(response.Hotels, message)
	}
	for _, occupant := range list.Unassigned {
		response.Unassigned = append(response.Unassigned, &hajjv1.RoomlistOccupant{
			PilgrimId: occupant.PilgrimID, FullName: occupant.FullName,
			Gender: occupant.Gender, HasMahram: occupant.MahramID != "",
		})
	}
	return response, nil
}

func itinerarySegmentMessage(seg repository.ItinerarySegment) *hajjv1.ItinerarySegment {
	msg := &hajjv1.ItinerarySegment{
		Id: seg.ID, Position: seg.Position, SegmentType: seg.SegmentType, Notes: seg.Notes,
		MovementId: seg.MovementID, MovementName: seg.MovementName, MovementMode: seg.MovementMode,
		HotelId: seg.HotelID, HotelName: seg.HotelName, HotelCity: seg.HotelCity,
	}
	if seg.MovementScheduledAt != nil {
		msg.MovementScheduledAt = timestamppb.New(*seg.MovementScheduledAt)
	}
	return msg
}

// ListItinerary returns Rangkaian for one kloter.
func (s *KloterService) ListItinerary(ctx context.Context, orgID string, req *hajjv1.ListKloterItineraryRequest) (*hajjv1.ListKloterItineraryResponse, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.ListItinerary", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.ListItinerary", err)
	}
	segments, err := s.kloterRepository.ListItinerarySegments(ctx, op.ID, req.KloterId)
	if err != nil {
		return nil, serviceError("KloterService.ListItinerary", err)
	}
	response := &hajjv1.ListKloterItineraryResponse{}
	for _, seg := range segments {
		response.Segments = append(response.Segments, itinerarySegmentMessage(seg))
	}
	return response, nil
}

// SetItinerary replaces the whole Rangkaian for one kloter — see
// KloterRepository.SetItinerarySegments for why this is a replace, not an
// incremental edit.
func (s *KloterService) SetItinerary(ctx context.Context, orgID string, req *hajjv1.SetKloterItineraryRequest) (*hajjv1.SetKloterItineraryResponse, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.SetItinerary", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.SetItinerary", err)
	}
	inputs := make([]repository.ItinerarySegmentInput, 0, len(req.Segments))
	for _, seg := range req.Segments {
		inputs = append(inputs, repository.ItinerarySegmentInput{
			SegmentType: strings.ToUpper(strings.TrimSpace(seg.SegmentType)),
			MovementID:  strings.TrimSpace(seg.MovementId),
			HotelID:     strings.TrimSpace(seg.HotelId),
			Notes:       strings.TrimSpace(seg.Notes),
		})
	}
	segments, err := s.kloterRepository.SetItinerarySegments(ctx, op.ID, req.KloterId, inputs)
	if err != nil {
		return nil, serviceError("KloterService.SetItinerary", err)
	}
	response := &hajjv1.SetKloterItineraryResponse{}
	for _, seg := range segments {
		response.Segments = append(response.Segments, itinerarySegmentMessage(seg))
	}
	return response, nil
}

func rundownItemMessage(item repository.RundownItem) *hajjv1.RundownItem {
	return &hajjv1.RundownItem{
		Id: item.ID, DayNumber: item.DayNumber, Position: item.Position,
		TimeLabel: item.TimeLabel, Title: item.Title, Location: item.Location,
		Pic: item.PIC, Notes: item.Notes,
	}
}

// ListRundown returns the day-by-day operational schedule for one kloter.
func (s *KloterService) ListRundown(ctx context.Context, orgID string, req *hajjv1.ListKloterRundownRequest) (*hajjv1.ListKloterRundownResponse, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.ListRundown", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.ListRundown", err)
	}
	items, err := s.kloterRepository.ListRundownItems(ctx, op.ID, req.KloterId)
	if err != nil {
		return nil, serviceError("KloterService.ListRundown", err)
	}
	response := &hajjv1.ListKloterRundownResponse{}
	for _, item := range items {
		response.Items = append(response.Items, rundownItemMessage(item))
	}
	return response, nil
}

// SetRundown replaces the whole rundown for one kloter.
func (s *KloterService) SetRundown(ctx context.Context, orgID string, req *hajjv1.SetKloterRundownRequest) (*hajjv1.SetKloterRundownResponse, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.SetRundown", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.SetRundown", err)
	}
	inputs := make([]repository.RundownItem, 0, len(req.Items))
	for _, item := range req.Items {
		inputs = append(inputs, repository.RundownItem{
			DayNumber: item.DayNumber, TimeLabel: strings.TrimSpace(item.TimeLabel),
			Title: strings.TrimSpace(item.Title), Location: strings.TrimSpace(item.Location),
			PIC: strings.TrimSpace(item.Pic), Notes: strings.TrimSpace(item.Notes),
		})
	}
	items, err := s.kloterRepository.SetRundownItems(ctx, op.ID, req.KloterId, inputs)
	if err != nil {
		return nil, serviceError("KloterService.SetRundown", err)
	}
	response := &hajjv1.SetKloterRundownResponse{}
	for _, item := range items {
		response.Items = append(response.Items, rundownItemMessage(item))
	}
	return response, nil
}
