package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

type GroupLeaderRepository struct{ queries *db.Queries }

func NewGroupLeaderRepository(queries *db.Queries) *GroupLeaderRepository {
	return &GroupLeaderRepository{queries: queries}
}

func (r *GroupLeaderRepository) ListMyGroups(ctx context.Context, operatorID, leaderUserID string) ([]*domain.LeaderGroup, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListGroupsByLeader(ctx, db.ListGroupsByLeaderParams{OperatorID: opUUID, LeaderID: pgText(leaderUserID)})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.LeaderGroup, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.LeaderGroup{ID: uuid.UUID(row.ID.Bytes).String(), Name: row.Name, Capacity: row.Capacity, PilgrimCount: row.PilgrimCount})
	}
	return result, nil
}

// EnsureLeaderOwnsGroup confirms the group belongs to this operator and this
// leader before any roster/check-in action — every GroupLeaderService method
// must call this rather than trusting the group_id in the request.
func (r *GroupLeaderRepository) EnsureLeaderOwnsGroup(ctx context.Context, operatorID, groupID, leaderUserID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return err
	}
	_, err = r.queries.GetGroupForLeader(ctx, db.GetGroupForLeaderParams{ID: groupUUID, OperatorID: opUUID, LeaderID: pgText(leaderUserID)})
	return err
}

func (r *GroupLeaderRepository) GetRoster(ctx context.Context, operatorID, groupID string) ([]*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListGroupRoster(ctx, db.ListGroupRosterParams{GroupID: groupUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Pilgrim, 0, len(rows))
	for _, row := range rows {
		result = append(result, toPilgrim(row))
	}
	return result, nil
}

func (r *GroupLeaderRepository) CreateCheckIn(ctx context.Context, operatorID, movementID, pilgrimID, checkInType, checkedInBy string) (*domain.CheckIn, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	movementUUID, err := pgUUID(movementID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	checkIn, err := r.queries.CreateCheckIn(ctx, db.CreateCheckInParams{OperatorID: opUUID, MovementID: movementUUID, PilgrimID: pilgrimUUID, Type: checkInType, Column5: checkedInBy})
	if err != nil {
		return nil, err
	}
	return toCheckIn(checkIn), nil
}

func (r *GroupLeaderRepository) ListCheckIns(ctx context.Context, operatorID, movementID string) ([]*domain.CheckIn, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	movementUUID, err := pgUUID(movementID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListCheckInsByMovement(ctx, db.ListCheckInsByMovementParams{MovementID: movementUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.CheckIn, 0, len(rows))
	for _, row := range rows {
		result = append(result, toCheckIn(row))
	}
	return result, nil
}

func toCheckIn(value db.CheckIn) *domain.CheckIn {
	return &domain.CheckIn{ID: uuid.UUID(value.ID.Bytes).String(), OperatorID: uuid.UUID(value.OperatorID.Bytes).String(), MovementID: uuid.UUID(value.MovementID.Bytes).String(), PilgrimID: uuid.UUID(value.PilgrimID.Bytes).String(), Type: value.Type, CreatedAt: value.CreatedAt.Time}
}
