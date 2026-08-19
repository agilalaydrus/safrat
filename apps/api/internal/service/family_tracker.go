package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FamilyTrackerService struct {
	repository *repository.FamilyTrackerRepository
}

func NewFamilyTrackerService(repository *repository.FamilyTrackerRepository) *FamilyTrackerService {
	return &FamilyTrackerService{repository: repository}
}

// GetFamilyStatus is public — app_access_code is the only identity token,
// no operator/pilgrim session at all. Never distinguishes "code doesn't
// exist" from any other lookup failure in its response — a family member
// or an attacker both just see "not found".
func (s *FamilyTrackerService) GetFamilyStatus(ctx context.Context, req *hajjv1.GetFamilyStatusRequest) (*hajjv1.FamilyStatus, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" || len(req.AppAccessCode) > 64 {
		return nil, serviceError("FamilyTrackerService.GetFamilyStatus", apperror.ErrValidation)
	}
	status, err := s.repository.Get(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("FamilyTrackerService.GetFamilyStatus", apperror.ErrNotFound)
	}
	return familyStatusMessage(status), nil
}

func familyStatusMessage(value *domain.FamilyStatus) *hajjv1.FamilyStatus {
	result := &hajjv1.FamilyStatus{
		FirstName: value.FirstName, PaymentStatus: value.PaymentStatus, HotelCheckedIn: value.HotelCheckedIn,
		PilgrimStatus: value.PilgrimStatus, SeasonName: value.SeasonName, DepartureDate: timestamppb.New(value.DepartureDate),
		GroupName: value.GroupName, LeaderName: value.LeaderName, HasActiveSos: value.HasActiveSOS,
	}
	if value.LastLocationAt != nil {
		result.LastLocationAt = timestamppb.New(*value.LastLocationAt)
	}
	return result
}
