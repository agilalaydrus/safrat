package service

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CRMService struct {
	operators   *repository.OperatorRepository
	crm         *repository.CRMRepository
	audit       *repository.AuditRepository
	entitlement *EntitlementChecker
}

func NewCRMService(operators *repository.OperatorRepository, crm *repository.CRMRepository, audit *repository.AuditRepository, entitlement *EntitlementChecker) *CRMService {
	return &CRMService{operators: operators, crm: crm, audit: audit, entitlement: entitlement}
}

func (s *CRMService) operator(ctx context.Context, orgID string) (string, error) {
	operator, err := s.operators.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return "", err
	}
	if err := s.entitlement.Check(ctx, operator.ID, "crm"); err != nil {
		return "", err
	}
	return operator.ID, nil
}

func (s *CRMService) CreateLead(ctx context.Context, orgID string, req *hajjv1.CreateLeadRequest) (*hajjv1.CreateLeadResponse, error) {
	if req == nil {
		return nil, serviceError("CRMService.CreateLead", apperror.ErrValidation)
	}
	draft, err := crmDraft(req.FullName, req.Phone, req.Email, crmSourceDB(req.Source), req.Campaign,
		req.SeasonId, req.ProductId, req.AssigneeUserId, req.Pax, req.EstimatedValueIdr,
		req.NextAction, timestampInput(req.NextFollowUpAt), req.Note, req.IdempotencyKey)
	if err != nil {
		return nil, serviceError("CRMService.CreateLead", err)
	}
	opID, err := s.operator(ctx, orgID)
	if err != nil {
		return nil, serviceError("CRMService.CreateLead", err)
	}
	userID := middleware.UserIDFromCtx(ctx)
	lead, created, err := s.crm.CreateLead(ctx, opID, userID, draft)
	if err != nil {
		return nil, serviceError("CRMService.CreateLead", err)
	}
	if created {
		_ = s.audit.Write(ctx, opID, userID, "crm_lead_created", "crm_lead", lead.ID, lead.FullName)
	}
	return &hajjv1.CreateLeadResponse{Lead: crmLeadMessage(*lead), Created: created}, nil
}

func (s *CRMService) ListLeads(ctx context.Context, orgID string, req *hajjv1.ListLeadsRequest) (*hajjv1.ListLeadsResponse, error) {
	if req == nil {
		return nil, serviceError("CRMService.ListLeads", apperror.ErrValidation)
	}
	opID, err := s.operator(ctx, orgID)
	if err != nil {
		return nil, serviceError("CRMService.ListLeads", err)
	}
	leads, total, err := s.crm.ListLeads(ctx, opID, domain.CRMLeadFilter{
		Stage: crmStageDB(req.Stage), Source: crmSourceFilterDB(req.Source), Search: req.Search,
		Limit: req.Limit, Offset: req.Offset,
	})
	if err != nil {
		return nil, serviceError("CRMService.ListLeads", err)
	}
	result := &hajjv1.ListLeadsResponse{Leads: make([]*hajjv1.CRMLead, 0, len(leads)), TotalCount: total}
	for _, lead := range leads {
		result.Leads = append(result.Leads, crmLeadMessage(lead))
	}
	return result, nil
}

func (s *CRMService) GetLead(ctx context.Context, orgID string, req *hajjv1.GetLeadRequest) (*hajjv1.CRMLeadDetail, error) {
	if req == nil || !isUUID(req.LeadId) {
		return nil, serviceError("CRMService.GetLead", apperror.ErrValidation)
	}
	opID, err := s.operator(ctx, orgID)
	if err != nil {
		return nil, serviceError("CRMService.GetLead", err)
	}
	detail, err := s.crm.GetLeadDetail(ctx, opID, req.LeadId)
	if err != nil {
		return nil, serviceError("CRMService.GetLead", err)
	}
	result := &hajjv1.CRMLeadDetail{Lead: crmLeadMessage(detail.Lead), Activities: make([]*hajjv1.CRMLeadActivity, 0, len(detail.Activities))}
	for _, activity := range detail.Activities {
		result.Activities = append(result.Activities, crmActivityMessage(activity))
	}
	return result, nil
}

