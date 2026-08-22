package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GroupLeaderService struct {
	operatorRepository    *repository.OperatorRepository
	groupLeaderRepository *repository.GroupLeaderRepository
	sosRepository         *repository.SOSRepository
	pilgrimRepository     *repository.PilgrimRepository
	groupRepository       *repository.GroupRepository
	journeyService        *JourneyService
	pushNotifier          PushNotifier
}

func NewGroupLeaderService(operators *repository.OperatorRepository, groupLeaders *repository.GroupLeaderRepository, sos *repository.SOSRepository, pilgrims *repository.PilgrimRepository, groups *repository.GroupRepository, journey *JourneyService, push PushNotifier) *GroupLeaderService {
	return &GroupLeaderService{operatorRepository: operators, groupLeaderRepository: groupLeaders, sosRepository: sos, pilgrimRepository: pilgrims, groupRepository: groups, journeyService: journey, pushNotifier: push}
}

// ListMySOSAlerts scopes the coordinator-wide SOS surface down to only
// pilgrims in groups this leader leads.
func (s *GroupLeaderService) ListMySOSAlerts(ctx context.Context, orgID string) (*hajjv1.ListSOSAlertsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupLeaderService.ListMySOSAlerts", err)
	}
	alerts, err := s.sosRepository.ListActiveForLeader(ctx, op.ID, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("GroupLeaderService.ListMySOSAlerts", err)
	}
	result := &hajjv1.ListSOSAlertsResponse{Alerts: make([]*hajjv1.SOSAlert, 0, len(alerts))}
	for _, alert := range alerts {
		result.Alerts = append(result.Alerts, sosAlertMessage(alert))
	}
	return result, nil
}

func (s *GroupLeaderService) ListMyGroups(ctx context.Context, orgID string) (*hajjv1.ListMyGroupsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupLeaderService.ListMyGroups", err)
	}
	groups, err := s.groupLeaderRepository.ListMyGroups(ctx, op.ID, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("GroupLeaderService.ListMyGroups", err)
	}
	result := &hajjv1.ListMyGroupsResponse{Groups: make([]*hajjv1.LeaderGroup, 0, len(groups))}
	for _, group := range groups {
		result.Groups = append(result.Groups, leaderGroupMessage(group))
	}
	return result, nil
}

// UpdateMyGroupCity is the Muttawwif's Tab Lokasi one-tap action.
func (s *GroupLeaderService) UpdateMyGroupCity(ctx context.Context, orgID string, req *hajjv1.UpdateMyGroupCityRequest) (*hajjv1.LeaderGroup, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" || strings.TrimSpace(req.City) == "" {
		return nil, serviceError("GroupLeaderService.UpdateMyGroupCity", apperror.ErrValidation)
	}
	op, err := s.authorizeGroup(ctx, orgID, req.GroupId)
	if err != nil {
		return nil, err
	}
	group, err := s.groupRepository.UpdateCity(ctx, op, req.GroupId, req.City, req.Activity, req.Location, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("GroupLeaderService.UpdateMyGroupCity", err)
	}
	if journeyStatus, ok := cityToJourneyStatus[req.City]; ok && s.journeyService != nil {
		if _, err := s.journeyService.BulkUpdateForGroup(ctx, op, group.ID, journeyStatus, req.Notes); err != nil {
			sentry.CaptureException(fmt.Errorf("GroupLeaderService.UpdateMyGroupCity: journey cascade: %w", err))
		}
	}
	if s.pushNotifier != nil {
		s.pushNotifier.NotifyGroupPilgrims(ctx, op, group.ID, "Tawafiq Hub", groupCityPushBody(req.City))
	}
	return &hajjv1.LeaderGroup{Id: group.ID, Name: group.Name, Capacity: group.Capacity, SeasonId: group.SeasonID, CurrentCity: group.CurrentCity, LastUpdate: timestampOrNil(group.LastUpdate), CurrentActivity: group.CurrentActivity}, nil
}

