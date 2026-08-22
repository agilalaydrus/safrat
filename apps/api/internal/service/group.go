package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GroupService struct {
	operatorRepository *repository.OperatorRepository
	groupRepository    *repository.GroupRepository
	auditRepository    *repository.AuditRepository
	agentRepository    *repository.AgentRepository
	journeyService     *JourneyService
}

func NewGroupService(operators *repository.OperatorRepository, groups *repository.GroupRepository, audit *repository.AuditRepository, agents *repository.AgentRepository, journey *JourneyService) *GroupService {
	return &GroupService{operatorRepository: operators, groupRepository: groups, auditRepository: audit, agentRepository: agents, journeyService: journey}
}

// cityToJourneyStatus cascades a Muttawwif's location update to every
// pilgrim in the group — only cities with a direct 1:1 journey-status
// equivalent cascade (INDONESIA/TRANSIT/DEPARTED are ambiguous — could be
// outbound or return — so they don't).
var cityToJourneyStatus = map[string]string{
	"MADINAH":    "IN_MADINAH",
	"MAKKAH":     "IN_MAKKAH",
	"ARAFAH":     "IN_ARAFAH",
	"MUZDALIFAH": "IN_MUZDALIFAH",
	"MINA":       "IN_MINA",
}

func (s *GroupService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "group", entityID, message)
}

func (s *GroupService) ListGroups(ctx context.Context, orgID string, req *hajjv1.ListGroupsRequest) (*hajjv1.ListGroupsResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("GroupService.ListGroups", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupService.ListGroups", err)
	}
	groups, err := s.groupRepository.ListForOperator(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("GroupService.ListGroups", err)
	}
	result := &hajjv1.ListGroupsResponse{Groups: make([]*hajjv1.Group, 0, len(groups))}
	for _, group := range groups {
		result.Groups = append(result.Groups, groupMessage(group))
	}
	return result, nil
}

func (s *GroupService) CreateGroup(ctx context.Context, orgID string, req *hajjv1.CreateGroupRequest) (*hajjv1.Group, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" || strings.TrimSpace(req.Name) == "" || req.Capacity < 1 {
		return nil, serviceError("GroupService.CreateGroup", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupService.CreateGroup", err)
	}
	group, err := s.groupRepository.Create(ctx, op.ID, req.SeasonId, req.Name, req.Capacity)
	if err != nil {
		return nil, serviceError("GroupService.CreateGroup", err)
	}
	s.logActivity(ctx, op.ID, "group_created", group.ID, fmt.Sprintf("Rombongan %s dibuat", group.Name))
	return groupMessage(group), nil
}

func (s *GroupService) UpdateGroup(ctx context.Context, orgID string, req *hajjv1.UpdateGroupRequest) (*hajjv1.Group, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" || strings.TrimSpace(req.Name) == "" || req.Capacity < 1 {
		return nil, serviceError("GroupService.UpdateGroup", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupService.UpdateGroup", err)
	}
	group, err := s.groupRepository.Update(ctx, op.ID, req.GroupId, req.Name, req.Capacity, req.LeaderId)
	if err != nil {
		return nil, serviceError("GroupService.UpdateGroup", err)
	}
	// Leaders automatically become agents — they can sell any of the
	// operator's products via the existing referral/commission system.
	// Best-effort: a leader assignment must still succeed even if this
	// fails (e.g. the transient state right after ListOperatorMembers
	// but before the "user" row is fully visible in this transaction).
	if strings.TrimSpace(req.LeaderId) != "" {
		if err := s.agentRepository.EnsureAgentForLeader(ctx, op.ID, req.LeaderId); err != nil {
			sentry.CaptureException(fmt.Errorf("GroupService.UpdateGroup: ensure agent for leader: %w", err))
		}
	}
	s.logActivity(ctx, op.ID, "group_updated", group.ID, fmt.Sprintf("Rombongan %s diperbarui", group.Name))
	return groupMessage(group), nil
}

func (s *GroupService) DeleteGroup(ctx context.Context, orgID string, req *hajjv1.DeleteGroupRequest) (*emptypb.Empty, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" {
		return nil, serviceError("GroupService.DeleteGroup", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupService.DeleteGroup", err)
	}
	if err := s.groupRepository.Delete(ctx, op.ID, req.GroupId); err != nil {
		return nil, serviceError("GroupService.DeleteGroup", err)
	}
	return &emptypb.Empty{}, nil
}

// ListOperatorMembers powers the leader picker on the admin Groups page —
// note this queries Better Auth's own org membership by orgID directly, not
// by operator.ID like everything else in this service.
func (s *GroupService) ListOperatorMembers(ctx context.Context, orgID string) (*hajjv1.ListOperatorMembersResponse, error) {
	members, err := s.groupRepository.ListOperatorMembers(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupService.ListOperatorMembers", err)
	}
	result := &hajjv1.ListOperatorMembersResponse{Members: make([]*hajjv1.OperatorMember, 0, len(members))}
	for _, member := range members {
		result.Members = append(result.Members, &hajjv1.OperatorMember{UserId: member.UserID, Name: member.Name, Email: member.Email})
	}
	return result, nil
}

