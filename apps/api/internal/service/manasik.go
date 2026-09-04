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

type ManasikService struct {
	operatorRepository *repository.OperatorRepository
	manasikRepository  *repository.ManasikRepository
}

func NewManasikService(operators *repository.OperatorRepository, manasik *repository.ManasikRepository) *ManasikService {
	return &ManasikService{operatorRepository: operators, manasikRepository: manasik}
}

func manasikCurriculumMessage(c *domain.ManasikCurriculum) *hajjv1.ManasikCurriculum {
	return &hajjv1.ManasikCurriculum{Id: c.ID, SeasonId: c.SeasonID, Title: c.Title, Description: c.Description, SortOrder: c.SortOrder}
}

func (s *ManasikService) CreateCurriculum(ctx context.Context, orgID string, req *hajjv1.CreateManasikCurriculumRequest) (*hajjv1.ManasikCurriculum, error) {
	if req == nil || !isUUID(req.SeasonId) || strings.TrimSpace(req.Title) == "" {
		return nil, serviceError("ManasikService.CreateCurriculum", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ManasikService.CreateCurriculum", err)
	}
	c, err := s.manasikRepository.CreateCurriculum(ctx, op.ID, req.SeasonId, strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), req.SortOrder)
	if err != nil {
		return nil, serviceError("ManasikService.CreateCurriculum", err)
	}
	return manasikCurriculumMessage(c), nil
}

func (s *ManasikService) UpdateCurriculum(ctx context.Context, orgID string, req *hajjv1.UpdateManasikCurriculumRequest) (*hajjv1.ManasikCurriculum, error) {
	if req == nil || strings.TrimSpace(req.CurriculumId) == "" || strings.TrimSpace(req.Title) == "" {
		return nil, serviceError("ManasikService.UpdateCurriculum", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ManasikService.UpdateCurriculum", err)
	}
	c, err := s.manasikRepository.UpdateCurriculum(ctx, op.ID, req.CurriculumId, strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), req.SortOrder)
	if err != nil {
		return nil, serviceError("ManasikService.UpdateCurriculum", err)
	}
	return manasikCurriculumMessage(c), nil
}

