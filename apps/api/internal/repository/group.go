package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
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
		result = append(result, &domain.Group{ID: uuid.UUID(row.ID.Bytes).String(), SeasonID: uuid.UUID(row.SeasonID.Bytes).String(), OperatorID: uuid.UUID(row.OperatorID.Bytes).String(), Name: row.Name, Capacity: row.Capacity, PilgrimCount: row.PilgrimCount, LeaderID: row.LeaderID.String, LeaderName: row.LeaderName.String, KloterID: nullableUUIDString(row.KloterID), CurrentCity: row.CurrentCity, Status: row.Status, LastUpdate: timestamptzPtr(row.LastUpdate), CurrentActivity: row.CurrentActivity})
	}
	return result, nil
}

// ListByKloter powers the Kloter Detail "Rombongan" tab — every group
// assigned to this kloter, with its Muttawwif, jamaah count, and current
// location (so a coordinator can see the whole kloter's spread at a glance).
func (r *GroupRepository) ListByKloter(ctx context.Context, operatorID, kloterID string) ([]*domain.Group, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListGroupsByKloter(ctx, db.ListGroupsByKloterParams{OperatorID: opUUID, KloterID: kloterUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Group, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.Group{ID: uuid.UUID(row.ID.Bytes).String(), SeasonID: uuid.UUID(row.SeasonID.Bytes).String(), OperatorID: uuid.UUID(row.OperatorID.Bytes).String(), Name: row.Name, Capacity: row.Capacity, PilgrimCount: row.PilgrimCount, LeaderID: row.LeaderID.String, LeaderName: row.LeaderName.String, KloterID: nullableUUIDString(row.KloterID), CurrentCity: row.CurrentCity, Status: row.Status, LastUpdate: timestamptzPtr(row.LastUpdate), CurrentActivity: row.CurrentActivity})
	}
	return result, nil
}

// UpdateCity is the Muttawwif's one-tap location update — also appends an
// immutable group_location_log row so the trip trail can be reconstructed.
func (r *GroupRepository) UpdateCity(ctx context.Context, operatorID, groupID, city, activity, location, updatedByUserID string) (*domain.Group, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, err
	}
	v, err := r.queries.UpdateGroupCity(ctx, db.UpdateGroupCityParams{ID: groupUUID, OperatorID: opUUID, CurrentCity: city, CurrentActivity: activity})
	if err != nil {
		return nil, err
	}
	if err := r.queries.InsertGroupLocationLog(ctx, db.InsertGroupLocationLogParams{
		OperatorID: opUUID, GroupID: groupUUID, City: city, Location: location, UpdatedBy: pgtype.Text{String: updatedByUserID, Valid: updatedByUserID != ""},
	}); err != nil {
		return nil, err
	}
	return toGroup(v), nil
}

func (r *GroupRepository) ListMuttawwif(ctx context.Context, operatorID string) ([]*domain.Muttawwif, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListMuttawwif(ctx, opUUID)
	if err != nil {
		return nil, err
	}
	// One row per (leader, group) — fold rows sharing a user_id into one
	// Muttawwif with multiple groups, preserving the query's ORDER BY
	// user_name (map iteration order isn't stable, so track it separately).
	order := make([]string, 0, len(rows))
	byUser := make(map[string]*domain.Muttawwif, len(rows))
	for _, row := range rows {
		m, ok := byUser[row.UserID]
		if !ok {
			m = &domain.Muttawwif{UserID: row.UserID, Name: row.UserName, Email: row.UserEmail, Phone: row.Phone, AgentID: nullableUUIDString(row.AgentID), KYCStatus: row.KycStatus}
			byUser[row.UserID] = m
			order = append(order, row.UserID)
		}
		m.Groups = append(m.Groups, domain.LeaderGroup{
			ID: uuid.UUID(row.GroupID.Bytes).String(), Name: row.GroupName,
			Capacity: row.Capacity, PilgrimCount: row.PilgrimCount, SeasonID: uuid.UUID(row.SeasonID.Bytes).String(),
		})
	}
	result := make([]*domain.Muttawwif, 0, len(order))
	for _, userID := range order {
		result = append(result, byUser[userID])
	}
	return result, nil
}

// EnsureGroupBelongsToOperator is the multi-tenant boundary for
// operator-wide group access (e.g. admin chat monitoring) — weaker than
// GroupLeaderRepository.EnsureLeaderOwnsGroup (any operator member passes,
// not just the assigned leader), but still required: without it, a caller
// could pass another operator's group_id and write/read across tenants.
func (r *GroupRepository) EnsureGroupBelongsToOperator(ctx context.Context, operatorID, groupID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return err
	}
	_, err = r.queries.GetGroupForOperator(ctx, db.GetGroupForOperatorParams{ID: groupUUID, OperatorID: opUUID})
	return err
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

func (r *GroupRepository) GetRoster(ctx context.Context, operatorID, groupID string) ([]*domain.Pilgrim, error) {
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
	return &domain.Group{ID: uuid.UUID(group.ID.Bytes).String(), SeasonID: uuid.UUID(group.SeasonID.Bytes).String(), OperatorID: uuid.UUID(group.OperatorID.Bytes).String(), Name: group.Name, Capacity: group.Capacity, LeaderID: group.LeaderID.String, KloterID: nullableUUIDString(group.KloterID), CurrentCity: group.CurrentCity, Status: group.Status, LastUpdate: timestamptzPtr(group.LastUpdate), CurrentActivity: group.CurrentActivity}
}
