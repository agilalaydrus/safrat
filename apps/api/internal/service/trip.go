package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TripService backs the Tour Leader portal's "Perjalanan Saya" tab. Every
// method resolves the caller's own kloter_staff assignment and refuses to
// act on any kloter_id it doesn't own — see TripRepository's doc comment.
type TripService struct {
	operatorRepository    *repository.OperatorRepository
	tripRepository        *repository.TripRepository
	pilgrimRepository     *repository.PilgrimRepository
	sosRepository         *repository.SOSRepository
	groupLeaderRepository *repository.GroupLeaderRepository
	transportRepository   *repository.TransportRepository
	kloterService         *KloterService
}

func NewTripService(operators *repository.OperatorRepository, trips *repository.TripRepository, pilgrims *repository.PilgrimRepository, sos *repository.SOSRepository, groupLeaders *repository.GroupLeaderRepository, transport *repository.TransportRepository, kloters *KloterService) *TripService {
	return &TripService{operatorRepository: operators, tripRepository: trips, pilgrimRepository: pilgrims, sosRepository: sos, groupLeaderRepository: groupLeaders, transportRepository: transport, kloterService: kloters}
}

// UpdateTripKloterStatus is the Tour Leader's own trigger for the kloter
// lifecycle — same transitions/cascade as KloterService.UpdateKloterStatus,
// reusing its unexported core (ownership already confirmed by
// authorizeKloter, unlike the admin-dashboard RPC which only checks org
// membership).
func (s *TripService) UpdateTripKloterStatus(ctx context.Context, orgID string, req *hajjv1.UpdateTripKloterStatusRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.Status) == "" {
		return nil, serviceError("TripService.UpdateTripKloterStatus", apperror.ErrValidation)
	}
	op, err := s.authorizeKloter(ctx, orgID, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.UpdateTripKloterStatus", err)
	}
	kloter, err := s.kloterService.updateStatus(ctx, op, req.KloterId, req.Status)
	if err != nil {
		return nil, serviceError("TripService.UpdateTripKloterStatus", err)
	}
	return kloterMessage(kloter), nil
}

// authorizeKloter resolves the operator and confirms the caller is actually
// assigned to kloterID before any other trip data is touched.
func (s *TripService) authorizeKloter(ctx context.Context, orgID, kloterID string) (string, error) {
	if !isUUID(kloterID) {
		return "", apperror.ErrValidation
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return "", err
	}
	if err := s.tripRepository.EnsureStaffAssignedToKloter(ctx, op.ID, kloterID, middleware.UserIDFromCtx(ctx)); err != nil {
		return "", apperror.ErrForbidden
	}
	return op.ID, nil
}

func (s *TripService) GetTripRoster(ctx context.Context, orgID string, req *hajjv1.GetTripRosterRequest) (*hajjv1.GetTripRosterResponse, error) {
	if req == nil {
		return nil, serviceError("TripService.GetTripRoster", apperror.ErrValidation)
	}
	op, err := s.authorizeKloter(ctx, orgID, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.GetTripRoster", err)
	}
	pilgrims, err := s.tripRepository.ListRoster(ctx, op, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.GetTripRoster", err)
	}
	result := &hajjv1.GetTripRosterResponse{Pilgrims: make([]*hajjv1.Pilgrim, 0, len(pilgrims))}
	for _, p := range pilgrims {
		result.Pilgrims = append(result.Pilgrims, pilgrimMessage(p))
	}
	return result, nil
}

func (s *TripService) SetTripHotelCheckIn(ctx context.Context, orgID string, req *hajjv1.SetTripHotelCheckInRequest) (*hajjv1.Pilgrim, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("TripService.SetTripHotelCheckIn", apperror.ErrValidation)
	}
	op, err := s.authorizeKloter(ctx, orgID, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.SetTripHotelCheckIn", err)
	}
	pilgrim, err := s.pilgrimRepository.Get(ctx, op, req.PilgrimId)
	if err != nil {
		return nil, serviceError("TripService.SetTripHotelCheckIn", err)
	}
	if pilgrim.KloterID != req.KloterId {
		return nil, serviceError("TripService.SetTripHotelCheckIn", apperror.ErrForbidden)
	}
	updated, err := s.pilgrimRepository.CheckInHotel(ctx, op, req.PilgrimId, req.CheckedIn)
	if err != nil {
		return nil, serviceError("TripService.SetTripHotelCheckIn", err)
	}
	return pilgrimMessage(updated), nil
}

