package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type InquiryService struct {
	operators   *repository.OperatorRepository
	inquiries   *repository.InquiryRepository
	crm         *repository.CRMRepository
	audit       *repository.AuditRepository
	entitlement *EntitlementChecker
}

func NewInquiryService(operators *repository.OperatorRepository, inquiries *repository.InquiryRepository, crm *repository.CRMRepository, audit *repository.AuditRepository, entitlement *EntitlementChecker) *InquiryService {
	return &InquiryService{operators: operators, inquiries: inquiries, crm: crm, audit: audit, entitlement: entitlement}
}

// Submit is public — a storefront visitor with no Better Auth session, same
// trust boundary as RegistrationService.Submit. Available on every plan,
// unlike CreateLead: an operator without the CRM feature should still hear
// from an interested visitor, they just can't file it into a pipeline yet.
func (s *InquiryService) Submit(ctx context.Context, req *hajjv1.SubmitInquiryRequest) (*hajjv1.SubmitInquiryResponse, error) {
	if req == nil {
		return nil, serviceError("InquiryService.Submit", apperror.ErrValidation)
	}
	fullName, phone, email := strings.TrimSpace(req.FullName), strings.TrimSpace(req.Phone), strings.TrimSpace(req.Email)
	if fullName == "" || (phone == "" && email == "") {
		return nil, serviceError("InquiryService.Submit", apperror.ErrValidation)
	}
	if _, err := s.operators.GetByID(ctx, req.OperatorId); err != nil {
		return nil, serviceError("InquiryService.Submit", apperror.ErrNotFound)
	}
	inquiry, err := s.inquiries.Create(ctx, req.OperatorId, fullName, phone, email, req.Message, req.UtmSource, req.UtmCampaign)
	if err != nil {
		return nil, serviceError("InquiryService.Submit", err)
	}
	_ = s.audit.Write(ctx, req.OperatorId, "", "storefront_inquiry_submitted", "storefront_inquiry", inquiry.ID, inquiry.FullName)
	return &hajjv1.SubmitInquiryResponse{Message: "Pesan Anda telah terkirim. Tim kami akan segera menghubungi Anda."}, nil
}

func (s *InquiryService) List(ctx context.Context, orgID string, req *hajjv1.ListInquiriesRequest) (*hajjv1.ListInquiriesResponse, error) {
	if req == nil {
		return nil, serviceError("InquiryService.List", apperror.ErrValidation)
	}
	operator, err := s.operators.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InquiryService.List", err)
	}
	inquiries, err := s.inquiries.List(ctx, operator.ID, req.Status)
	if err != nil {
		return nil, serviceError("InquiryService.List", err)
	}
	result := &hajjv1.ListInquiriesResponse{Inquiries: make([]*hajjv1.StorefrontInquiry, 0, len(inquiries))}
	for _, item := range inquiries {
		result.Inquiries = append(result.Inquiries, inquiryMessage(item))
	}
	return result, nil
}

// ConvertToLead is where K2.5 actually pays off: source is always WEBSITE
// (it can only be a storefront form) and campaign comes from the visitor's
// own utm_campaign (falling back to utm_source) instead of a staff member
// typing what they remember from a phone call. Still a staff action — the
// entitlement check and idempotency key are the same ones CreateLead already
// requires, just filled in here instead of on a form.
func (s *InquiryService) ConvertToLead(ctx context.Context, orgID string, req *hajjv1.ConvertInquiryToLeadRequest) (*hajjv1.CRMLead, error) {
	if req == nil || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("InquiryService.ConvertToLead", apperror.ErrValidation)
	}
	operator, err := s.operators.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InquiryService.ConvertToLead", err)
	}
	if err := s.entitlement.Check(ctx, operator.ID, "crm"); err != nil {
		return nil, serviceError("InquiryService.ConvertToLead", err)
	}
	inquiry, err := s.inquiries.Get(ctx, operator.ID, req.InquiryId)
	if err != nil {
		return nil, serviceError("InquiryService.ConvertToLead", err)
	}
	if inquiry.Status != "NEW" {
		return nil, serviceError("InquiryService.ConvertToLead", apperror.ErrFailedPrecondition)
	}
	campaign := inquiry.UTMCampaign
	if campaign == "" {
		campaign = inquiry.UTMSource
	}
	draft, err := crmDraft(inquiry.FullName, inquiry.Phone, inquiry.Email, "WEBSITE", campaign,
		"", "", "", 1, 0, "", nil, "Dikonversi dari pesan storefront: "+inquiry.Message, req.IdempotencyKey)
	if err != nil {
		return nil, serviceError("InquiryService.ConvertToLead", err)
	}
	userID := middleware.UserIDFromCtx(ctx)
	lead, created, err := s.crm.CreateLead(ctx, operator.ID, userID, draft)
	if err != nil {
		return nil, serviceError("InquiryService.ConvertToLead", err)
	}
	if err := s.inquiries.MarkConverted(ctx, operator.ID, inquiry.ID, lead.ID); err != nil {
		return nil, serviceError("InquiryService.ConvertToLead", err)
	}
	if created {
		_ = s.audit.Write(ctx, operator.ID, userID, "crm_lead_created", "crm_lead", lead.ID, lead.FullName)
	}
	return crmLeadMessage(*lead), nil
}

func (s *InquiryService) Dismiss(ctx context.Context, orgID string, req *hajjv1.DismissInquiryRequest) (*hajjv1.DismissInquiryResponse, error) {
	if req == nil {
		return nil, serviceError("InquiryService.Dismiss", apperror.ErrValidation)
	}
	operator, err := s.operators.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InquiryService.Dismiss", err)
	}
	inquiry, err := s.inquiries.Get(ctx, operator.ID, req.InquiryId)
	if err != nil {
		return nil, serviceError("InquiryService.Dismiss", err)
	}
	if inquiry.Status != "NEW" {
		return nil, serviceError("InquiryService.Dismiss", apperror.ErrFailedPrecondition)
	}
	if err := s.inquiries.MarkDismissed(ctx, operator.ID, inquiry.ID); err != nil {
		return nil, serviceError("InquiryService.Dismiss", err)
	}
	return &hajjv1.DismissInquiryResponse{Dismissed: true}, nil
}

func inquiryMessage(value domain.StorefrontInquiry) *hajjv1.StorefrontInquiry {
	return &hajjv1.StorefrontInquiry{
		Id: value.ID, FullName: value.FullName, Phone: value.Phone, Email: value.Email,
		Message: value.Message, UtmSource: value.UTMSource, UtmCampaign: value.UTMCampaign,
		Status: value.Status, ConvertedLeadId: value.ConvertedLeadID,
		CreatedAt: timestamppb.New(value.CreatedAt),
	}
}
