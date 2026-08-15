package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/emptypb"
)

type GroupService struct {
	operatorRepository *repository.OperatorRepository
	groupRepository    *repository.GroupRepository
}

func NewGroupService(operators *repository.OperatorRepository, groups *repository.GroupRepository) *GroupService {
	return &GroupService{operatorRepository: operators, groupRepository: groups}
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

func groupMessage(group *domain.Group) *hajjv1.Group {
	return &hajjv1.Group{Id: group.ID, SeasonId: group.SeasonID, Name: group.Name, Capacity: group.Capacity, PilgrimCount: group.PilgrimCount, LeaderId: group.LeaderID, LeaderName: group.LeaderName}
}
