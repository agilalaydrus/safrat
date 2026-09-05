package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
)

type NotificationSettingsService struct {
	operatorRepository *repository.OperatorRepository
	repository         *repository.NotificationSettingsRepository
}

func NewNotificationSettingsService(operators *repository.OperatorRepository, repo *repository.NotificationSettingsRepository) *NotificationSettingsService {
	return &NotificationSettingsService{operatorRepository: operators, repository: repo}
}

func minutesToClock(minutes int32) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

// clockToMinutes trusts the caller has already validated the "HH:MM" shape
// via protovalidate's regex — SetNotificationSettings is the only caller,
// and its request field carries that constraint.
func clockToMinutes(clock string) (int32, error) {
	parts := strings.SplitN(clock, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time %q", clock)
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	mins, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	return int32(hours*60 + mins), nil
}

func notificationSettingsMessage(s *domain.NotificationSettings) *hajjv1.NotificationSettings {
	return &hajjv1.NotificationSettings{
		QuietHoursEnabled: s.QuietHoursEnabled, QuietHoursStart: minutesToClock(s.QuietHoursStartMinutes), QuietHoursEnd: minutesToClock(s.QuietHoursEndMinutes),
		NotifyGroupCityChange: s.NotifyGroupCityChange, NotifyKloterStatusChange: s.NotifyKloterStatusChange, NotifyRitualBulkComplete: s.NotifyRitualBulkComplete,
	}
}

func (s *NotificationSettingsService) GetNotificationSettings(ctx context.Context, orgID string, req *hajjv1.GetNotificationSettingsRequest) (*hajjv1.NotificationSettings, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("NotificationSettingsService.GetNotificationSettings", err)
	}
	settings, err := s.repository.Get(ctx, op.ID)
	if err != nil {
		return nil, serviceError("NotificationSettingsService.GetNotificationSettings", err)
	}
	return notificationSettingsMessage(settings), nil
}

func (s *NotificationSettingsService) SetNotificationSettings(ctx context.Context, orgID string, req *hajjv1.SetNotificationSettingsRequest) (*hajjv1.NotificationSettings, error) {
	if req == nil {
		return nil, serviceError("NotificationSettingsService.SetNotificationSettings", apperror.ErrValidation)
	}
	start, err := clockToMinutes(req.QuietHoursStart)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	end, err := clockToMinutes(req.QuietHoursEnd)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("NotificationSettingsService.SetNotificationSettings", err)
	}
	settings, err := s.repository.Set(ctx, domain.NotificationSettings{
		OperatorID: op.ID, QuietHoursEnabled: req.QuietHoursEnabled, QuietHoursStartMinutes: start, QuietHoursEndMinutes: end,
		NotifyGroupCityChange: req.NotifyGroupCityChange, NotifyKloterStatusChange: req.NotifyKloterStatusChange, NotifyRitualBulkComplete: req.NotifyRitualBulkComplete,
	})
	if err != nil {
		return nil, serviceError("NotificationSettingsService.SetNotificationSettings", err)
	}
	return notificationSettingsMessage(settings), nil
}
