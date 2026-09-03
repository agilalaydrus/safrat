package service

import (
	"context"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RegistrationService struct {
	operatorRepository     *repository.OperatorRepository
	registrationRepository *repository.RegistrationRepository
	auditRepository        *repository.AuditRepository
	agentRepository        *repository.AgentRepository
}

func NewRegistrationService(operators *repository.OperatorRepository, registrations *repository.RegistrationRepository, audit *repository.AuditRepository, agents *repository.AgentRepository) *RegistrationService {
	return &RegistrationService{operatorRepository: operators, registrationRepository: registrations, auditRepository: audit, agentRepository: agents}
}

// SubmitRegistration is public — a prospective pilgrim fills this out with
// no Better Auth session, authenticated only by knowing a real
// operator_id+season_id link an operator shared with them. Never trusts
// anything about "is this operator/season real and accepting registrations"
// from the client; re-validated against the DB every time.
func (s *RegistrationService) Submit(ctx context.Context, req *hajjv1.SubmitRegistrationRequest) (*hajjv1.SubmitRegistrationResponse, error) {
	if req == nil || !isUUID(req.OperatorId) || !isUUID(req.SeasonId) {
		return nil, serviceError("RegistrationService.Submit", apperror.ErrValidation)
	}
	if _, _, err := s.registrationRepository.GetOperatorSeasonInfo(ctx, req.OperatorId, req.SeasonId); err != nil {
		return nil, serviceError("RegistrationService.Submit", apperror.ErrNotFound)
	}
	var dob *time.Time
	if req.DateOfBirth != nil {
		t := req.DateOfBirth.AsTime()
		dob = &t
	}
	// A typo'd or inactive referral code never blocks registration — just
	// silently results in no agent linkage.
	agentID := ""
	if code := strings.TrimSpace(req.ReferralCode); code != "" {
		if agent, err := s.agentRepository.GetByReferralCode(ctx, req.OperatorId, code); err == nil && agent.IsActive {
			agentID = agent.ID
		}
	}
	registration, err := s.registrationRepository.Create(ctx, req.OperatorId, req.SeasonId, req.ProductId, req.FullName, req.PassportNumber, dob, req.Gender, req.Phone, req.Email, req.Nationality, req.Address, agentID,
		strings.TrimSpace(req.UtmSource), strings.TrimSpace(req.UtmCampaign))
	if err != nil {
		return nil, serviceError("RegistrationService.Submit", err)
	}
	_ = s.auditRepository.Write(ctx, req.OperatorId, "", "registration_submitted", "pilgrim_registration", registration.ID, registration.FullName)
	return &hajjv1.SubmitRegistrationResponse{
		RegistrationId: registration.ID,
		Message:        "Pendaftaran Anda telah diterima. Tim kami akan menghubungi Anda dalam 1x24 jam.",
	}, nil
}

func (s *RegistrationService) GetForm(ctx context.Context, req *hajjv1.GetRegistrationFormRequest) (*hajjv1.RegistrationFormInfo, error) {
	if req == nil || !isUUID(req.OperatorId) || !isUUID(req.SeasonId) {
		return nil, serviceError("RegistrationService.GetForm", apperror.ErrValidation)
	}
	operatorName, seasonName, err := s.registrationRepository.GetOperatorSeasonInfo(ctx, req.OperatorId, req.SeasonId)
	if err != nil {
		return nil, serviceError("RegistrationService.GetForm", apperror.ErrNotFound)
	}
	products, err := s.registrationRepository.ListActiveProductNames(ctx, req.OperatorId, req.SeasonId)
	if err != nil {
		return nil, serviceError("RegistrationService.GetForm", err)
	}
	return &hajjv1.RegistrationFormInfo{OperatorName: operatorName, SeasonName: seasonName, AvailableProducts: products}, nil
}

func (s *RegistrationService) List(ctx context.Context, orgID string, req *hajjv1.ListRegistrationsRequest) (*hajjv1.ListRegistrationsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("RegistrationService.List", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("RegistrationService.List", err)
	}
	registrations, err := s.registrationRepository.List(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("RegistrationService.List", err)
	}
	result := &hajjv1.ListRegistrationsResponse{Registrations: make([]*hajjv1.PilgrimRegistration, 0, len(registrations))}
	for _, r := range registrations {
		result.Registrations = append(result.Registrations, registrationMessage(r))
	}
	return result, nil
}

func (s *RegistrationService) Approve(ctx context.Context, orgID string, req *hajjv1.ApproveRegistrationRequest) (*hajjv1.PilgrimRegistration, error) {
	if req == nil || !isUUID(req.RegistrationId) {
		return nil, serviceError("RegistrationService.Approve", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("RegistrationService.Approve", err)
	}
	registration, err := s.registrationRepository.UpdateStatus(ctx, op.ID, req.RegistrationId, "APPROVED", req.Notes)
	if err != nil {
		return nil, serviceError("RegistrationService.Approve", err)
	}
	_ = s.auditRepository.Write(ctx, op.ID, middleware.UserIDFromCtx(ctx), "registration_approved", "pilgrim_registration", registration.ID, registration.FullName)
	return registrationMessage(registration), nil
}

func (s *RegistrationService) Reject(ctx context.Context, orgID string, req *hajjv1.RejectRegistrationRequest) (*hajjv1.PilgrimRegistration, error) {
	if req == nil || !isUUID(req.RegistrationId) || strings.TrimSpace(req.Notes) == "" {
		return nil, serviceError("RegistrationService.Reject", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("RegistrationService.Reject", err)
	}
	registration, err := s.registrationRepository.UpdateStatus(ctx, op.ID, req.RegistrationId, "REJECTED", req.Notes)
	if err != nil {
		return nil, serviceError("RegistrationService.Reject", err)
	}
	_ = s.auditRepository.Write(ctx, op.ID, middleware.UserIDFromCtx(ctx), "registration_rejected", "pilgrim_registration", registration.ID, req.Notes)
	return registrationMessage(registration), nil
}

func registrationMessage(value *domain.PilgrimRegistration) *hajjv1.PilgrimRegistration {
	result := &hajjv1.PilgrimRegistration{
		Id: value.ID, OperatorId: value.OperatorID, SeasonId: value.SeasonID, ProductId: value.ProductID,
		FullName: value.FullName, PassportNumber: value.PassportNumber, Gender: value.Gender, Phone: value.Phone,
		Email: value.Email, Nationality: value.Nationality, Address: value.Address, Status: value.Status, Notes: value.Notes,
		CreatedAt: timestamppb.New(value.CreatedAt), ReferredByAgentId: value.AgentID, ReferredByAgentName: value.AgentName,
	}
	if value.DateOfBirth != nil {
		result.DateOfBirth = timestamppb.New(*value.DateOfBirth)
	}
	return result
}
