package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskCascadeDispatch = "cascade:dispatch"

const (
	// After cascadeMaxAttempts failed tries an event stops being claimed and
	// stays as a visible dead-letter row (last_error populated) for ops to
	// inspect, rather than looping forever.
	cascadeMaxAttempts = 5
	cascadeBatchLimit  = 50
)

func NewCascadeDispatchTask() *asynq.Task {
	return asynq.NewTask(TaskCascadeDispatch, nil)
}

// OperatorPusher is the slice of the push notifier the relay needs — kept as a
// local interface so the worker doesn't depend on the service package.
type OperatorPusher interface {
	NotifyOperatorStaff(ctx context.Context, operatorID, title, body string)
}

// OutboxHandler drains the cascade_events outbox and performs each event's
// side-effect. Delivery is at-least-once (claim increments attempts before the
// side-effect runs), so side-effects must be idempotent.
type OutboxHandler struct {
	logger *slog.Logger
	outbox *repository.OutboxRepository
	push   OperatorPusher
}

func NewOutboxHandler(logger *slog.Logger, outbox *repository.OutboxRepository, push OperatorPusher) *OutboxHandler {
	return &OutboxHandler{logger: logger, outbox: outbox, push: push}
}

func (h *OutboxHandler) HandleDispatch(ctx context.Context, _ *asynq.Task) error {
	events, err := h.outbox.Claim(ctx, cascadeMaxAttempts, cascadeBatchLimit)
	if err != nil {
		return err
	}
	for _, ev := range events {
		if err := h.dispatch(ctx, ev); err != nil {
			h.logger.Error("cascade dispatch failed", "id", ev.ID, "type", ev.EventType, "attempts", ev.Attempts, "error", err)
			_ = h.outbox.MarkFailed(ctx, ev.ID, err.Error())
			continue
		}
		if err := h.outbox.MarkProcessed(ctx, ev.ID); err != nil {
			h.logger.Error("cascade mark processed", "id", ev.ID, "error", err)
		}
	}
	return nil
}

func (h *OutboxHandler) dispatch(ctx context.Context, ev domain.CascadeEvent) error {
	switch ev.EventType {
	case domain.EventHealthReportCreated:
		var payload domain.HealthReportCreatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err
		}
		if payload.Severity == "BERAT" && h.push != nil {
			h.push.NotifyOperatorStaff(ctx, ev.OperatorID, "⚠ Laporan Kesehatan BERAT", fmt.Sprintf("%s — perlu perhatian segera.", payload.PilgrimName))
		}
		return nil
	default:
		// Unknown type — don't retry forever; record and move on.
		h.logger.Warn("cascade unknown event type", "id", ev.ID, "type", ev.EventType)
		return nil
	}
}