func leaderGroupMessage(group *domain.LeaderGroup) *hajjv1.LeaderGroup {
	return &hajjv1.LeaderGroup{Id: group.ID, Name: group.Name, Capacity: group.Capacity, PilgrimCount: group.PilgrimCount, SeasonId: group.SeasonID, CurrentCity: group.CurrentCity, LastUpdate: timestampOrNil(group.LastUpdate), CurrentActivity: group.CurrentActivity}
}

func timestampOrNil(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func (s *GroupLeaderService) GetGroupRoster(ctx context.Context, orgID string, req *hajjv1.GetGroupRosterRequest) (*hajjv1.GetGroupRosterResponse, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" {
		return nil, serviceError("GroupLeaderService.GetGroupRoster", apperror.ErrValidation)
	}
	op, err := s.authorizeGroup(ctx, orgID, req.GroupId)
	if err != nil {
		return nil, err
	}
	pilgrims, err := s.groupLeaderRepository.GetRoster(ctx, op, req.GroupId)
	if err != nil {
		return nil, serviceError("GroupLeaderService.GetGroupRoster", err)
	}
	result := &hajjv1.GetGroupRosterResponse{Pilgrims: make([]*hajjv1.Pilgrim, 0, len(pilgrims))}
	for _, p := range pilgrims {
		result.Pilgrims = append(result.Pilgrims, pilgrimMessage(p))
	}
	return result, nil
}

