package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
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
		result = append(result, &domain.LeaderGroup{ID: uuid.UUID(row.ID.Bytes).String(), SeasonID: uuid.UUID(row.SeasonID.Bytes).String(), Name: row.Name, Capacity: row.Capacity, PilgrimCount: row.PilgrimCount, CurrentCity: row.CurrentCity, LastUpdate: timestamptzPtr(row.LastUpdate)})
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

// EnsureLeaderOwnsPilgrim confirms pilgrimID is in a group this leader
// actually leads — the leader-scoped counterpart of PilgrimRepository.Get
// used before any per-pilgrim action (hotel check-in, movement check-in).
func (r *GroupLeaderRepository) EnsureLeaderOwnsPilgrim(ctx context.Context, operatorID, pilgrimID, leaderUserID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return err
	}
	_, err = r.queries.PilgrimBelongsToLeader(ctx, db.PilgrimBelongsToLeaderParams{ID: pilgrimUUID, OperatorID: opUUID, LeaderID: pgText(leaderUserID)})
	return err
}

// GetMovementKloter resolves a movement's kloter_id, so ListCheckIns can be
// scoped to whether this leader has a pilgrim in that kloter — a movement
// with no kloter (kloter_id NULL) is never leader-visible.
func (r *GroupLeaderRepository) GetMovementKloter(ctx context.Context, operatorID, movementID string) (string, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return "", err
	}
	movementUUID, err := pgUUID(movementID)
	if err != nil {
		return "", err
	}
	kloterID, err := r.queries.GetMovementKloterID(ctx, db.GetMovementKloterIDParams{ID: movementUUID, OperatorID: opUUID})
	if err != nil {
		return "", err
	}
	if !kloterID.Valid {
		return "", pgx.ErrNoRows
	}
	return uuid.UUID(kloterID.Bytes).String(), nil
}

// EnsureLeaderHasPilgrimInKloter confirms this leader has at least one
// pilgrim in kloterID — used to scope ListCheckIns to movements actually
// relevant to this leader's own group, since ListCheckIns only has a
// movement_id, not a specific pilgrim.
func (r *GroupLeaderRepository) EnsureLeaderHasPilgrimInKloter(ctx context.Context, operatorID, kloterID, leaderUserID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return err
	}
	_, err = r.queries.LeaderHasPilgrimInKloter(ctx, db.LeaderHasPilgrimInKloterParams{OperatorID: opUUID, LeaderID: pgText(leaderUserID), KloterID: kloterUUID})
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
