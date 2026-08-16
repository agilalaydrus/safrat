package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type KloterService struct {
	operatorRepository *repository.OperatorRepository
	kloterRepository   *repository.KloterRepository
	auditRepository    *repository.AuditRepository
}

func NewKloterService(operators *repository.OperatorRepository, kloters *repository.KloterRepository, audit *repository.AuditRepository) *KloterService {
	return &KloterService{operatorRepository: operators, kloterRepository: kloters, auditRepository: audit}
}

func (s *KloterService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "kloter", entityID, message)
}

func (s *KloterService) ListKloters(ctx context.Context, orgID string, req *hajjv1.ListKlotersRequest) (*hajjv1.ListKlotersResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("KloterService.ListKloters", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.ListKloters", err)
	}
	kloters, err := s.kloterRepository.ListForOperator(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("KloterService.ListKloters", err)
	}
	result := &hajjv1.ListKlotersResponse{Kloters: make([]*hajjv1.Kloter, 0, len(kloters))}
	for _, k := range kloters {
		result.Kloters = append(result.Kloters, kloterMessage(k))
	}
	return result, nil
}

func (s *KloterService) CreateKloter(ctx context.Context, orgID string, req *hajjv1.CreateKloterRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("KloterService.CreateKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.CreateKloter", err)
	}
	kloter, err := s.kloterRepository.Create(ctx, op.ID, req.SeasonId, req.Code, req.Embarkation, req.FlightNumber, timestampPtr(req.DepartureDate), req.Capacity)
	if err != nil {
		return nil, serviceError("KloterService.CreateKloter", err)
	}
	s.logActivity(ctx, op.ID, "kloter_created", kloter.ID, fmt.Sprintf("Kloter %s dibuat", kloter.Code))
	return kloterMessage(kloter), nil
}

func (s *KloterService) UpdateKloter(ctx context.Context, orgID string, req *hajjv1.UpdateKloterRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("KloterService.UpdateKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloter", err)
	}
	kloter, err := s.kloterRepository.Update(ctx, op.ID, req.KloterId, req.Code, req.Embarkation, req.FlightNumber, timestampPtr(req.DepartureDate), req.Capacity)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloter", err)
	}
	s.logActivity(ctx, op.ID, "kloter_updated", kloter.ID, fmt.Sprintf("Kloter %s diperbarui", kloter.Code))
	return kloterMessage(kloter), nil
}

func (s *KloterService) DeleteKloter(ctx context.Context, orgID string, req *hajjv1.DeleteKloterRequest) (*emptypb.Empty, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.DeleteKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.DeleteKloter", err)
	}
	if err := s.kloterRepository.Delete(ctx, op.ID, req.KloterId); err != nil {
		return nil, serviceError("KloterService.DeleteKloter", err)
	}
	return &emptypb.Empty{}, nil
}

func kloterMessage(k *domain.Kloter) *hajjv1.Kloter {
	msg := &hajjv1.Kloter{Id: k.ID, SeasonId: k.SeasonID, Code: k.Code, Embarkation: k.Embarkation, FlightNumber: k.FlightNumber, Capacity: k.Capacity, PilgrimCount: k.PilgrimCount}
	if k.DepartureDate != nil {
		msg.DepartureDate = timestamppb.New(*k.DepartureDate)
	}
	return msg
}