func (s *GroupLeaderService) ListCheckIns(ctx context.Context, orgID string, req *hajjv1.ListCheckInsRequest) (*hajjv1.ListCheckInsResponse, error) {
	if req == nil || strings.TrimSpace(req.MovementId) == "" {
		return nil, serviceError("GroupLeaderService.ListCheckIns", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupLeaderService.ListCheckIns", err)
	}
	movement, err := s.groupLeaderRepository.GetMovementKloter(ctx, op.ID, req.MovementId)
	if err != nil {
		return nil, serviceError("GroupLeaderService.ListCheckIns", apperror.ErrNotFound)
	}
	if err := s.groupLeaderRepository.EnsureLeaderHasPilgrimInKloter(ctx, op.ID, movement, middleware.UserIDFromCtx(ctx)); err != nil {
		return nil, serviceError("GroupLeaderService.ListCheckIns", apperror.ErrForbidden)
	}
	checkIns, err := s.groupLeaderRepository.ListCheckIns(ctx, op.ID, req.MovementId)
	if err != nil {
		return nil, serviceError("GroupLeaderService.ListCheckIns", err)
	}
	result := &hajjv1.ListCheckInsResponse{CheckIns: make([]*hajjv1.CheckIn, 0, len(checkIns))}
	for _, c := range checkIns {
		result.CheckIns = append(result.CheckIns, &hajjv1.CheckIn{Id: c.ID, MovementId: c.MovementID, PilgrimId: c.PilgrimID, Type: c.Type, CreatedAt: timestamppb.New(c.CreatedAt)})
	}
	return result, nil
}

func (s *GroupLeaderService) CreateCheckIn(ctx context.Context, orgID string, req *hajjv1.CreateCheckInRequest) (*hajjv1.CheckIn, error) {
	if req == nil || strings.TrimSpace(req.MovementId) == "" || strings.TrimSpace(req.PilgrimId) == "" || (req.Type != "DEPARTURE" && req.Type != "ARRIVAL") {
		return nil, serviceError("GroupLeaderService.CreateCheckIn", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupLeaderService.CreateCheckIn", err)
	}
	if err := s.groupLeaderRepository.EnsureLeaderOwnsPilgrim(ctx, op.ID, req.PilgrimId, middleware.UserIDFromCtx(ctx)); err != nil {
		return nil, serviceError("GroupLeaderService.CreateCheckIn", apperror.ErrForbidden)
	}
	checkIn, err := s.groupLeaderRepository.CreateCheckIn(ctx, op.ID, req.MovementId, req.PilgrimId, req.Type, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("GroupLeaderService.CreateCheckIn", apperror.ErrAlreadyExists)
	}
	return &hajjv1.CheckIn{Id: checkIn.ID, MovementId: checkIn.MovementID, PilgrimId: checkIn.PilgrimID, Type: checkIn.Type, CreatedAt: timestamppb.New(checkIn.CreatedAt)}, nil
}

// CheckInGroupPilgrimHotel is the leader-scoped counterpart to
// PilgrimService.CheckInPilgrimHotel — only ever touches a pilgrim in one
// of this leader's own groups.
func (s *GroupLeaderService) CheckInGroupPilgrimHotel(ctx context.Context, orgID string, req *hajjv1.CheckInGroupPilgrimHotelRequest) (*hajjv1.Pilgrim, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("GroupLeaderService.CheckInGroupPilgrimHotel", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupLeaderService.CheckInGroupPilgrimHotel", err)
	}
	if err := s.groupLeaderRepository.EnsureLeaderOwnsPilgrim(ctx, op.ID, req.PilgrimId, middleware.UserIDFromCtx(ctx)); err != nil {
		return nil, serviceError("GroupLeaderService.CheckInGroupPilgrimHotel", apperror.ErrForbidden)
	}
	pilgrim, err := s.pilgrimRepository.CheckInHotel(ctx, op.ID, req.PilgrimId, req.CheckedIn)
	if err != nil {
		return nil, serviceError("GroupLeaderService.CheckInGroupPilgrimHotel", err)
	}
	return pilgrimMessage(pilgrim), nil
}

// authorizeAlertOwnedByLeader confirms alertID is currently visible via
// ListMySOSAlerts (i.e. for a pilgrim in one of this leader's own groups)
// before allowing it to be acknowledged/resolved.
func (s *GroupLeaderService) authorizeAlertOwnedByLeader(ctx context.Context, op, alertID string) error {
	alerts, err := s.sosRepository.ListActiveForLeader(ctx, op, middleware.UserIDFromCtx(ctx))
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

func (s *GroupLeaderService) AcknowledgeMySOSAlert(ctx context.Context, orgID string, req *hajjv1.AcknowledgeMySOSAlertRequest) (*hajjv1.SOSAlert, error) {
	if req == nil || !isUUID(req.SosAlertId) {
		return nil, serviceError("GroupLeaderService.AcknowledgeMySOSAlert", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupLeaderService.AcknowledgeMySOSAlert", err)
	}
	if err := s.authorizeAlertOwnedByLeader(ctx, op.ID, req.SosAlertId); err != nil {
		return nil, serviceError("GroupLeaderService.AcknowledgeMySOSAlert", err)
	}
	alert, err := s.sosRepository.Acknowledge(ctx, op.ID, req.SosAlertId, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("GroupLeaderService.AcknowledgeMySOSAlert", err)
	}
	return sosAlertMessage(alert), nil
}

func (s *GroupLeaderService) ResolveMySOSAlert(ctx context.Context, orgID string, req *hajjv1.ResolveMySOSAlertRequest) (*hajjv1.SOSAlert, error) {
	if req == nil || !isUUID(req.SosAlertId) {
		return nil, serviceError("GroupLeaderService.ResolveMySOSAlert", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupLeaderService.ResolveMySOSAlert", err)
	}
	if err := s.authorizeAlertOwnedByLeader(ctx, op.ID, req.SosAlertId); err != nil {
		return nil, serviceError("GroupLeaderService.ResolveMySOSAlert", err)
	}
	alert, err := s.sosRepository.Resolve(ctx, op.ID, req.SosAlertId, middleware.UserIDFromCtx(ctx), strings.TrimSpace(req.Notes))
	if err != nil {
		return nil, serviceError("GroupLeaderService.ResolveMySOSAlert", err)
	}
	return sosAlertMessage(alert), nil
}

func (s *GroupLeaderService) authorizeGroup(ctx context.Context, orgID, groupID string) (string, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return "", serviceError("GroupLeaderService", err)
	}
	if err := s.groupLeaderRepository.EnsureLeaderOwnsGroup(ctx, op.ID, groupID, middleware.UserIDFromCtx(ctx)); err != nil {
		return "", serviceError("GroupLeaderService", apperror.ErrForbidden)
	}
	return op.ID, nil
}
