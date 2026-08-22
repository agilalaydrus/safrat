package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type JourneyRepository struct{ queries *db.Queries }

func NewJourneyRepository(queries *db.Queries) *JourneyRepository {
	return &JourneyRepository{queries: queries}
}

// UpdateStatus upserts the pilgrim's current status and appends an
// immutable log entry — from is the caller's already-known prior status
// (GetStatus or a prior list call), so this never needs a read-then-write
// race inside the repository.
func (r *JourneyRepository) UpdateStatus(ctx context.Context, operatorID, pilgrimID, from, to, updatedByUserID, notes string) (*domain.PilgrimJourneyStatus, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	updatedBy := pgtype.Text{String: updatedByUserID, Valid: updatedByUserID != ""}
	v, err := r.queries.UpsertPilgrimJourneyStatus(ctx, db.UpsertPilgrimJourneyStatusParams{OperatorID: opUUID, PilgrimID: pilgrimUUID, Status: to, UpdatedBy: updatedBy, Notes: notes})
	if err != nil {
		return nil, err
	}
	if err := r.queries.InsertPilgrimJourneyLog(ctx, db.InsertPilgrimJourneyLogParams{OperatorID: opUUID, PilgrimID: pilgrimUUID, FromStatus: from, ToStatus: to, UpdatedBy: updatedBy, Notes: notes}); err != nil {
		return nil, err
	}
	return &domain.PilgrimJourneyStatus{PilgrimID: uuidString(v.PilgrimID), Status: v.Status, UpdatedAt: v.UpdatedAt.Time, Notes: v.Notes}, nil
}

func (r *JourneyRepository) GetStatus(ctx context.Context, operatorID, pilgrimID string) (*domain.PilgrimJourneyStatus, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	v, err := r.queries.GetPilgrimJourneyStatus(ctx, db.GetPilgrimJourneyStatusParams{PilgrimID: pilgrimUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	return &domain.PilgrimJourneyStatus{PilgrimID: uuidString(v.PilgrimID), Status: v.Status, UpdatedByName: v.UpdatedByName, UpdatedAt: v.UpdatedAt.Time, Notes: v.Notes}, nil
}

// ListByKloter returns every non-substituted pilgrim in the kloter with
// their current status (REGISTERED if they've never had one recorded) —
// used by BulkUpdateStatus to know each pilgrim's "from" status for the log.
func (r *JourneyRepository) ListByKloter(ctx context.Context, operatorID, kloterID string) (map[string]string, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListJourneyStatusesByKloter(ctx, db.ListJourneyStatusesByKloterParams{OperatorID: opUUID, KloterID: kloterUUID})
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		result[uuidString(row.PilgrimID)] = row.Status
	}
	return result, nil
}

func (r *JourneyRepository) CountByKloter(ctx context.Context, operatorID, kloterID string) (map[string]int32, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.CountJourneyStatusesByKloter(ctx, db.CountJourneyStatusesByKloterParams{OperatorID: opUUID, KloterID: kloterUUID})
	if err != nil {
		return nil, err
	}
	result := make(map[string]int32, len(rows))
	for _, row := range rows {
		result[row.Status] = row.Count
	}
	return result, nil
}

// ListPilgrimIDsByGroup resolves the pilgrim ids for a group's bulk-status
// cascade (used by GroupService.UpdateGroupCity) — reuses the same
// operator/group scoping as GroupRepository.GetRoster.
func (r *JourneyRepository) ListPilgrimIDsByGroup(ctx context.Context, operatorID, groupID string) ([]string, error) {
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
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, uuidString(row.ID))
	}
	return result, nil
}

// ListPilgrimIDsByKloter is the kloter-wide equivalent, used by
// KloterService.UpdateKloterStatus's bulk cascade.
func (r *JourneyRepository) ListPilgrimIDsByKloter(ctx context.Context, operatorID, kloterID string) ([]string, error) {
	statuses, err := r.ListByKloter(ctx, operatorID, kloterID)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(statuses))
	for id := range statuses {
		result = append(result, id)
	}
	return result, nil
}
