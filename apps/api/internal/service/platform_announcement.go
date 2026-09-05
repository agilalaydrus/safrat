package service

import (
	"context"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// announcementRecipientOverlapWindow is the readiness score's 24-hour check
// (§10.1 DESAIN) — a fixed, non-configurable window, same reasoning as
// TenantDeletionGraceDays: this is a product decision, not a setting to expose.
const announcementRecipientOverlapWindow = 24 * time.Hour

func recipientFilterFromProto(f *hajjv1.AnnouncementRecipientFilter) repository.RecipientFilter {
	if f == nil {
		return repository.RecipientFilter{}
	}
	return repository.RecipientFilter{Mode: f.Mode, Plan: f.Plan, OperatorIDs: f.OperatorIds}
}

func recipientFilterToProto(f repository.RecipientFilter) *hajjv1.AnnouncementRecipientFilter {
	return &hajjv1.AnnouncementRecipientFilter{Mode: f.Mode, Plan: f.Plan, OperatorIds: f.OperatorIDs}
}

func announcementMessage(a repository.Announcement) *hajjv1.Announcement {
	msg := &hajjv1.Announcement{
		Id: a.ID, Title: a.Title, Body: a.Body, Link: a.Link, Channels: a.Channels,
		RecipientFilter: recipientFilterToProto(a.RecipientFilter), RecipientCount: a.RecipientCount,
		ScheduledAt: timestamppb.New(a.ScheduledAt), CreatedAt: timestamppb.New(a.CreatedAt),
		ReadCount: a.ReadCount, AdminEmail: a.AdminEmail,
	}
	if a.SentAt != nil {
		msg.SentAt = timestamppb.New(*a.SentAt)
	}
	return msg
}

// PreviewAnnouncementRecipients answers step 1 of the wizard (§10.1 DESAIN):
// how many tenants this criteria matches right now, and how many of them
// already got a different announcement in the last 24 hours — computed from
// live data, never estimated, and never persisted (a preview commits nothing).
func (s *PlatformService) PreviewAnnouncementRecipients(ctx context.Context, req *hajjv1.PreviewAnnouncementRecipientsRequest) (*hajjv1.PreviewAnnouncementRecipientsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.Filter == nil {
		return nil, serviceError("PlatformService.PreviewAnnouncementRecipients", apperror.ErrValidation)
	}
	recipients, err := s.announcementRepository.ResolveRecipients(ctx, recipientFilterFromProto(req.Filter))
	if err != nil {
		return nil, serviceError("PlatformService.PreviewAnnouncementRecipients", err)
	}
	overlap, err := s.announcementRepository.OverlapWithRecentSends(ctx, recipients, announcementRecipientOverlapWindow)
	if err != nil {
		return nil, serviceError("PlatformService.PreviewAnnouncementRecipients", err)
	}
	return &hajjv1.PreviewAnnouncementRecipientsResponse{Count: int32(len(recipients)), OverlappingRecentCount: overlap}, nil
}

// SendAnnouncement stores the announcement and, when scheduled for now or
// the past, dispatches it in the same call — a future schedule is left for
// the worker sweep (internal/worker/announcement.go) to pick up, which
// re-resolves recipients at that moment rather than trusting this one.
func (s *PlatformService) SendAnnouncement(ctx context.Context, req *hajjv1.SendAnnouncementRequest) (*hajjv1.SendAnnouncementResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Body) == "" ||
		req.Filter == nil || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("PlatformService.SendAnnouncement", apperror.ErrValidation)
	}
	scheduledAt := time.Now()
	if req.ScheduledAt != nil && req.ScheduledAt.IsValid() {
		scheduledAt = req.ScheduledAt.AsTime()
	}
	announcement, err := s.announcementRepository.Create(ctx, userID, req.Title, req.Body, req.Link,
		req.Channels, recipientFilterFromProto(req.Filter), scheduledAt, strings.TrimSpace(req.IdempotencyKey))
	if err != nil {
		return nil, serviceError("PlatformService.SendAnnouncement", err)
	}
	if !scheduledAt.After(time.Now()) {
		announcement, err = s.announcementRepository.Dispatch(ctx, announcement.ID)
		if err != nil {
			return nil, serviceError("PlatformService.SendAnnouncement", err)
		}
	}
	return &hajjv1.SendAnnouncementResponse{Announcement: announcementMessage(announcement)}, nil
}

func (s *PlatformService) ListPlatformAnnouncements(ctx context.Context, req *hajjv1.ListPlatformAnnouncementsRequest) (*hajjv1.ListPlatformAnnouncementsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	limit := int32(50)
	if req != nil && req.Limit > 0 {
		limit = req.Limit
	}
	rows, err := s.announcementRepository.ListHistory(ctx, limit)
	if err != nil {
		return nil, serviceError("PlatformService.ListPlatformAnnouncements", err)
	}
	response := &hajjv1.ListPlatformAnnouncementsResponse{}
	for _, row := range rows {
		response.Announcements = append(response.Announcements, announcementMessage(row))
	}
	return response, nil
}
