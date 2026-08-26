package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskSubscriptionSweep = "subscription:sweep"

// NewSubscriptionSweepTask enqueues the periodic subscription housekeeping.
func NewSubscriptionSweepTask() *asynq.Task {
	return asynq.NewTask(TaskSubscriptionSweep, nil)
}

// SubscriptionHandler closes invoices nobody paid and marks lapsed
// subscriptions.
//
// Expiring invoices is not cosmetic. A pending bank transfer holds its unique
// amount through a partial unique index, and that amount is what ties an
// incoming mutation to an invoice. Without this sweep, every abandoned invoice
// keeps its suffix forever, the pool of 999 drains, and issuing a transfer
// eventually fails outright.
type subscriptionSweeper interface {
	ExpireOverdueInvoices(context.Context) (int64, error)
	MarkLapsed(context.Context) (int64, error)
}

type SubscriptionHandler struct {
	logger  *slog.Logger
	sweeper subscriptionSweeper
}

func NewSubscriptionHandler(logger *slog.Logger, subscriptions *repository.SubscriptionRepository) *SubscriptionHandler {
	return &SubscriptionHandler{logger: logger, sweeper: subscriptions}
}

func (h *SubscriptionHandler) HandleSweep(ctx context.Context, _ *asynq.Task) error {
	expired, err := h.sweeper.ExpireOverdueInvoices(ctx)
	if err != nil {
		// Reported, not returned: a retry storm would not help, and the next
		// tick does the same work.
		h.logger.Error("expire overdue invoices", "error", err)
		return nil
	}
	// Access is already governed by access_until, so this only keeps the status
	// readable. It never grants or removes access on its own.
	lapsed, err := h.sweeper.MarkLapsed(ctx)
	if err != nil {
		h.logger.Error("mark lapsed subscriptions", "error", err)
		return nil
	}
	if expired > 0 || lapsed > 0 {
		h.logger.Info("subscription sweep", "invoices_expired", expired, "subscriptions_lapsed", lapsed)
	}
	return nil
}
