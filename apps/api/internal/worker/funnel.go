package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskFunnelRollUp = "funnel:rollup"

// funnelRetentionDays is how long raw visitor events are kept. The daily
// summaries outlive them; these are what the summaries are computed from, and
// keeping them longer buys nothing while carrying both storage and liability.
const funnelRetentionDays = 90

func NewFunnelRollUpTask() *asynq.Task {
	return asynq.NewTask(TaskFunnelRollUp, nil)
}

type funnelRoller interface {
	RollUpDay(context.Context, time.Time) (int64, error)
	PurgeRawEvents(context.Context, int) (int64, error)
}

type FunnelHandler struct {
	logger *slog.Logger
	funnel funnelRoller
}

func NewFunnelRollUpHandler(logger *slog.Logger, funnelRepository *repository.FunnelRepository) *FunnelHandler {
	return &FunnelHandler{logger: logger, funnel: funnelRepository}
}

// HandleRollUp recomputes yesterday and today, then purges what has aged out.
//
// Yesterday as well as today because a day is only complete once it has ended,
// and a job that ran at 23:00 saw an unfinished one. Recomputing is cheap and
// the write is idempotent, so redoing a finished day costs a second pass and
// changes nothing.
func (h *FunnelHandler) HandleRollUp(ctx context.Context, _ *asynq.Task) error {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Falling back to UTC would silently shift every day boundary by seven
		// hours, so this refuses instead.
		h.logger.Error("load Asia/Jakarta", "error", err)
		return err
	}
	today := time.Now().In(jakarta)
	for _, day := range []time.Time{today.AddDate(0, 0, -1), today} {
		rows, rollErr := h.funnel.RollUpDay(ctx, day)
		if rollErr != nil {
			h.logger.Error("roll up funnel day", "day", day.Format("2006-01-02"), "error", rollErr)
			continue
		}
		if rows > 0 {
			h.logger.Info("funnel day rolled up", "day", day.Format("2006-01-02"), "rows", rows)
		}
	}
	purged, purgeErr := h.funnel.PurgeRawEvents(ctx, funnelRetentionDays)
	if purgeErr != nil {
		h.logger.Error("purge funnel events", "error", purgeErr)
		return purgeErr
	}
	if purged > 0 {
		h.logger.Info("funnel raw events purged", "rows", purged, "keep_days", funnelRetentionDays)
	}
	return nil
}