func (s *TripService) ListTripMovements(ctx context.Context, orgID string, req *hajjv1.ListTripMovementsRequest) (*hajjv1.ListTripMovementsResponse, error) {
	if req == nil {
		return nil, serviceError("TripService.ListTripMovements", apperror.ErrValidation)
	}
	op, err := s.authorizeKloter(ctx, orgID, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.ListTripMovements", err)
	}
	movements, err := s.tripRepository.ListMovements(ctx, op, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.ListTripMovements", err)
	}
	result := &hajjv1.ListTripMovementsResponse{Movements: make([]*hajjv1.Movement, 0, len(movements))}
	for _, m := range movements {
		result.Movements = append(result.Movements, movementMessage(m))
	}
	return result, nil
}

// authorizeMovementInKloter additionally confirms movementID actually
// belongs to a kloter the caller already owns — a movement id alone must
// never be trusted from the request.
func (s *TripService) authorizeMovementInKloter(ctx context.Context, op, kloterID, movementID string) error {
	movement, err := s.transportRepository.GetMovement(ctx, op, movementID)
	if err != nil {
		return err
	}
	if movement.KloterID != kloterID {
		return apperror.ErrForbidden
	}
	return nil
}

func (s *TripService) ListTripCheckIns(ctx context.Context, orgID string, req *hajjv1.ListTripCheckInsRequest) (*hajjv1.ListTripCheckInsResponse, error) {
	if req == nil || !isUUID(req.MovementId) {
		return nil, serviceError("TripService.ListTripCheckIns", apperror.ErrValidation)
	}
	op, err := s.authorizeKloter(ctx, orgID, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.ListTripCheckIns", err)
	}
	if err := s.authorizeMovementInKloter(ctx, op, req.KloterId, req.MovementId); err != nil {
		return nil, serviceError("TripService.ListTripCheckIns", err)
	}
	checkIns, err := s.groupLeaderRepository.ListCheckIns(ctx, op, req.MovementId)
	if err != nil {
		return nil, serviceError("TripService.ListTripCheckIns", err)
	}
	result := &hajjv1.ListTripCheckInsResponse{CheckIns: make([]*hajjv1.CheckIn, 0, len(checkIns))}
	for _, c := range checkIns {
		result.CheckIns = append(result.CheckIns, &hajjv1.CheckIn{Id: c.ID, MovementId: c.MovementID, PilgrimId: c.PilgrimID, Type: c.Type, CreatedAt: timestamppb.New(c.CreatedAt)})
	}
	return result, nil
}

func (s *TripService) CreateTripCheckIn(ctx context.Context, orgID string, req *hajjv1.CreateTripCheckInRequest) (*hajjv1.CheckIn, error) {
	if req == nil || !isUUID(req.MovementId) || !isUUID(req.PilgrimId) || (req.Type != "DEPARTURE" && req.Type != "ARRIVAL") {
		return nil, serviceError("TripService.CreateTripCheckIn", apperror.ErrValidation)
	}
	op, err := s.authorizeKloter(ctx, orgID, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.CreateTripCheckIn", err)
	}
	if err := s.authorizeMovementInKloter(ctx, op, req.KloterId, req.MovementId); err != nil {
		return nil, serviceError("TripService.CreateTripCheckIn", err)
	}
	pilgrim, err := s.pilgrimRepository.Get(ctx, op, req.PilgrimId)
	if err != nil {
		return nil, serviceError("TripService.CreateTripCheckIn", err)
	}
	if pilgrim.KloterID != req.KloterId {
		return nil, serviceError("TripService.CreateTripCheckIn", apperror.ErrForbidden)
	}
	checkIn, err := s.groupLeaderRepository.CreateCheckIn(ctx, op, req.MovementId, req.PilgrimId, req.Type, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("TripService.CreateTripCheckIn", apperror.ErrAlreadyExists)
	}
	return &hajjv1.CheckIn{Id: checkIn.ID, MovementId: checkIn.MovementID, PilgrimId: checkIn.PilgrimID, Type: checkIn.Type, CreatedAt: timestamppb.New(checkIn.CreatedAt)}, nil
}