func (s *ManasikService) DeleteCurriculum(ctx context.Context, orgID string, req *hajjv1.DeleteManasikCurriculumRequest) error {
	if req == nil || strings.TrimSpace(req.CurriculumId) == "" {
		return serviceError("ManasikService.DeleteCurriculum", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("ManasikService.DeleteCurriculum", err)
	}
	if err := s.manasikRepository.DeleteCurriculum(ctx, op.ID, req.CurriculumId); err != nil {
		return serviceError("ManasikService.DeleteCurriculum", err)
	}
	return nil
}

func (s *ManasikService) ListCurricula(ctx context.Context, orgID string, req *hajjv1.ListManasikCurriculaRequest) (*hajjv1.ListManasikCurriculaResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("ManasikService.ListCurricula", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ManasikService.ListCurricula", err)
	}
	curricula, err := s.manasikRepository.ListCurricula(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("ManasikService.ListCurricula", err)
	}
	response := &hajjv1.ListManasikCurriculaResponse{}
	for _, c := range curricula {
		response.Curricula = append(response.Curricula, manasikCurriculumMessage(c))
	}
	return response, nil
}

func manasikSessionMessage(sess *domain.ManasikSession) *hajjv1.ManasikSession {
	return &hajjv1.ManasikSession{
		Id: sess.ID, SeasonId: sess.SeasonID, CurriculumId: sess.CurriculumID, CurriculumTitle: sess.CurriculumTitle,
		KloterId: sess.KloterID, KloterCode: sess.KloterCode, Title: sess.Title, Location: sess.Location,
		InstructorName: sess.InstructorName, ScheduledAt: timestamppb.New(sess.ScheduledAt),
		DurationMinutes: sess.DurationMinutes, Capacity: sess.Capacity, Notes: sess.Notes, Status: sess.Status,
	}
}

func (s *ManasikService) CreateSession(ctx context.Context, orgID string, req *hajjv1.CreateManasikSessionRequest) (*hajjv1.ManasikSession, error) {
	if req == nil || !isUUID(req.SeasonId) || strings.TrimSpace(req.Title) == "" || req.ScheduledAt == nil {
		return nil, serviceError("ManasikService.CreateSession", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ManasikService.CreateSession", err)
	}
	duration := req.DurationMinutes
	if duration <= 0 {
		duration = 60
	}
	sess, err := s.manasikRepository.CreateSession(ctx, op.ID, req.SeasonId, strings.TrimSpace(req.CurriculumId), strings.TrimSpace(req.KloterId),
		strings.TrimSpace(req.Title), strings.TrimSpace(req.Location), strings.TrimSpace(req.InstructorName),
		req.ScheduledAt.AsTime(), duration, req.Capacity, strings.TrimSpace(req.Notes))
	if err != nil {
		return nil, serviceError("ManasikService.CreateSession", err)
	}
	return manasikSessionMessage(sess), nil
}

func (s *ManasikService) UpdateSession(ctx context.Context, orgID string, req *hajjv1.UpdateManasikSessionRequest) (*hajjv1.ManasikSession, error) {
	if req == nil || strings.TrimSpace(req.SessionId) == "" || strings.TrimSpace(req.Title) == "" || req.ScheduledAt == nil {
		return nil, serviceError("ManasikService.UpdateSession", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ManasikService.UpdateSession", err)
	}
	duration := req.DurationMinutes
	if duration <= 0 {
		duration = 60
	}
	sess, err := s.manasikRepository.UpdateSession(ctx, op.ID, req.SessionId, strings.TrimSpace(req.CurriculumId), strings.TrimSpace(req.KloterId),
		strings.TrimSpace(req.Title), strings.TrimSpace(req.Location), strings.TrimSpace(req.InstructorName),
		req.ScheduledAt.AsTime(), duration, req.Capacity, strings.TrimSpace(req.Notes))
	if err != nil {
		return nil, serviceError("ManasikService.UpdateSession", err)
	}
	return manasikSessionMessage(sess), nil
}

func (s *ManasikService) UpdateSessionStatus(ctx context.Context, orgID string, req *hajjv1.UpdateManasikSessionStatusRequest) (*hajjv1.ManasikSession, error) {
	if req == nil || strings.TrimSpace(req.SessionId) == "" || strings.TrimSpace(req.Status) == "" {
		return nil, serviceError("ManasikService.UpdateSessionStatus", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ManasikService.UpdateSessionStatus", err)
	}
	sess, err := s.manasikRepository.UpdateSessionStatus(ctx, op.ID, req.SessionId, strings.ToUpper(strings.TrimSpace(req.Status)))
	if err != nil {
		return nil, serviceError("ManasikService.UpdateSessionStatus", err)
	}
	return manasikSessionMessage(sess), nil
}

func (s *ManasikService) DeleteSession(ctx context.Context, orgID string, req *hajjv1.DeleteManasikSessionRequest) error {
	if req == nil || strings.TrimSpace(req.SessionId) == "" {
		return serviceError("ManasikService.DeleteSession", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("ManasikService.DeleteSession", err)
	}
	if err := s.manasikRepository.DeleteSession(ctx, op.ID, req.SessionId); err != nil {
		return serviceError("ManasikService.DeleteSession", err)
	}
	return nil
}

func (s *ManasikService) ListSessions(ctx context.Context, orgID string, req *hajjv1.ListManasikSessionsRequest) (*hajjv1.ListManasikSessionsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("ManasikService.ListSessions", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ManasikService.ListSessions", err)
	}
	sessions, err := s.manasikRepository.ListSessions(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("ManasikService.ListSessions", err)
	}
	response := &hajjv1.ListManasikSessionsResponse{}
	for _, sess := range sessions {
		response.Sessions = append(response.Sessions, manasikSessionMessage(sess))
	}
	return response, nil
}

var validAttendanceStatus = map[string]bool{"PRESENT": true, "ABSENT": true, "EXCUSED": true}

func (s *ManasikService) RecordAttendance(ctx context.Context, orgID string, req *hajjv1.RecordManasikAttendanceRequest) error {
	if req == nil || strings.TrimSpace(req.SessionId) == "" || strings.TrimSpace(req.PilgrimId) == "" {
		return serviceError("ManasikService.RecordAttendance", apperror.ErrValidation)
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if !validAttendanceStatus[status] {
		return serviceError("ManasikService.RecordAttendance", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("ManasikService.RecordAttendance", err)
	}
	if err := s.manasikRepository.RecordAttendance(ctx, op.ID, req.SessionId, req.PilgrimId, status, strings.TrimSpace(req.Notes)); err != nil {
		return serviceError("ManasikService.RecordAttendance", err)
	}
	return nil
}

func (s *ManasikService) ListAttendance(ctx context.Context, orgID string, req *hajjv1.ListManasikAttendanceRequest) (*hajjv1.ListManasikAttendanceResponse, error) {
	if req == nil || strings.TrimSpace(req.SessionId) == "" {
		return nil, serviceError("ManasikService.ListAttendance", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ManasikService.ListAttendance", err)
	}
	rows, err := s.manasikRepository.ListAttendance(ctx, op.ID, req.SessionId)
	if err != nil {
		return nil, serviceError("ManasikService.ListAttendance", err)
	}
	summary, err := s.manasikRepository.AttendanceSummary(ctx, op.ID, req.SessionId)
	if err != nil {
		return nil, serviceError("ManasikService.ListAttendance", err)
	}
	response := &hajjv1.ListManasikAttendanceResponse{
		PresentCount: summary.PresentCount, AbsentCount: summary.AbsentCount, ExcusedCount: summary.ExcusedCount,
	}
	for _, row := range rows {
		response.Rows = append(response.Rows, &hajjv1.ManasikAttendanceRow{
			Id: row.ID, PilgrimId: row.PilgrimID, PilgrimName: row.PilgrimName, PassportNumber: row.PassportNumber,
			Status: row.Status, Notes: row.Notes,
		})
	}
	return response, nil
}