func (s *CRMService) UpdateLead(ctx context.Context, orgID string, req *hajjv1.UpdateLeadRequest) (*hajjv1.CRMLead, error) {
	if req == nil || !isUUID(req.LeadId) {
		return nil, serviceError("CRMService.UpdateLead", apperror.ErrValidation)
	}
	draft, err := crmDraft(req.FullName, req.Phone, req.Email, crmSourceDB(req.Source), req.Campaign,
		req.SeasonId, req.ProductId, req.AssigneeUserId, req.Pax, req.EstimatedValueIdr,
		req.NextAction, timestampInput(req.NextFollowUpAt), "", req.IdempotencyKey)
	if err != nil || strings.TrimSpace(req.Reason) == "" {
		return nil, serviceError("CRMService.UpdateLead", apperror.ErrValidation)
	}
	opID, err := s.operator(ctx, orgID)
	if err != nil {
		return nil, serviceError("CRMService.UpdateLead", err)
	}
	userID := middleware.UserIDFromCtx(ctx)
	lead, applied, err := s.crm.UpdateLead(ctx, opID, userID, req.LeadId, req.Reason, req.IdempotencyKey, draft)
	if err != nil {
		return nil, serviceError("CRMService.UpdateLead", err)
	}
	if applied {
		_ = s.audit.Write(ctx, opID, userID, "crm_lead_updated", "crm_lead", lead.ID, req.Reason)
	}
	return crmLeadMessage(*lead), nil
}

func (s *CRMService) MoveLeadStage(ctx context.Context, orgID string, req *hajjv1.MoveLeadStageRequest) (*hajjv1.CRMLead, error) {
	if req == nil || !isUUID(req.LeadId) || req.Stage == hajjv1.CRMLeadStage_CRM_LEAD_STAGE_UNSPECIFIED || strings.TrimSpace(req.Reason) == "" {
		return nil, serviceError("CRMService.MoveLeadStage", apperror.ErrValidation)
	}
	opID, err := s.operator(ctx, orgID)
	if err != nil {
		return nil, serviceError("CRMService.MoveLeadStage", err)
	}
	userID := middleware.UserIDFromCtx(ctx)
	lead, applied, err := s.crm.MoveStage(ctx, opID, userID, req.LeadId, crmStageDB(req.Stage), req.Reason, req.IdempotencyKey)
	if err != nil {
		return nil, serviceError("CRMService.MoveLeadStage", err)
	}
	if applied {
		_ = s.audit.Write(ctx, opID, userID, "crm_lead_stage_changed", "crm_lead", lead.ID, req.Reason)
	}
	return crmLeadMessage(*lead), nil
}

func (s *CRMService) AddLeadActivity(ctx context.Context, orgID string, req *hajjv1.AddLeadActivityRequest) (*hajjv1.CRMLeadActivity, error) {
	if req == nil || !isUUID(req.LeadId) || strings.TrimSpace(req.Note) == "" {
		return nil, serviceError("CRMService.AddLeadActivity", apperror.ErrValidation)
	}
	kind := crmActivityDB(req.Kind)
	if kind == "" {
		return nil, serviceError("CRMService.AddLeadActivity", apperror.ErrValidation)
	}
	occurredAt := time.Now()
	if req.OccurredAt != nil {
		if !req.OccurredAt.IsValid() || req.OccurredAt.AsTime().After(time.Now().Add(5*time.Minute)) {
			return nil, serviceError("CRMService.AddLeadActivity", apperror.ErrValidation)
		}
		occurredAt = req.OccurredAt.AsTime()
	}
	opID, err := s.operator(ctx, orgID)
	if err != nil {
		return nil, serviceError("CRMService.AddLeadActivity", err)
	}
	userID := middleware.UserIDFromCtx(ctx)
	activity, created, err := s.crm.AddActivity(ctx, opID, userID, req.LeadId, kind, req.Note,
		req.NextAction, req.IdempotencyKey, occurredAt, timestampInput(req.NextFollowUpAt))
	if err != nil {
		return nil, serviceError("CRMService.AddLeadActivity", err)
	}
	if created {
		_ = s.audit.Write(ctx, opID, userID, "crm_lead_activity_added", "crm_lead", req.LeadId, kind)
	}
	return crmActivityMessage(*activity), nil
}

