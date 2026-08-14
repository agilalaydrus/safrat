package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type PilgrimRepository struct {
	queries *db.Queries
}

func NewPilgrimRepository(queries *db.Queries) *PilgrimRepository {
	return &PilgrimRepository{queries: queries}
}

func (r *PilgrimRepository) Create(ctx context.Context, operatorID string, input domain.PilgrimInput) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(input.SeasonID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.CreatePilgrim(ctx, db.CreatePilgrimParams{
		SeasonID:           seasonUUID,
		OperatorID:         operatorUUID,
		Column3:            input.GroupID,
		FullName:           input.FullName,
		PassportNumber:     input.PassportNumber,
		Nationality:        input.Nationality,
		DateOfBirth:        pgTimestamp(input.DateOfBirth),
		Gender:             input.Gender,
		Column9:            input.PhotoURL,
		Column10:           input.Phone,
		Column11:           input.EmergencyContact,
		PreferredLang:      input.PreferredLang,
		Column13:           input.MedicalNotes,
		RequiresWheelchair: input.RequiresWheelchair,
		Column15:           input.MahramID,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) Get(ctx context.Context, operatorID, pilgrimID string) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.GetPilgrim(ctx, db.GetPilgrimParams{ID: pilgrimUUID, OperatorID: operatorUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) GetByPassport(ctx context.Context, operatorID, seasonID, passportNumber string) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.GetPilgrimByPassport(ctx, db.GetPilgrimByPassportParams{OperatorID: operatorUUID, SeasonID: seasonUUID, PassportNumber: passportNumber})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) List(ctx context.Context, operatorID, seasonID string, limit, offset int32) ([]*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	pilgrims, err := r.queries.ListPilgrims(ctx, db.ListPilgrimsParams{OperatorID: operatorUUID, SeasonID: seasonUUID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, databaseError(err)
	}
	results := make([]*domain.Pilgrim, 0, len(pilgrims))
	for _, pilgrim := range pilgrims {
		results = append(results, toPilgrim(pilgrim))
	}
	return results, nil
}

func (r *PilgrimRepository) Update(ctx context.Context, operatorID, pilgrimID string, input domain.PilgrimInput) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.UpdatePilgrim(ctx, db.UpdatePilgrimParams{
		ID:                 pilgrimUUID,
		OperatorID:         operatorUUID,
		Column3:            input.GroupID,
		FullName:           input.FullName,
		PassportNumber:     input.PassportNumber,
		Nationality:        input.Nationality,
		DateOfBirth:        pgTimestamp(input.DateOfBirth),
		Gender:             input.Gender,
		Column9:            input.PhotoURL,
		Column10:           input.Phone,
		Column11:           input.EmergencyContact,
		PreferredLang:      input.PreferredLang,
		Column13:           input.MedicalNotes,
		RequiresWheelchair: input.RequiresWheelchair,
		Column15:           input.MahramID,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) MarkSubstituted(ctx context.Context, operatorID, pilgrimID string) error {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return err
	}
	if err := r.queries.MarkSubstituted(ctx, db.MarkSubstitutedParams{ID: pilgrimUUID, OperatorID: operatorUUID}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *PilgrimRepository) GetTx(ctx context.Context, tx pgx.Tx, operatorID, pilgrimID string) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.WithTx(tx).GetPilgrim(ctx, db.GetPilgrimParams{ID: pilgrimUUID, OperatorID: operatorUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) SubstitutePilgrimTx(ctx context.Context, tx pgx.Tx, originalID, replacementID, operatorID string) error {
	originalUUID, err := pgUUID(originalID)
	if err != nil {
		return err
	}
	replacementUUID, err := pgUUID(replacementID)
	if err != nil {
		return err
	}
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	return databaseError(r.queries.WithTx(tx).SubstitutePilgrim(ctx, db.SubstitutePilgrimParams{ID: originalUUID, SubstitutedByID: replacementUUID, OperatorID: operatorUUID}))
}

func (r *PilgrimRepository) TransferPilgrimGroupTx(ctx context.Context, tx pgx.Tx, pilgrimID, groupID, operatorID string) error {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return err
	}
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	return databaseError(r.queries.WithTx(tx).TransferPilgrimGroup(ctx, db.TransferPilgrimGroupParams{ID: pilgrimUUID, GroupID: groupUUID, OperatorID: operatorUUID}))
}

func (r *PilgrimRepository) WriteAuditLogTx(ctx context.Context, tx pgx.Tx, operatorID, userID, action, entityID, message string) error {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	entityUUID, err := pgUUID(entityID)
	if err != nil {
		return err
	}
	return databaseError(r.queries.WithTx(tx).CreateAuditLog(ctx, db.CreateAuditLogParams{OperatorID: operatorUUID, UserID: userID, Action: action, EntityID: entityUUID, JsonbBuildObject: message}))
}

func (r *PilgrimRepository) CountByOperator(ctx context.Context, operatorID string) (int64, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return 0, err
	}
	count, err := r.queries.CountPilgrimsByOperator(ctx, operatorUUID)
	if err != nil {
		return 0, databaseError(err)
	}
	return count, nil
}

func (r *PilgrimRepository) GetStats(ctx context.Context, operatorID, seasonID string) (db.GetPilgrimStatsRow, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return db.GetPilgrimStatsRow{}, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return db.GetPilgrimStatsRow{}, err
	}
	stats, err := r.queries.GetPilgrimStats(ctx, db.GetPilgrimStatsParams{OperatorID: operatorUUID, SeasonID: seasonUUID})
	if err != nil {
		return db.GetPilgrimStatsRow{}, databaseError(err)
	}
	return stats, nil
}

func databaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.ErrNotFound
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return apperror.ErrAlreadyExists
	}
	return err
}

func toPilgrim(value db.Pilgrim) *domain.Pilgrim {
	return &domain.Pilgrim{
		ID:                 uuidString(value.ID),
		SeasonID:           uuidString(value.SeasonID),
		OperatorID:         uuidString(value.OperatorID),
		GroupID:            nullableUUIDString(value.GroupID),
		FullName:           value.FullName,
		PassportNumber:     value.PassportNumber,
		Nationality:        value.Nationality,
		DateOfBirth:        value.DateOfBirth.Time,
		Gender:             value.Gender,
		PhotoURL:           value.PhotoUrl.String,
		Phone:              value.Phone.String,
		EmergencyContact:   value.EmergencyContact.String,
		PreferredLang:      value.PreferredLang,
		MedicalNotes:       value.MedicalNotes.String,
		RequiresWheelchair: value.RequiresWheelchair,
		MahramID:           nullableUUIDString(value.MahramID),
		IsSubstituted:      value.IsSubstituted,
		SubstitutedByID:    nullableUUIDString(value.SubstitutedByID),
		AppAccessCode:      value.AppAccessCode,
		CreatedAt:          value.CreatedAt.Time,
		UpdatedAt:          value.UpdatedAt.Time,
	}
}

func uuidString(value pgtype.UUID) string {
	return uuid.UUID(value.Bytes).String()
}

func nullableUUIDString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuidString(value)
}
