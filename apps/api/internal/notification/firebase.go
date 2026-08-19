// Package notification sends Firebase push notifications. It's kept separate
// from the service package so service.SOSService only depends on the small
// SOSNotifier interface, not the Firebase SDK directly.
package notification

import (
	"context"
	"log/slog"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/api/option"
)

// FirebasePusher sends SOS alerts to every registered coordinator/leader
// device for an operator. A nil *messaging.Client (unset
// FIREBASE_SERVICE_ACCOUNT_JSON) makes every call a logged no-op — this must
// never block or fail SOS creation itself.
type FirebasePusher struct {
	logger *slog.Logger
	client *messaging.Client
	tokens *repository.NotificationRepository
}

// NewFirebasePusher returns nil, nil when serviceAccountJSON is empty —
// callers should treat a nil *FirebasePusher as "no notifier" (it still
// satisfies service.SOSNotifier as a no-op via the nil-receiver methods below).
func NewFirebasePusher(ctx context.Context, logger *slog.Logger, serviceAccountJSON string, tokens *repository.NotificationRepository) (*FirebasePusher, error) {
	if serviceAccountJSON == "" {
		return nil, nil
	}
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(serviceAccountJSON)))
	if err != nil {
		return nil, err
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}
	return &FirebasePusher{logger: logger, client: client, tokens: tokens}, nil
}

func (p *FirebasePusher) NotifySOSAlert(ctx context.Context, operatorID string, alert *domain.SOSAlert) {
	if p == nil || p.client == nil {
		return
	}
	tokens, err := p.tokens.ListTokensForOperator(ctx, operatorID)
	if err != nil {
		p.logger.Error("list push tokens", "error", err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	title, body := sosNotificationText(alert)
	response, err := p.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{
		Tokens:       tokens,
		Notification: &messaging.Notification{Title: title, Body: body},
	})
	if err != nil {
		p.logger.Error("send SOS push", "error", err)
		return
	}
	if response.FailureCount > 0 {
		p.logger.Warn("SOS push partial failure", "failure_count", response.FailureCount, "success_count", response.SuccessCount)
	}
}

// NotifyLostReport pushes to every registered coordinator/leader device for
// the operator — same broadcast scope as NotifySOSAlert. There's no
// per-leader token targeting in this system (tokens are keyed to operator,
// not to a specific leader/group), so a leader learns about their own
// group's report the same way any coordinator does: via this broadcast,
// then filters to their group in the leader app UI.
func (p *FirebasePusher) NotifyLostReport(ctx context.Context, operatorID, pilgrimName string) {
	if p == nil || p.client == nil {
		return
	}
	tokens, err := p.tokens.ListTokensForOperator(ctx, operatorID)
	if err != nil {
		p.logger.Error("list push tokens", "error", err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	response, err := p.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{
		Tokens:       tokens,
		Notification: &messaging.Notification{Title: "🟡 Jamaah Terpisah", Body: pilgrimName + " melaporkan diri terpisah dari rombongan."},
	})
	if err != nil {
		p.logger.Error("send lost report push", "error", err)
		return
	}
	if response.FailureCount > 0 {
		p.logger.Warn("lost report push partial failure", "failure_count", response.FailureCount, "success_count", response.SuccessCount)
	}
}

func sosNotificationText(alert *domain.SOSAlert) (string, string) {
	if alert.Status == "ESCALATED" {
		return "SOS ESCALATED", alert.PilgrimName + " has not been acknowledged in 10 minutes."
	}
	return "SOS Alert", alert.PilgrimName + " needs help."
}
