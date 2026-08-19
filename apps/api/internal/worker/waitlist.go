package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskWaitlistExpire = "waitlist:expire"

// NewWaitlistExpireTask enqueues the periodic sweep for stale PROMOTED
// waitlist entries whose 48h confirmation window has lapsed.
func NewWaitlistExpireTask() *asynq.Task {
	return asynq.NewTask(TaskWaitlistExpire, nil)
}

// WaitlistHandler expires stale PROMOTED entries and promotes the next
// WAITING entry in each affected season — the same "no one waiting" no-op
// path ConfirmCancellation uses after a live cancellation.
type WaitlistHandler struct {
	logger   *slog.Logger
	waitlist *repository.WaitlistRepository
}

func NewWaitlistHandler(logger *slog.Logger, waitlist *repository.WaitlistRepository) *WaitlistHandler {
	return &WaitlistHandler{logger: logger, waitlist: waitlist}
}

func (h *WaitlistHandler) HandleExpire(ctx context.Context, _ *asynq.Task) error {
	affected, err := h.waitlist.ExpireStale(ctx)
	if err != nil {
		h.logger.Error("expire waitlist entries", "error", err)
		return nil
	}
	for _, pair := range affected {
		if _, err := h.waitlist.PromoteNextWaiting(ctx, pair.OperatorID, pair.SeasonID); err != nil {
			h.logger.Error("promote next waitlist entry", "error", err, "season_id", pair.SeasonID)
		}
	}
	return nil
}
