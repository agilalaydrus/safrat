package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/queue"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/hibiken/asynq"
)

const (
	TaskFulfilmentSweep    = "fulfilment:sweep"
	TaskFulfilmentDispatch = "fulfilment:dispatch"
)

// stuckAfter is how long a fulfilment may sit unanswered before somebody should
// be told. Suppliers here normally answer in seconds; an hour is generous
// enough that a slow one does not raise an alarm, and short enough that a
// jamaah is not waiting a day for pulsa nobody noticed was missing.
const stuckAfter = time.Hour

func NewFulfilmentSweepTask() *asynq.Task {
	return asynq.NewTask(TaskFulfilmentSweep, nil)
}

func NewFulfilmentDispatchTask() *asynq.Task {
	return asynq.NewTask(TaskFulfilmentDispatch, nil)
}

// HandleDispatchOne sends one named order immediately.
//
// This is the path that actually meets a digital product's latency: enqueued
// the moment a payment settles, picked up in milliseconds. The periodic sweep
// below is the net underneath it, not the mechanism.
//
// Finding nothing to send is a normal outcome, not a failure — the sweep may
// have reached the same order first, and whichever arrives second should
// simply stop.
func (h *FulfilmentHandler) HandleDispatchOne(ctx context.Context, task *asynq.Task) error {
	var payload queue.DispatchPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		// A payload we cannot read will never become readable, so retrying is
		// pointless; asynq is told to stop rather than to keep trying.
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	pending, err := h.suppliers.PendingDispatchFor(ctx, payload.OrderID)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	}
	return h.service.Dispatch(ctx, pending)
}

// FulfilmentHandler watches the transactions that need a human.
//
// Two kinds end up here, and both are deliberate consequences of decisions made
// elsewhere:
//
//   - NEEDS_REVIEW — a supplier said something no rule recognised. Refusing to
//     read that as failure is right: refunding a jamaah for something the
//     supplier may well have delivered is worse and irreversible. The price is
//     that it waits for a person.
//   - SENT and silent — the supplier never answered at all.
//
// Without this sweep, both wait forever behind a screen nobody has a reason to
// open. That was the gap: the decision to hold rather than refund is only
// defensible if somebody is actually told to look.
type FulfilmentHandler struct {
	logger      *slog.Logger
	fulfilments *repository.FulfilmentRepository
	suppliers   *repository.SupplierRepository
	service     *service.FulfilmentService
}

func NewFulfilmentHandler(logger *slog.Logger, fulfilments *repository.FulfilmentRepository,
	suppliers *repository.SupplierRepository, fulfilmentService *service.FulfilmentService) *FulfilmentHandler {
	return &FulfilmentHandler{logger: logger, fulfilments: fulfilments, suppliers: suppliers, service: fulfilmentService}
}

// HandleDispatch sends fulfilments that have never been sent.
//
// Deliberately a sweep rather than a queued job per order. A job enqueued at
// payment time is lost if the enqueue fails or the queue is drained, and the
// jamaah is left waiting with nothing to notice it; a sweep re-reads the truth
// from the database every pass, so anything unsent is eventually picked up
// however it got that way.
//
// One failure never stops the batch: the next order in line may be somebody
// who has been waiting far longer.
func (h *FulfilmentHandler) HandleDispatch(ctx context.Context, _ *asynq.Task) error {
	pending, err := h.suppliers.ListPendingDispatch(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range pending {
		if err := h.service.Dispatch(ctx, item); err != nil {
			h.logger.Error("dispatch fulfilment", "order_id", item.OrderID, "error", err)
			continue
		}
	}
	if len(pending) > 0 {
		h.logger.Info("dispatched fulfilments", "count", len(pending), "task", TaskFulfilmentDispatch)
	}
	return nil
}

func (h *FulfilmentHandler) HandleSweep(ctx context.Context, _ *asynq.Task) error {
	waiting, err := h.fulfilments.ListNeedingAttention(ctx, stuckAfter, 50)
	if err != nil {
		return err
	}
	if len(waiting) == 0 {
		// Silent when there is nothing to say. A line every sweep would bury
		// the one that matters.
		return nil
	}

	// WARN rather than INFO: this is money a jamaah has paid for something that
	// has not arrived, and it is the level Sentry and log alerting watch.
	h.logger.Warn("transactions are waiting on a person",
		"count", len(waiting), "task", TaskFulfilmentSweep)
	for _, item := range waiting {
		// Each one individually, because "seven are stuck" is not actionable
		// and "this jamaah's pulsa never arrived" is.
		h.logger.Warn("fulfilment needs attention",
			"order_id", item.OrderID, "status", item.Status, "supplier", item.SupplierName,
			"product", item.ProductName, "pilgrim", item.PilgrimName,
			"attempts", item.Attempts, "waiting_since", item.CreatedAt,
			"reason", item.LastError)
	}
	return nil
}