// ListMuttawwif is the operator-wide roster (one entry per leader, not per
// group) — contrast with ListGroups/ListForOperator, which lists groups.
func (s *GroupService) ListMuttawwif(ctx context.Context, orgID string) (*hajjv1.ListMuttawwifResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupService.ListMuttawwif", err)
	}
	rows, err := s.groupRepository.ListMuttawwif(ctx, op.ID)
	if err != nil {
		return nil, serviceError("GroupService.ListMuttawwif", err)
	}
	result := &hajjv1.ListMuttawwifResponse{Muttawwif: make([]*hajjv1.Muttawwif, 0, len(rows))}
	for _, row := range rows {
		groups := make([]*hajjv1.LeaderGroup, 0, len(row.Groups))
		for _, g := range row.Groups {
			groups = append(groups, &hajjv1.LeaderGroup{Id: g.ID, Name: g.Name, Capacity: g.Capacity, PilgrimCount: g.PilgrimCount, SeasonId: g.SeasonID})
		}
		result.Muttawwif = append(result.Muttawwif, &hajjv1.Muttawwif{UserId: row.UserID, Name: row.Name, Email: row.Email, Phone: row.Phone, Groups: groups})
	}
	return result, nil
}

// GetGroupRoster lets an admin/coordinator inspect any of the operator's
// groups' member lists from the Groups dashboard — operator-scoped like the
// rest of GroupService, not leader-scoped like GroupLeaderService.GetGroupRoster.
func (s *GroupService) GetGroupRoster(ctx context.Context, orgID string, req *hajjv1.GetGroupRosterRequest) (*hajjv1.GetGroupRosterResponse, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" {
		return nil, serviceError("GroupService.GetGroupRoster", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupService.GetGroupRoster", err)
	}
	pilgrims, err := s.groupRepository.GetRoster(ctx, op.ID, req.GroupId)
	if err != nil {
		return nil, serviceError("GroupService.GetGroupRoster", err)
	}
	result := &hajjv1.GetGroupRosterResponse{Pilgrims: make([]*hajjv1.Pilgrim, 0, len(pilgrims))}
	for _, p := range pilgrims {
		result.Pilgrims = append(result.Pilgrims, pilgrimMessage(p))
	}
	return result, nil
}

// ListGroupsByKloter powers the Kloter Detail "Rombongan" tab.
func (s *GroupService) ListGroupsByKloter(ctx context.Context, orgID string, req *hajjv1.ListGroupsByKloterRequest) (*hajjv1.ListGroupsResponse, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("GroupService.ListGroupsByKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupService.ListGroupsByKloter", err)
	}
	groups, err := s.groupRepository.ListByKloter(ctx, op.ID, req.KloterId)
	if err != nil {
		return nil, serviceError("GroupService.ListGroupsByKloter", err)
	}
	result := &hajjv1.ListGroupsResponse{Groups: make([]*hajjv1.Group, 0, len(groups))}
	for _, group := range groups {
		result.Groups = append(result.Groups, groupMessage(group))
	}
	return result, nil
}

// UpdateGroupCity is the Muttawwif's one-tap "we're at X now" action.
func (s *GroupService) UpdateGroupCity(ctx context.Context, orgID string, req *hajjv1.UpdateGroupCityRequest) (*hajjv1.Group, error) {
	if req == nil || strings.TrimSpace(req.GroupId) == "" || strings.TrimSpace(req.City) == "" {
		return nil, serviceError("GroupService.UpdateGroupCity", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("GroupService.UpdateGroupCity", err)
	}
	group, err := s.groupRepository.UpdateCity(ctx, op.ID, req.GroupId, req.City, req.Location, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("GroupService.UpdateGroupCity", err)
	}
	s.logActivity(ctx, op.ID, "group_city_updated", group.ID, fmt.Sprintf("Rombongan %s kini di %s", group.Name, req.City))
	// Cascade: best-effort, never rolls back the location update itself —
	// see kloterToJourneyStatus for the same pattern.
	if journeyStatus, ok := cityToJourneyStatus[req.City]; ok && s.journeyService != nil {
		if _, err := s.journeyService.BulkUpdateForGroup(ctx, op.ID, group.ID, journeyStatus, req.Notes); err != nil {
			sentry.CaptureException(fmt.Errorf("GroupService.UpdateGroupCity: journey cascade: %w", err))
		}
	}
	return groupMessage(group), nil
}

func groupMessage(group *domain.Group) *hajjv1.Group {
	msg := &hajjv1.Group{Id: group.ID, SeasonId: group.SeasonID, Name: group.Name, Capacity: group.Capacity, PilgrimCount: group.PilgrimCount, LeaderId: group.LeaderID, LeaderName: group.LeaderName, KloterId: group.KloterID, CurrentCity: group.CurrentCity, Status: group.Status}
	if group.LastUpdate != nil {
		msg.LastUpdate = timestamppb.New(*group.LastUpdate)
	}
	return msg
}
