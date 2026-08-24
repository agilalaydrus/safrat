package repository

import (
	"context"
	"errors"

	db "github.com/hajj-saas/api/internal/gen/db"
)

type NotificationRepository struct{ queries *db.Queries }

func NewNotificationRepository(queries *db.Queries) *NotificationRepository {
	return &NotificationRepository{queries: queries}
}

func (r *NotificationRepository) DeleteInvalidToken(ctx context.Context, operatorID, fcmToken string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	params := db.DeletePushSubscriptionTokenParams{OperatorID: opUUID, FcmToken: fcmToken}
	staffErr := r.queries.DeletePushSubscriptionToken(ctx, params)
	pilgrimErr := r.queries.DeletePilgrimPushToken(ctx, db.DeletePilgrimPushTokenParams(params))
	return errors.Join(staffErr, pilgrimErr)
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

// RegisterPilgrimToken is the pilgrim-app counterpart of RegisterToken —
// pilgrim_push_tokens is keyed to pilgrim_id, not a Better Auth user_id,
// since most of the pilgrim PWA is public (app_access_code), no guaranteed
// session.
func (r *NotificationRepository) RegisterPilgrimToken(ctx context.Context, operatorID, pilgrimID, fcmToken string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return err
	}
	return r.queries.UpsertPilgrimPushToken(ctx, db.UpsertPilgrimPushTokenParams{OperatorID: opUUID, PilgrimID: pilgrimUUID, FcmToken: fcmToken})
}

func (r *NotificationRepository) ListTokensForGroup(ctx context.Context, operatorID, groupID string) ([]string, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return nil, err
	}
	return r.queries.ListPushTokensForGroup(ctx, db.ListPushTokensForGroupParams{OperatorID: opUUID, GroupID: groupUUID})
}

func (r *NotificationRepository) ListTokensForKloter(ctx context.Context, operatorID, kloterID string) ([]string, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return nil, err
	}
	return r.queries.ListPushTokensForKloter(ctx, db.ListPushTokensForKloterParams{OperatorID: opUUID, KloterID: kloterUUID})
}
