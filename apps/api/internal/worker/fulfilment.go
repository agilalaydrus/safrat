package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskFulfilmentSweep = "fulfilment:sweep"

// stuckAfter is how long a fulfilment may sit unanswered before somebody should
// be told. Suppliers here normally answer in seconds; an hour is generous
// enough that a slow one does not raise an alarm, and short enough that a
// jamaah is not waiting a day for pulsa nobody noticed was missing.
const stuckAfter = time.Hour

func NewFulfilmentSweepTask() *asynq.Task {
	return asynq.NewTask(TaskFulfilmentSweep, nil)
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
}

func NewFulfilmentHandler(logger *slog.Logger, fulfilments *repository.FulfilmentRepository) *FulfilmentHandler {
	return &FulfilmentHandler{logger: logger, fulfilments: fulfilments}
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
