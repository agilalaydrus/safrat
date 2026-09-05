package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AnnouncementService is the operator-facing half of Pengumuman (E2,
// TUGAS-PANEL-SAAS.md §10.1 DESAIN): reading what the platform actually sent
// to this tenant, and marking it read. Composing and sending lives on
// PlatformService — see platform_announcement.go — the same split
// SupportService uses.
type AnnouncementService struct {
	operatorRepository     *repository.OperatorRepository
	announcementRepository *repository.AnnouncementRepository
}

func NewAnnouncementService(operators *repository.OperatorRepository, announcements *repository.AnnouncementRepository) *AnnouncementService {
	return &AnnouncementService{operatorRepository: operators, announcementRepository: announcements}
}

func operatorAnnouncementMessage(a repository.OperatorAnnouncement) *hajjv1.OperatorAnnouncement {
	msg := &hajjv1.OperatorAnnouncement{
		Id: a.ID, Title: a.Title, Body: a.Body, Link: a.Link, SentAt: timestamppb.New(a.SentAt),
	}
	if a.ReadAt != nil {
		msg.ReadAt = timestamppb.New(*a.ReadAt)
	}
	return msg
}

func (s *AnnouncementService) ListMyAnnouncements(ctx context.Context, orgID string, req *hajjv1.ListMyAnnouncementsRequest) (*hajjv1.ListMyAnnouncementsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AnnouncementService.ListMyAnnouncements", err)
	}
	rows, err := s.announcementRepository.ListForOperator(ctx, op.ID, 30)
	if err != nil {
		return nil, serviceError("AnnouncementService.ListMyAnnouncements", err)
	}
	response := &hajjv1.ListMyAnnouncementsResponse{}
	for _, row := range rows {
		response.Announcements = append(response.Announcements, operatorAnnouncementMessage(row))
	}
	return response, nil
}

func (s *AnnouncementService) MarkAnnouncementRead(ctx context.Context, orgID, userID string, req *hajjv1.MarkAnnouncementReadRequest) (*hajjv1.MarkAnnouncementReadResponse, error) {
	if req == nil || strings.TrimSpace(req.AnnouncementId) == "" {
		return nil, serviceError("AnnouncementService.MarkAnnouncementRead", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AnnouncementService.MarkAnnouncementRead", err)
	}
	if err := s.announcementRepository.MarkRead(ctx, op.ID, req.AnnouncementId, userID); err != nil {
		return nil, serviceError("AnnouncementService.MarkAnnouncementRead", err)
	}
	return &hajjv1.MarkAnnouncementReadResponse{}, nil
}