func (s *TripService) ListTripSOSAlerts(ctx context.Context, orgID string, req *hajjv1.ListTripSOSAlertsRequest) (*hajjv1.ListTripSOSAlertsResponse, error) {
	if req == nil {
		return nil, serviceError("TripService.ListTripSOSAlerts", apperror.ErrValidation)
	}
	op, err := s.authorizeKloter(ctx, orgID, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.ListTripSOSAlerts", err)
	}
	alerts, err := s.tripRepository.ListActiveSOSAlerts(ctx, op, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.ListTripSOSAlerts", err)
	}
	result := &hajjv1.ListTripSOSAlertsResponse{Alerts: make([]*hajjv1.SOSAlert, 0, len(alerts))}
	for _, a := range alerts {
		result.Alerts = append(result.Alerts, sosAlertMessage(a))
	}
	return result, nil
}

// authorizeAlertInKloter confirms the alert belongs to a pilgrim in a
// kloter the caller owns, by checking it's present in the already-scoped
// active-alerts list — no separate query needed since that list is the
// authoritative "what this kloter can see" set.
func (s *TripService) authorizeAlertInKloter(ctx context.Context, op, kloterID, alertID string) error {
	alerts, err := s.tripRepository.ListActiveSOSAlerts(ctx, op, kloterID)
	if err != nil {
		return err
	}
	for _, a := range alerts {
		if a.ID == alertID {
			return nil
		}
	}
	return apperror.ErrForbidden
}

func (s *TripService) AcknowledgeTripSOSAlert(ctx context.Context, orgID string, req *hajjv1.AcknowledgeTripSOSAlertRequest) (*hajjv1.SOSAlert, error) {
	if req == nil || !isUUID(req.SosAlertId) {
		return nil, serviceError("TripService.AcknowledgeTripSOSAlert", apperror.ErrValidation)
	}
	op, err := s.authorizeKloter(ctx, orgID, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.AcknowledgeTripSOSAlert", err)
	}
	if err := s.authorizeAlertInKloter(ctx, op, req.KloterId, req.SosAlertId); err != nil {
		return nil, serviceError("TripService.AcknowledgeTripSOSAlert", err)
	}
	alert, err := s.sosRepository.Acknowledge(ctx, op, req.SosAlertId, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("TripService.AcknowledgeTripSOSAlert", err)
	}
	return sosAlertMessage(alert), nil
}

func (s *TripService) ResolveTripSOSAlert(ctx context.Context, orgID string, req *hajjv1.ResolveTripSOSAlertRequest) (*hajjv1.SOSAlert, error) {
	if req == nil || !isUUID(req.SosAlertId) {
		return nil, serviceError("TripService.ResolveTripSOSAlert", apperror.ErrValidation)
	}
	op, err := s.authorizeKloter(ctx, orgID, req.KloterId)
	if err != nil {
		return nil, serviceError("TripService.ResolveTripSOSAlert", err)
	}
	if err := s.authorizeAlertInKloter(ctx, op, req.KloterId, req.SosAlertId); err != nil {
		return nil, serviceError("TripService.ResolveTripSOSAlert", err)
	}
	alert, err := s.sosRepository.Resolve(ctx, op, req.SosAlertId, middleware.UserIDFromCtx(ctx), strings.TrimSpace(req.Notes))
	if err != nil {
		return nil, serviceError("TripService.ResolveTripSOSAlert", err)
	}
	return sosAlertMessage(alert), nil
}