func (s *CRMService) GetDashboard(ctx context.Context, orgID string, _ *hajjv1.GetDashboardRequest) (*hajjv1.CRMDashboard, error) {
	opID, err := s.operator(ctx, orgID)
	if err != nil {
		return nil, serviceError("CRMService.GetDashboard", err)
	}
	dashboard, err := s.crm.Dashboard(ctx, opID)
	if err != nil {
		return nil, serviceError("CRMService.GetDashboard", err)
	}
	result := &hajjv1.CRMDashboard{
		ActiveCount: dashboard.ActiveCount, PipelineValueIdr: dashboard.PipelineValueIDR,
		MonthlyConversionBps: dashboard.MonthlyConversionBPS,
		OverdueFollowUpCount: dashboard.OverdueFollowUpCount, SourceCount: dashboard.SourceCount,
		UpdatedAt: timestamppb.New(dashboard.UpdatedAt),
	}
	for _, item := range dashboard.Stages {
		result.Stages = append(result.Stages, &hajjv1.CRMStageMetric{Stage: crmStageProto(item.Stage), LeadCount: item.Count, ValueIdr: item.ValueIDR})
	}
	for _, item := range dashboard.Sources {
		result.Sources = append(result.Sources, &hajjv1.CRMSourceMetric{Source: crmSourceProto(item.Source), LeadCount: item.Count, ValueIdr: item.ValueIDR})
	}
	for _, item := range dashboard.Assignees {
		result.Assignees = append(result.Assignees, &hajjv1.CRMAssigneeMetric{UserId: item.UserID, Name: item.Name, ActiveCount: item.ActiveCount, ClosingCount: item.ClosingCount, ValueIdr: item.ValueIDR})
	}
	for _, lead := range dashboard.AttentionLeads {
		result.AttentionLeads = append(result.AttentionLeads, crmLeadMessage(lead))
	}
	return result, nil
}

func (s *CRMService) ListAssignees(ctx context.Context, orgID string, _ *hajjv1.ListAssigneesRequest) (*hajjv1.ListAssigneesResponse, error) {
	opID, err := s.operator(ctx, orgID)
	if err != nil {
		return nil, serviceError("CRMService.ListAssignees", err)
	}
	assignees, err := s.crm.ListAssignees(ctx, opID)
	if err != nil {
		return nil, serviceError("CRMService.ListAssignees", err)
	}
	result := &hajjv1.ListAssigneesResponse{Assignees: make([]*hajjv1.CRMAssignee, 0, len(assignees))}
	for _, assignee := range assignees {
		result.Assignees = append(result.Assignees, &hajjv1.CRMAssignee{UserId: assignee.UserID, Name: assignee.Name, Email: assignee.Email})
	}
	return result, nil
}

func crmDraft(fullName, phone, email, source, campaign, seasonID, productID, assigneeID string, pax int32,
	value int64, nextAction string, nextFollowUp *time.Time, note, idempotencyKey string) (domain.CRMLeadDraft, error) {
	fullName, phone, email = strings.TrimSpace(fullName), strings.TrimSpace(phone), strings.ToLower(strings.TrimSpace(email))
	if fullName == "" || source == "" || (phone == "" && email == "") || pax < 1 || value < 0 || strings.TrimSpace(idempotencyKey) == "" {
		return domain.CRMLeadDraft{}, apperror.ErrValidation
	}
	if email != "" {
		address, err := mail.ParseAddress(email)
		if err != nil || !strings.EqualFold(address.Address, email) {
			return domain.CRMLeadDraft{}, apperror.ErrValidation
		}
	}
	if (seasonID != "" && !isUUID(seasonID)) || (productID != "" && !isUUID(productID)) {
		return domain.CRMLeadDraft{}, apperror.ErrValidation
	}
	return domain.CRMLeadDraft{FullName: fullName, Phone: phone, Email: email, Source: source,
		Campaign: strings.TrimSpace(campaign), SeasonID: seasonID, ProductID: productID,
		AssigneeUserID: assigneeID, Pax: pax, EstimatedValueIDR: value,
		NextAction: strings.TrimSpace(nextAction), NextFollowUpAt: nextFollowUp,
		Note: strings.TrimSpace(note), IdempotencyKey: strings.TrimSpace(idempotencyKey)}, nil
}

