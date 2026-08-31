package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/notification"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type LostReportService struct {
	operatorRepository    *repository.OperatorRepository
	pilgrimRepository     *repository.PilgrimRepository
	lostReportRepository  *repository.LostReportRepository
	groupLeaderRepository *repository.GroupLeaderRepository
	firebasePusher        *notification.FirebasePusher
}

func NewLostReportService(
	operators *repository.OperatorRepository,
	pilgrims *repository.PilgrimRepository,
	lostReports *repository.LostReportRepository,
	groupLeaders *repository.GroupLeaderRepository,
	firebasePusher *notification.FirebasePusher,
) *LostReportService {
	return &LostReportService{
		operatorRepository: operators, pilgrimRepository: pilgrims, lostReportRepository: lostReports,
		groupLeaderRepository: groupLeaders, firebasePusher: firebasePusher,
	}
}

// ReportLost is public — app_access_code is the only identity token.
// pilgrim_id/operator_id/group_id are always derived server-side from it,
// never trusted from the request body.
func (s *LostReportService) ReportLost(ctx context.Context, req *hajjv1.ReportLostRequest) (*hajjv1.LostReport, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("LostReportService.ReportLost", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("LostReportService.ReportLost", apperror.ErrNotFound)
	}
	pilgrim, err := s.pilgrimRepository.Get(ctx, info.OperatorID, info.ID)
	if err != nil {
		return nil, serviceError("LostReportService.ReportLost", err)
	}
	report, err := s.lostReportRepository.Create(ctx, info.ID, info.OperatorID, pilgrim.GroupID, req.Latitude, req.Longitude, req.LastKnownLocation)
	if err != nil {
		return nil, serviceError("LostReportService.ReportLost", err)
	}
	report.PilgrimName = info.FullName
	s.firebasePusher.NotifyLostReport(ctx, info.OperatorID, info.FullName)
	return lostReportMessage(report), nil
}

func (s *LostReportService) ListActive(ctx context.Context, authenticatedOrgID string) (*hajjv1.ListActiveLostReportsResponse, error) {
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("LostReportService.ListActive", err)
	}
	reports, err := s.lostReportRepository.ListActive(ctx, operator.ID)
	if err != nil {
		return nil, serviceError("LostReportService.ListActive", err)
	}
	result := &hajjv1.ListActiveLostReportsResponse{Reports: make([]*hajjv1.LostReport, 0, len(reports))}
	for _, report := range reports {
		result.Reports = append(result.Reports, lostReportMessage(report))
	}
	return result, nil
}

func (s *LostReportService) Resolve(ctx context.Context, authenticatedOrgID string, req *hajjv1.ResolveLostReportRequest) (*hajjv1.ResolveLostReportResponse, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("LostReportService.Resolve", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("LostReportService.Resolve", err)
	}
	if err := s.lostReportRepository.Resolve(ctx, operator.ID, req.Id); err != nil {
		return nil, serviceError("LostReportService.Resolve", err)
	}
	return &hajjv1.ResolveLostReportResponse{}, nil
}

// ResolveForGroup is the leader-facing resolve — EnsureLeaderOwnsGroup keeps
// a leader from resolving another leader's report by guessing an id, and
// ResolveForGroup's own group_id-scoped UPDATE enforces it at the DB level.
func (s *LostReportService) ResolveForGroup(ctx context.Context, authenticatedOrgID string, req *hajjv1.ResolveGroupLostReportRequest) (*hajjv1.ResolveLostReportResponse, error) {
	if req == nil || !isUUID(req.GroupId) || !isUUID(req.Id) {
		return nil, serviceError("LostReportService.ResolveForGroup", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("LostReportService.ResolveForGroup", err)
	}
	if err := s.groupLeaderRepository.EnsureLeaderOwnsGroup(ctx, operator.ID, req.GroupId, middleware.UserIDFromCtx(ctx)); err != nil {
		return nil, serviceError("LostReportService.ResolveForGroup", apperror.ErrForbidden)
	}
	if err := s.lostReportRepository.ResolveForGroup(ctx, operator.ID, req.GroupId, req.Id); err != nil {
		return nil, serviceError("LostReportService.ResolveForGroup", err)
	}
	return &hajjv1.ResolveLostReportResponse{}, nil
}

// ListForGroup is the leader-facing view — EnsureLeaderOwnsGroup keeps a
// leader from listing another leader's group by guessing a group_id.
func (s *LostReportService) ListForGroup(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListGroupLostReportsRequest) (*hajjv1.ListGroupLostReportsResponse, error) {
	if req == nil || !isUUID(req.GroupId) {
		return nil, serviceError("LostReportService.ListForGroup", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("LostReportService.ListForGroup", err)
	}
	if err := s.groupLeaderRepository.EnsureLeaderOwnsGroup(ctx, operator.ID, req.GroupId, middleware.UserIDFromCtx(ctx)); err != nil {
		return nil, serviceError("LostReportService.ListForGroup", apperror.ErrForbidden)
	}
	reports, err := s.lostReportRepository.ListForGroup(ctx, operator.ID, req.GroupId)
	if err != nil {
		return nil, serviceError("LostReportService.ListForGroup", err)
	}
	result := &hajjv1.ListGroupLostReportsResponse{Reports: make([]*hajjv1.LostReport, 0, len(reports))}
	for _, report := range reports {
		result.Reports = append(result.Reports, lostReportMessage(report))
	}
	return result, nil
}

func lostReportMessage(value *domain.LostReport) *hajjv1.LostReport {
	report := &hajjv1.LostReport{
		Id: value.ID, PilgrimId: value.PilgrimID, PilgrimName: value.PilgrimName, PilgrimPhone: value.PilgrimPhone,
		GroupName: value.GroupName, Latitude: value.Latitude, Longitude: value.Longitude,
		LastKnownLocation: value.LastKnownLocation, Status: value.Status, CreatedAt: timestamppb.New(value.CreatedAt),
	}
	if value.ResolvedAt != nil {
		report.ResolvedAt = timestamppb.New(*value.ResolvedAt)
	}
	return report
}
