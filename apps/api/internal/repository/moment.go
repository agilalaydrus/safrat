package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
)

type MomentRepository struct {
	queries *db.Queries
}

func NewMomentRepository(queries *db.Queries) *MomentRepository {
	return &MomentRepository{queries: queries}
}

func (r *MomentRepository) Create(ctx context.Context, operatorID, seasonID, pilgrimID, groupID, photoKey, caption, createdBy string) (*domain.Moment, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateMoment(ctx, db.CreateMomentParams{
		OperatorID: op, SeasonID: season, PilgrimID: pgUUIDOrNull(pilgrimID), GroupID: pgUUIDOrNull(groupID),
		PhotoKey: photoKey, Caption: caption, CreatedBy: createdBy,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.Moment{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
		PilgrimID: nullableUUIDString(row.PilgrimID), GroupID: nullableUUIDString(row.GroupID),
		PhotoKey: row.PhotoKey, Caption: row.Caption, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt.Time,
	}, nil
}

// Delete returns the stored photo key so the caller can remove the object
// from S3 too — an orphaned object left behind is a private photo nobody
// can reach through the app but that still sits in the bucket forever.
func (r *MomentRepository) Delete(ctx context.Context, operatorID, id string) (string, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return "", apperror.ErrValidation
	}
	momentID, err := pgUUID(id)
	if err != nil {
		return "", apperror.ErrValidation
	}
	photoKey, err := r.queries.DeleteMoment(ctx, db.DeleteMomentParams{ID: momentID, OperatorID: op})
	if err != nil {
		return "", databaseError(err)
	}
	return photoKey, nil
}

func (r *MomentRepository) ListForSeason(ctx context.Context, operatorID, seasonID string) ([]*domain.Moment, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListMoments(ctx, db.ListMomentsParams{OperatorID: op, SeasonID: season})
	if err != nil {
		return nil, databaseError(err)
	}
	moments := make([]*domain.Moment, 0, len(rows))
	for _, row := range rows {
		moments = append(moments, &domain.Moment{
			ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
			PilgrimID: nullableUUIDString(row.PilgrimID), PilgrimName: row.PilgrimName,
			GroupID: nullableUUIDString(row.GroupID), GroupName: row.GroupName,
			PhotoKey: row.PhotoKey, Caption: row.Caption, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt.Time,
		})
	}
	return moments, nil
}

// ListForFamily is the public read path: every moment addressed to this
// pilgrim directly, or to the group they belong to.
func (r *MomentRepository) ListForFamily(ctx context.Context, pilgrimID string) ([]*domain.FamilyMoment, error) {
	pilgrim, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListFamilyMoments(ctx, pilgrim)
	if err != nil {
		return nil, databaseError(err)
	}
	moments := make([]*domain.FamilyMoment, 0, len(rows))
	for _, row := range rows {
		moments = append(moments, &domain.FamilyMoment{PhotoKey: row.PhotoKey, Caption: row.Caption, CreatedAt: row.CreatedAt.Time})
	}
	return moments, nil
}
