package service

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type StaffScheduleService struct {
	operatorRepository *repository.OperatorRepository
	staffRepository    *repository.StaffScheduleRepository
}

func NewStaffScheduleService(operators *repository.OperatorRepository, staff *repository.StaffScheduleRepository) *StaffScheduleService {
	return &StaffScheduleService{operatorRepository: operators, staffRepository: staff}
}

func (s *StaffScheduleService) Assign(ctx context.Context, authenticatedOrgID string, req *hajjv1.AssignStaffToKloterRequest) (*hajjv1.KloterStaff, error) {
	if req == nil || !isUUID(req.KloterId) || req.StaffId == "" || req.StaffName == "" {
		return nil, serviceError("StaffScheduleService.Assign", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("StaffScheduleService.Assign", err)
	}
	assignment, err := s.staffRepository.Assign(ctx, operator.ID, req.KloterId, req.StaffId, req.StaffName, req.StaffEmail, req.Role, req.Duties)
	if err != nil {
		return nil, serviceError("StaffScheduleService.Assign", err)
	}
	return kloterStaffMessage(assignment), nil
}

func (s *StaffScheduleService) ListForKloter(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListKloterStaffRequest) (*hajjv1.ListKloterStaffResponse, error) {
	if req == nil || !isUUID(req.KloterId) {
		return nil, serviceError("StaffScheduleService.ListForKloter", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("StaffScheduleService.ListForKloter", err)
	}
	assignments, err := s.staffRepository.ListForKloter(ctx, operator.ID, req.KloterId)
	if err != nil {
		return nil, serviceError("StaffScheduleService.ListForKloter", err)
	}
	result := &hajjv1.ListKloterStaffResponse{Staff: make([]*hajjv1.KloterStaff, 0, len(assignments))}
	for _, assignment := range assignments {
		result.Staff = append(result.Staff, kloterStaffMessage(assignment))
	}
	return result, nil
}

func (s *StaffScheduleService) Remove(ctx context.Context, authenticatedOrgID string, req *hajjv1.RemoveStaffFromKloterRequest) (*hajjv1.RemoveStaffFromKloterResponse, error) {
	if req == nil || !isUUID(req.KloterId) || req.StaffId == "" {
		return nil, serviceError("StaffScheduleService.Remove", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("StaffScheduleService.Remove", err)
	}
	if err := s.staffRepository.Remove(ctx, req.KloterId, req.StaffId, operator.ID); err != nil {
		return nil, serviceError("StaffScheduleService.Remove", err)
	}
	return &hajjv1.RemoveStaffFromKloterResponse{}, nil
}

func (s *StaffScheduleService) ListAll(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListAllStaffScheduleRequest) (*hajjv1.ListAllStaffScheduleResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("StaffScheduleService.ListAll", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("StaffScheduleService.ListAll", err)
	}
	summaries, err := s.staffRepository.ListAll(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("StaffScheduleService.ListAll", err)
	}
	result := &hajjv1.ListAllStaffScheduleResponse{Kloters: make([]*hajjv1.KloterScheduleSummary, 0, len(summaries))}
	for _, summary := range summaries {
		entry := &hajjv1.KloterScheduleSummary{
			KloterId: summary.KloterID, KloterName: summary.KloterName, SeasonName: summary.SeasonName,
			StaffCount: int32(summary.StaffCount), StaffNames: summary.StaffNames,
		}
		if summary.DepartureDate != nil {
			entry.DepartureDate = timestamppb.New(*summary.DepartureDate)
		}
		result.Kloters = append(result.Kloters, entry)
	}
	return result, nil
}

func (s *StaffScheduleService) ListMine(ctx context.Context, authenticatedOrgID, userID string) (*hajjv1.ListMyAssignmentsResponse, error) {
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("StaffScheduleService.ListMine", err)
	}
	assignments, err := s.staffRepository.ListMine(ctx, operator.ID, userID)
	if err != nil {
		return nil, serviceError("StaffScheduleService.ListMine", err)
	}
	result := &hajjv1.ListMyAssignmentsResponse{Assignments: make([]*hajjv1.KloterStaff, 0, len(assignments))}
	for _, assignment := range assignments {
		result.Assignments = append(result.Assignments, kloterStaffMessage(assignment))
	}
	return result, nil
}

func kloterStaffMessage(value *domain.KloterStaff) *hajjv1.KloterStaff {
	staff := &hajjv1.KloterStaff{
		Id: value.ID, KloterId: value.KloterID, KloterName: value.KloterName, StaffId: value.StaffID,
		StaffName: value.StaffName, StaffEmail: value.StaffEmail, Role: value.Role, Duties: value.Duties, SeasonName: value.SeasonName, SeasonId: value.SeasonID,
	}
	if value.DepartureDate != nil {
		staff.DepartureDate = timestamppb.New(*value.DepartureDate)
	}
	return staff
}