func timestampInput(value *timestamppb.Timestamp) *time.Time {
	if value == nil || !value.IsValid() {
		return nil
	}
	t := value.AsTime()
	return &t
}

func crmLeadMessage(value domain.CRMLead) *hajjv1.CRMLead {
	result := &hajjv1.CRMLead{Id: value.ID, OperatorId: value.OperatorID, BranchId: value.BranchID,
		FullName: value.FullName, Phone: value.Phone, Email: value.Email, Source: crmSourceProto(value.Source),
		Campaign: value.Campaign, Stage: crmStageProto(value.Stage), SeasonId: value.SeasonID,
		SeasonName: value.SeasonName, ProductId: value.ProductID, ProductName: value.ProductName,
		AssigneeUserId: value.AssigneeUserID, AssigneeName: value.AssigneeName, Pax: value.Pax,
		EstimatedValueIdr: value.EstimatedValueIDR, NextAction: value.NextAction,
		CreatedAt: timestamppb.New(value.CreatedAt), UpdatedAt: timestamppb.New(value.UpdatedAt)}
	if value.NextFollowUpAt != nil {
		result.NextFollowUpAt = timestamppb.New(*value.NextFollowUpAt)
	}
	if value.LastContactAt != nil {
		result.LastContactAt = timestamppb.New(*value.LastContactAt)
	}
	if value.ClosedAt != nil {
		result.ClosedAt = timestamppb.New(*value.ClosedAt)
	}
	return result
}

func crmActivityMessage(value domain.CRMLeadActivity) *hajjv1.CRMLeadActivity {
	return &hajjv1.CRMLeadActivity{Id: value.ID, LeadId: value.LeadID, Kind: crmActivityProto(value.Kind),
		FromStage: crmStageProto(value.FromStage), ToStage: crmStageProto(value.ToStage), Note: value.Note,
		ActorUserId: value.ActorUserID, ActorName: value.ActorName, OccurredAt: timestamppb.New(value.OccurredAt)}
}

func crmStageDB(value hajjv1.CRMLeadStage) string {
	return map[hajjv1.CRMLeadStage]string{1: "NEW", 2: "CONTACT", 3: "OFFER", 4: "HOT", 5: "CLOSING", 6: "CANCELLED"}[value]
}
func crmStageProto(value string) hajjv1.CRMLeadStage {
	return map[string]hajjv1.CRMLeadStage{"NEW": 1, "CONTACT": 2, "OFFER": 3, "HOT": 4, "CLOSING": 5, "CANCELLED": 6}[value]
}
func crmSourceDB(value hajjv1.CRMLeadSource) string {
	return map[hajjv1.CRMLeadSource]string{1: "WEBSITE", 2: "INSTAGRAM", 3: "WHATSAPP", 4: "WALK_IN", 5: "REFERRAL", 6: "ALUMNI", 7: "OTHER"}[value]
}
func crmSourceFilterDB(value hajjv1.CRMLeadSource) string {
	if value == hajjv1.CRMLeadSource_CRM_LEAD_SOURCE_UNSPECIFIED {
		return ""
	}
	return crmSourceDB(value)
}
func crmSourceProto(value string) hajjv1.CRMLeadSource {
	return map[string]hajjv1.CRMLeadSource{"WEBSITE": 1, "INSTAGRAM": 2, "WHATSAPP": 3, "WALK_IN": 4, "REFERRAL": 5, "ALUMNI": 6, "OTHER": 7}[value]
}
func crmActivityDB(value hajjv1.CRMActivityKind) string {
	return map[hajjv1.CRMActivityKind]string{3: "CONTACT", 4: "NOTE", 5: "OFFER_SENT"}[value]
}
func crmActivityProto(value string) hajjv1.CRMActivityKind {
	return map[string]hajjv1.CRMActivityKind{"CREATED": 1, "STAGE_CHANGED": 2, "CONTACT": 3, "NOTE": 4, "OFFER_SENT": 5, "PROFILE_UPDATED": 6}[value]
}
