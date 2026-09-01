// Package notification sends Firebase push notifications. It's kept separate
// from the service package so service.SOSService only depends on the small
// SOSNotifier interface, not the Firebase SDK directly.
package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

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

func (p *FirebasePusher) NotifySOSAlert(ctx context.Context, operatorID string, alert *domain.SOSAlert) error {
	if p == nil || p.client == nil {
		return nil
	}
	tokens, err := p.tokens.ListTokensForOperator(ctx, operatorID)
	if err != nil {
		return fmt.Errorf("list SOS push tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}
	title, body := sosNotificationText(alert)
	return p.send(ctx, operatorID, tokens, title, body)
}

// send performs a bounded, low-latency retry for transport-level Firebase
// failures. After a partial response only transiently failed tokens are
// retried, so successful recipients do not get duplicates. Unregistered
// tokens are removed from both staff and pilgrim token stores.
func (p *FirebasePusher) send(ctx context.Context, operatorID string, tokens []string, title, body string) error {
	if p == nil || p.client == nil || len(tokens) == 0 {
		return nil
	}
	retryCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	delays := [...]time.Duration{0, 100 * time.Millisecond, 250 * time.Millisecond}
	var lastErr error
	pending := tokens
	for attempt, delay := range delays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-retryCtx.Done():
				timer.Stop()
				return errors.Join(lastErr, retryCtx.Err())
			case <-timer.C:
			}
		}
		response, err := p.client.SendEachForMulticast(retryCtx, &messaging.MulticastMessage{
			Tokens:       pending,
			Notification: &messaging.Notification{Title: title, Body: body},
		})
		if err == nil {
			retryTokens := make([]string, 0, response.FailureCount)
			for index, result := range response.Responses {
				if result.Success || result.Error == nil {
					continue
				}
				token := pending[index]
				if messaging.IsUnregistered(result.Error) {
					if deleteErr := p.tokens.DeleteInvalidToken(retryCtx, operatorID, token); deleteErr != nil {
						p.logger.Warn("delete unregistered push token", "error", deleteErr)
					}
					continue
				}
				if messaging.IsInvalidArgument(result.Error) || messaging.IsSenderIDMismatch(result.Error) || messaging.IsThirdPartyAuthError(result.Error) {
					p.logger.Warn("permanent push failure", "error", result.Error, "title", title)
					continue
				}
				retryTokens = append(retryTokens, token)
			}
			if len(retryTokens) == 0 {
				return nil
			}
			pending = retryTokens
			lastErr = fmt.Errorf("%d transient per-token push failures", len(retryTokens))
			p.logger.Warn("push partial transient failure", "attempt", attempt+1, "retry_count", len(retryTokens), "success_count", response.SuccessCount, "title", title)
			continue
		}
		lastErr = err
		p.logger.Warn("push attempt failed", "attempt", attempt+1, "max_attempts", len(delays), "error", err, "title", title)
	}
	return fmt.Errorf("send push after %d attempts: %w", len(delays), lastErr)
}

// NotifyLostReport pushes to every registered coordinator/leader device for
// the operator — same broadcast scope as NotifySOSAlert. There's no
// per-leader token targeting in this system (tokens are keyed to operator,
// not to a specific leader/group), so a leader learns about their own
// group's report the same way any coordinator does: via this broadcast,
// then filters to their group in the leader app UI.
func (p *FirebasePusher) NotifyLostReport(ctx context.Context, operatorID, pilgrimName string) error {
	if p == nil || p.client == nil {
		return nil
	}
	tokens, err := p.tokens.ListTokensForOperator(ctx, operatorID)
	if err != nil {
		return fmt.Errorf("list lost-report push tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}
	return p.send(ctx, operatorID, tokens, "🟡 Jamaah Terpisah", pilgrimName+" melaporkan diri terpisah dari rombongan.")
}

// NotifyOperatorStaff broadcasts to every registered coordinator/leader
// device for the operator — same scope as NotifySOSAlert/NotifyLostReport,
// generalized for cascade events (health BERAT, kloter milestones the
// operator should know about even without watching the dashboard).
func (p *FirebasePusher) NotifyOperatorStaff(ctx context.Context, operatorID, title, body string) error {
	if p == nil || p.client == nil {
		return nil
	}
	tokens, err := p.tokens.ListTokensForOperator(ctx, operatorID)
	if err != nil {
		return fmt.Errorf("list operator push tokens: %w", err)
	}
	return p.send(ctx, operatorID, tokens, title, body)
}

// NotifyGroupPilgrims pushes to every pilgrim in the group who has
// registered a device (pilgrim_push_tokens) — used for the Muttawwif's
// location-update cascade ("Rombongan Anda kini di Makkah").
func (p *FirebasePusher) NotifyGroupPilgrims(ctx context.Context, operatorID, groupID, branchID, title, body string) error {
	if p == nil || p.client == nil {
		return nil
	}
	tokens, err := p.tokens.ListTokensForGroup(ctx, operatorID, groupID, branchID)
	if err != nil {
		return fmt.Errorf("list group push tokens: %w", err)
	}
	return p.send(ctx, operatorID, tokens, title, body)
}

// NotifyKloterPilgrims is the kloter-wide equivalent, used by
// KloterService's status-change cascade ("Perjalanan ibadah Anda dimulai").
func (p *FirebasePusher) NotifyKloterPilgrims(ctx context.Context, operatorID, kloterID, title, body string) error {
	if p == nil || p.client == nil {
		return nil
	}
	tokens, err := p.tokens.ListTokensForKloter(ctx, operatorID, kloterID)
	if err != nil {
		return fmt.Errorf("list kloter push tokens: %w", err)
	}
	return p.send(ctx, operatorID, tokens, title, body)
}

func sosNotificationText(alert *domain.SOSAlert) (string, string) {
	if alert.Status == "ESCALATED" {
		return "SOS ESCALATED", alert.PilgrimName + " has not been acknowledged in 10 minutes."
	}
	return "SOS Alert", alert.PilgrimName + " needs help."
}
