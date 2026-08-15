package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
)

type OperatorRepository struct {
	queries *db.Queries
}

func NewOperatorRepository(queries *db.Queries) *OperatorRepository {
	return &OperatorRepository{queries: queries}
}

func (r *OperatorRepository) Create(ctx context.Context, betterAuthOrgID, name, country, email, licenseNumber string) (*domain.Operator, error) {
	operator, err := r.queries.CreateOperator(ctx, db.CreateOperatorParams{
		BetterAuthOrgID: betterAuthOrgID,
		Name:            name,
		Country:         country,
		Email:           email,
		Column5:         licenseNumber,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return r.GetByBetterAuthOrgID(ctx, betterAuthOrgID)
	}
	if err != nil {
		return nil, err
	}
	return toOperator(operator), nil
}

func (r *OperatorRepository) GetByBetterAuthOrgID(ctx context.Context, betterAuthOrgID string) (*domain.Operator, error) {
	operator, err := r.queries.GetOperatorByBetterAuthOrgID(ctx, betterAuthOrgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toOperator(operator), nil
}

func (r *OperatorRepository) GetByID(ctx context.Context, operatorID string) (*domain.Operator, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	operator, err := r.queries.GetOperatorByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toOperator(operator), nil
}

func (r *OperatorRepository) ListIDs(ctx context.Context) ([]string, error) {
	rows, err := r.queries.ListOperatorIDs(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, uuid.UUID(row.Bytes).String())
	}
	return ids, nil
}

func (r *OperatorRepository) ListAuditLogs(ctx context.Context, operatorID string, limit int32) ([]*domain.AuditLog, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListAuditLogs(ctx, db.ListAuditLogsParams{OperatorID: id, Limit: limit})
	if err != nil {
		return nil, err
	}
	logs := make([]*domain.AuditLog, 0, len(rows))
	for _, row := range rows {
		logs = append(logs, &domain.AuditLog{ID: uuid.UUID(row.ID.Bytes).String(), Action: row.Action, EntityID: uuid.UUID(row.EntityID.Bytes).String(), Description: row.Description, CreatedAt: row.CreatedAt.Time})
	}
	return logs, nil
}

func toOperator(value db.Operator) *domain.Operator {
	return &domain.Operator{
		ID:              uuid.UUID(value.ID.Bytes).String(),
		BetterAuthOrgID: value.BetterAuthOrgID,
		Name:            value.Name,
		Country:         value.Country,
		Email:           value.Email,
		LicenseNumber:   value.LicenseNumber.String,
		CreatedAt:       value.CreatedAt.Time,
	}
}
