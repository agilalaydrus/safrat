package repository

import (
	"context"
	"errors"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type NotificationSettingsRepository struct {
	queries *db.Queries
}

func NewNotificationSettingsRepository(queries *db.Queries) *NotificationSettingsRepository {
	return &NotificationSettingsRepository{queries: queries}
}

func minutesToPgTime(minutes int32) pgtype.Time {
	return pgtype.Time{Microseconds: int64(minutes) * 60 * 1_000_000, Valid: true}
}

func pgTimeToMinutes(t pgtype.Time) int32 {
	return int32(t.Microseconds / 1_000_000 / 60)
}

// Get returns the zero-value defaults (quiet hours off, every event on)
// when the operator has never configured this — the common case.
func (r *NotificationSettingsRepository) Get(ctx context.Context, operatorID string) (*domain.NotificationSettings, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.GetOperatorNotificationSettings(ctx, op)
	if errors.Is(err, pgx.ErrNoRows) {
		return &domain.NotificationSettings{
			OperatorID: operatorID, QuietHoursStartMinutes: 22 * 60, QuietHoursEndMinutes: 6 * 60,
			NotifyGroupCityChange: true, NotifyKloterStatusChange: true, NotifyRitualBulkComplete: true,
		}, nil
	}
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.NotificationSettings{
		OperatorID: operatorID, QuietHoursEnabled: row.QuietHoursEnabled,
		QuietHoursStartMinutes: pgTimeToMinutes(row.QuietHoursStart), QuietHoursEndMinutes: pgTimeToMinutes(row.QuietHoursEnd),
		NotifyGroupCityChange: row.NotifyGroupCityChange, NotifyKloterStatusChange: row.NotifyKloterStatusChange,
		NotifyRitualBulkComplete: row.NotifyRitualBulkComplete,
	}, nil
}

func (r *NotificationSettingsRepository) Set(ctx context.Context, s domain.NotificationSettings) (*domain.NotificationSettings, error) {
	op, err := pgUUID(s.OperatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpsertOperatorNotificationSettings(ctx, db.UpsertOperatorNotificationSettingsParams{
		OperatorID: op, QuietHoursEnabled: s.QuietHoursEnabled,
		QuietHoursStart: minutesToPgTime(s.QuietHoursStartMinutes), QuietHoursEnd: minutesToPgTime(s.QuietHoursEndMinutes),
		NotifyGroupCityChange: s.NotifyGroupCityChange, NotifyKloterStatusChange: s.NotifyKloterStatusChange,
		NotifyRitualBulkComplete: s.NotifyRitualBulkComplete,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.NotificationSettings{
		OperatorID: s.OperatorID, QuietHoursEnabled: row.QuietHoursEnabled,
		QuietHoursStartMinutes: pgTimeToMinutes(row.QuietHoursStart), QuietHoursEndMinutes: pgTimeToMinutes(row.QuietHoursEnd),
		NotifyGroupCityChange: row.NotifyGroupCityChange, NotifyKloterStatusChange: row.NotifyKloterStatusChange,
		NotifyRitualBulkComplete: row.NotifyRitualBulkComplete,
	}, nil
}
