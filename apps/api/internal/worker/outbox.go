package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/events"
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
	NotifyOperatorStaff(ctx context.Context, operatorID, title, body string) error
	NotifyGroupPilgrims(ctx context.Context, operatorID, groupID, branchID, title, body string) error
	NotifyKloterPilgrims(ctx context.Context, operatorID, kloterID, title, body string) error
}

type JourneyCascader interface {
	BulkUpdateForGroupAs(ctx context.Context, operatorID, groupID, status, updatedByUserID, notes string) (int32, error)
	BulkUpdateForKloterAs(ctx context.Context, operatorID, kloterID, status, updatedByUserID, notes string) (int32, error)
}

// OutboxHandler drains the cascade_events outbox and performs each event's
// side-effect. Delivery is at-least-once (claim increments attempts before the
// side-effect runs), so side-effects must be idempotent.
type OutboxHandler struct {
	logger   *slog.Logger
	outbox   *repository.OutboxRepository
	push     OperatorPusher
	journeys JourneyCascader
	eventBus *events.Bus
}

func NewOutboxHandler(logger *slog.Logger, outbox *repository.OutboxRepository, push OperatorPusher, journeys JourneyCascader, bus *events.Bus) *OutboxHandler {
	return &OutboxHandler{logger: logger, outbox: outbox, push: push, journeys: journeys, eventBus: bus}
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
			return h.push.NotifyOperatorStaff(ctx, ev.OperatorID, "⚠ Laporan Kesehatan BERAT", fmt.Sprintf("%s — perlu perhatian segera.", payload.PilgrimName))
		}
		return nil
	case domain.EventGroupCityUpdated:
		var payload domain.GroupCityUpdatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err
		}
		if payload.GroupID == "" {
			return fmt.Errorf("group city event missing group_id")
		}
		if payload.JourneyStatus != "" && h.journeys != nil {
			if _, err := h.journeys.BulkUpdateForGroupAs(ctx, ev.OperatorID, payload.GroupID, payload.JourneyStatus, payload.UpdatedBy, payload.Notes); err != nil {
				return err
			}
		}
		if payload.NotificationBody != "" && h.push != nil {
			if err := h.push.NotifyGroupPilgrims(ctx, ev.OperatorID, payload.GroupID, "", "Tawafiq Hub", payload.NotificationBody); err != nil {
				return err
			}
		}
		h.eventBus.Publish(ev.OperatorID, "journey", payload.GroupID)
		return nil
	case domain.EventKloterStatusUpdated:
		var payload domain.KloterStatusUpdatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err
		}
		if payload.KloterID == "" {
			return fmt.Errorf("kloter status event missing kloter_id")
		}
		if payload.JourneyStatus != "" && h.journeys != nil {
			notes := fmt.Sprintf("Cascade dari kloter %s -> %s", payload.KloterCode, payload.Status)
			if _, err := h.journeys.BulkUpdateForKloterAs(ctx, ev.OperatorID, payload.KloterID, payload.JourneyStatus, payload.UpdatedBy, notes); err != nil {
				return err
			}
		}
		if payload.NotificationBody != "" && h.push != nil {
			if err := h.push.NotifyKloterPilgrims(ctx, ev.OperatorID, payload.KloterID, "Tawafiq Hub", payload.NotificationBody); err != nil {
				return err
			}
		}
		h.eventBus.Publish(ev.OperatorID, "journey", payload.KloterID)
		return nil
	case domain.EventRitualBulkCompleted:
		var payload domain.RitualBulkCompletedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err
		}
		if payload.GroupID == "" {
			return fmt.Errorf("ritual bulk event missing group_id")
		}
		if payload.NotificationBody != "" && h.push != nil {
			if err := h.push.NotifyGroupPilgrims(ctx, ev.OperatorID, payload.GroupID, payload.BranchID, "Tawafiq Hub", payload.NotificationBody); err != nil {
				return err
			}
		}
		h.eventBus.Publish(ev.OperatorID, "ritual", payload.GroupID)
		return nil
	default:
		return fmt.Errorf("unsupported cascade event type %q", ev.EventType)
	}
}
