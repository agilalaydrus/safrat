package service

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type InsuranceService struct {
	operatorRepository  *repository.OperatorRepository
	insuranceRepository *repository.InsuranceRepository
}

func NewInsuranceService(operators *repository.OperatorRepository, insurance *repository.InsuranceRepository) *InsuranceService {
	return &InsuranceService{operatorRepository: operators, insuranceRepository: insurance}
}

func (s *InsuranceService) CreateClaim(ctx context.Context, authenticatedOrgID string, req *hajjv1.CreateInsuranceClaimRequest) (*hajjv1.InsuranceClaim, error) {
	if req == nil || !isUUID(req.PilgrimId) || req.Description == "" {
		return nil, serviceError("InsuranceService.CreateClaim", apperror.ErrValidation)
	}
	incidentDate, err := time.Parse(dueDateLayout, req.IncidentDate)
	if err != nil {
		return nil, serviceError("InsuranceService.CreateClaim", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("InsuranceService.CreateClaim", err)
	}
	claim, err := s.insuranceRepository.CreateClaim(ctx, req.PilgrimId, operator.ID, req.ClaimType, incidentDate, req.Description, req.ClaimAmountIdr, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("InsuranceService.CreateClaim", err)
	}
	return insuranceClaimMessage(claim), nil
}

func (s *InsuranceService) ListClaims(ctx context.Context, authenticatedOrgID string) (*hajjv1.ListInsuranceClaimsResponse, error) {
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("InsuranceService.ListClaims", err)
	}
	claims, err := s.insuranceRepository.ListClaims(ctx, operator.ID)
	if err != nil {
		return nil, serviceError("InsuranceService.ListClaims", err)
	}
	result := &hajjv1.ListInsuranceClaimsResponse{Claims: make([]*hajjv1.InsuranceClaim, 0, len(claims))}
	for _, claim := range claims {
		result.Claims = append(result.Claims, insuranceClaimMessage(claim))
	}
	return result, nil
}

func (s *InsuranceService) UpdateClaimStatus(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdateInsuranceClaimStatusRequest) (*hajjv1.InsuranceClaim, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("InsuranceService.UpdateClaimStatus", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("InsuranceService.UpdateClaimStatus", err)
	}
	claim, err := s.insuranceRepository.UpdateClaimStatus(ctx, operator.ID, req.Id, req.Status, req.SettledAmountIdr)
	if err != nil {
		return nil, serviceError("InsuranceService.UpdateClaimStatus", err)
	}
	return insuranceClaimMessage(claim), nil
}

func (s *InsuranceService) GetExportData(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetInsuranceClaimExportDataRequest) (*hajjv1.InsuranceClaimExportData, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("InsuranceService.GetExportData", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("InsuranceService.GetExportData", err)
	}
	data, err := s.insuranceRepository.GetExportData(ctx, req.Id, operator.ID)
	if err != nil {
		return nil, serviceError("InsuranceService.GetExportData", err)
	}
	return &hajjv1.InsuranceClaimExportData{
		FullName: data.FullName, PassportNumber: data.PassportNumber, DateOfBirth: timestamppb.New(data.DateOfBirth),
		Gender: data.Gender, Nationality: data.Nationality, Phone: data.Phone,
		EmergencyContactName: data.EmergencyContactName, EmergencyContactPhone: data.EmergencyContactPhone,
		BloodType: data.BloodType, ChronicConditions: data.ChronicConditions, CurrentMedications: data.CurrentMedications,
		InsuranceProvider: data.InsuranceProvider, InsurancePolicyNo: data.InsurancePolicyNo, InsuranceClass: data.InsuranceClass,
		MedicalNotes: data.MedicalNotes, SeasonName: data.SeasonName, SeasonStartDate: timestamppb.New(data.SeasonStartDate),
		SeasonEndDate: timestamppb.New(data.SeasonEndDate), OperatorName: data.OperatorName, OperatorLicenseNumber: data.OperatorLicenseNumber,
		OperatorPhone: data.OperatorPhone, Claim: insuranceClaimMessage(&data.Claim),
	}, nil
}

func insuranceClaimMessage(value *domain.InsuranceClaim) *hajjv1.InsuranceClaim {
	return &hajjv1.InsuranceClaim{
		Id: value.ID, PilgrimId: value.PilgrimID, PilgrimName: value.PilgrimName, PassportNumber: value.PassportNumber,
		InsuranceProvider: value.InsuranceProvider, InsurancePolicyNo: value.InsurancePolicyNo,
		ClaimType: value.ClaimType, IncidentDate: value.IncidentDate.Format(dueDateLayout), Description: value.Description,
		Status: value.Status, ClaimAmountIdr: value.ClaimAmountIDR, SettledAmountIdr: value.SettledAmountIDR,
		FiledBy: value.FiledBy, CreatedAt: timestamppb.New(value.CreatedAt),
	}
}
