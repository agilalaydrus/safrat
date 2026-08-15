package service

import (
	"context"
	"errors"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OperatorService struct {
	repository *repository.OperatorRepository
}

func NewOperatorService(repository *repository.OperatorRepository) *OperatorService {
	return &OperatorService{repository: repository}
}

func (s *OperatorService) Create(ctx context.Context, authenticatedOrgID string, request *hajjv1.CreateOperatorRequest) (*hajjv1.Operator, error) {
	if request == nil {
		return nil, serviceError("OperatorService.Create", apperror.ErrValidation)
	}
	if authenticatedOrgID == "" {
		return nil, serviceError("OperatorService.Create", apperror.ErrUnauthorized)
	}
	if authenticatedOrgID != request.BetterAuthOrgId {
		return nil, serviceError("OperatorService.Create", apperror.ErrForbidden)
	}
	if request.Country != "" && len(request.Country) != 2 {
		return nil, serviceError("OperatorService.Create", apperror.ErrValidation)
	}
	operator, err := s.repository.GetByBetterAuthOrgID(ctx, request.BetterAuthOrgId)
	if err == nil {
		return operatorMessage(operator), nil
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return nil, serviceError("OperatorService.Create", err)
	}
	operator, err = s.repository.Create(ctx, request.BetterAuthOrgId, request.Name, request.Country, request.Email, request.LicenseNumber)
	if err != nil {
		return nil, serviceError("OperatorService.Create", err)
	}
	return operatorMessage(operator), nil
}

func (s *OperatorService) GetMy(ctx context.Context, authenticatedOrgID string) (*hajjv1.Operator, error) {
	if authenticatedOrgID == "" {
		return nil, serviceError("OperatorService.GetMy", apperror.ErrUnauthorized)
	}
	operator, err := s.repository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.GetMy", err)
	}
	return operatorMessage(operator), nil
}

func (s *OperatorService) ListAuditLogs(ctx context.Context, authenticatedOrgID string, limit int32) ([]*hajjv1.AuditLog, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	operator, err := s.repository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.ListAuditLogs", err)
	}
	rows, err := s.repository.ListAuditLogs(ctx, operator.ID, limit)
	if err != nil {
		return nil, serviceError("OperatorService.ListAuditLogs", err)
	}
	result := make([]*hajjv1.AuditLog, 0, len(rows))
	for _, row := range rows {
		result = append(result, &hajjv1.AuditLog{Id: row.ID, Action: row.Action, EntityType: row.EntityType, EntityId: row.EntityID, Description: row.Description, CreatedAt: timestamppb.New(row.CreatedAt), ActorName: row.ActorName})
	}
	return result, nil
}

func operatorMessage(value *domain.Operator) *hajjv1.Operator {
	return &hajjv1.Operator{
		Id:              value.ID,
		BetterAuthOrgId: value.BetterAuthOrgID,
		Name:            value.Name,
		Country:         value.Country,
		Email:           value.Email,
		LicenseNumber:   value.LicenseNumber,
		CreatedAt:       timestamppb.New(value.CreatedAt),
	}
}
