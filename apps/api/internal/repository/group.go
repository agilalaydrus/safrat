package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

type GroupRepository struct{ queries *db.Queries }

func NewGroupRepository(queries *db.Queries) *GroupRepository {
	return &GroupRepository{queries: queries}
}

func (r *GroupRepository) ListForOperator(ctx context.Context, operatorID, seasonID string) ([]*domain.Group, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListGroupsForOperator(ctx, db.ListGroupsForOperatorParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Group, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.Group{ID: uuid.UUID(row.ID.Bytes).String(), SeasonID: uuid.UUID(row.SeasonID.Bytes).String(), OperatorID: uuid.UUID(row.OperatorID.Bytes).String(), Name: row.Name, Capacity: row.Capacity, PilgrimCount: row.PilgrimCount, LeaderID: row.LeaderID.String, LeaderName: row.LeaderName.String})
	}
	return result, nil
}

func (r *GroupRepository) Create(ctx context.Context, operatorID, seasonID, name string, capacity int32) (*domain.Group, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	group, err := r.queries.CreateGroup(ctx, db.CreateGroupParams{OperatorID: opUUID, SeasonID: seasonUUID, Name: name, Capacity: capacity})
	if err != nil {
		return nil, err
	}
	return toGroup(group), nil
}

func (r *GroupRepository) Update(ctx context.Context, operatorID, groupID, name string, capacity int32, leaderID string) (*domain.Group, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, err
	}
	group, err := r.queries.UpdateGroup(ctx, db.UpdateGroupParams{ID: groupUUID, OperatorID: opUUID, Name: name, Capacity: capacity, Column5: leaderID})
	if err != nil {
		return nil, err
	}
	return toGroup(group), nil
}

func (r *GroupRepository) Delete(ctx context.Context, operatorID, groupID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return err
	}
	if err := r.queries.UnassignGroupPilgrims(ctx, db.UnassignGroupPilgrimsParams{GroupID: groupUUID, OperatorID: opUUID}); err != nil {
		return err
	}
	return r.queries.DeleteGroup(ctx, db.DeleteGroupParams{ID: groupUUID, OperatorID: opUUID})
}

func (r *GroupRepository) ListOperatorMembers(ctx context.Context, betterAuthOrgID string) ([]*domain.OperatorMember, error) {
	rows, err := r.queries.ListOperatorMembers(ctx, betterAuthOrgID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.OperatorMember, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.OperatorMember{UserID: row.ID, Name: row.Name, Email: row.Email})
	}
	return result, nil
}

func toGroup(group db.Group) *domain.Group {
	return &domain.Group{ID: uuid.UUID(group.ID.Bytes).String(), SeasonID: uuid.UUID(group.SeasonID.Bytes).String(), OperatorID: uuid.UUID(group.OperatorID.Bytes).String(), Name: group.Name, Capacity: group.Capacity, LeaderID: group.LeaderID.String}
}
