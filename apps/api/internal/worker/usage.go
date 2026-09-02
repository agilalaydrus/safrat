package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskUsageRecompute = "billing:usage-recompute"

func NewUsageRecomputeTask() *asynq.Task {
	return asynq.NewTask(TaskUsageRecompute, nil)
}

type usageRecomputer interface {
	RecomputeUsage(context.Context) (int64, error)
}

// UsageHandler takes the daily snapshot of what each tenant is using.
type UsageHandler struct {
	logger *slog.Logger
	usage  usageRecomputer
}

func NewUsageHandler(logger *slog.Logger, subscriptions *repository.SubscriptionRepository) *UsageHandler {
	return &UsageHandler{logger: logger, usage: subscriptions}
}

// HandleRecompute writes today's figures, overwriting any already written for
// today. Running twice costs a second pass and changes nothing.
func (h *UsageHandler) HandleRecompute(ctx context.Context, _ *asynq.Task) error {
	rows, err := h.usage.RecomputeUsage(ctx)
	if err != nil {
		h.logger.Error("recompute usage counters", "error", err)
		return err
	}
	h.logger.Info("usage counters recomputed", "rows", rows)
	return nil
}
