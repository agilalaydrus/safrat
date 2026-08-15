package repository

import (
	"context"

	db "github.com/hajj-saas/api/internal/gen/db"
)

type NotificationRepository struct{ queries *db.Queries }

func NewNotificationRepository(queries *db.Queries) *NotificationRepository {
	return &NotificationRepository{queries: queries}
}

func (r *NotificationRepository) RegisterToken(ctx context.Context, operatorID, userID, fcmToken string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	_, err = r.queries.UpsertPushSubscription(ctx, db.UpsertPushSubscriptionParams{OperatorID: opUUID, UserID: userID, FcmToken: fcmToken})
	return err
}

func (r *NotificationRepository) ListTokensForOperator(ctx context.Context, operatorID string) ([]string, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	return r.queries.ListPushTokensForOperator(ctx, opUUID)
}
