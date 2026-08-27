package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskDailySpendPurge = "daily_spend:purge"

func NewDailySpendPurgeTask() *asynq.Task {
	return asynq.NewTask(TaskDailySpendPurge, nil)
}

// DailySpendHandler expires daily-limit counters.
//
// Without it the table grows one row per account per day forever. It is small
// per row and unbounded over time, which is the shape of thing nobody notices
// until a backup starts taking noticeably longer.
type DailySpendHandler struct {
	logger *slog.Logger
	orders *repository.OrderRepository
}

func NewDailySpendHandler(logger *slog.Logger, orders *repository.OrderRepository) *DailySpendHandler {
	return &DailySpendHandler{logger: logger, orders: orders}
}

func (h *DailySpendHandler) HandlePurge(ctx context.Context, _ *asynq.Task) error {
	removed, err := h.orders.PurgeExpiredDailySpend(ctx, domain.DailySpendRetentionDays)
	if err != nil {
		return err
	}
	// Only worth a line when something happened. A daily "removed 0" says
	// nothing and trains people to skip the log.
	if removed > 0 {
		h.logger.Info("expired daily spend rows",
			"removed", removed, "keep_days", domain.DailySpendRetentionDays)
	}
	return nil
}
