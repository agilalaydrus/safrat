package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskAnnouncementDispatch = "announcement:dispatch"

// NewAnnouncementDispatchTask enqueues the periodic sweep for scheduled
// announcements whose moment has arrived. "Send now" announcements
// (PlatformService.SendAnnouncement) dispatch inline in the same request and
// never touch this sweep at all — it exists only for "terjadwal" (§10.1
// DESAIN).
func NewAnnouncementDispatchTask() *asynq.Task {
	return asynq.NewTask(TaskAnnouncementDispatch, nil)
}

type AnnouncementHandler struct {
	logger        *slog.Logger
	announcements *repository.AnnouncementRepository
}

func NewAnnouncementHandler(logger *slog.Logger, announcements *repository.AnnouncementRepository) *AnnouncementHandler {
	return &AnnouncementHandler{logger: logger, announcements: announcements}
}

// HandleDispatch fires each due announcement one at a time. Dispatch itself
// re-resolves recipients from the stored filter and is safe to call twice
// (an already-sent announcement is a no-op), so a slow run overlapping the
// next tick cannot double-send.
func (h *AnnouncementHandler) HandleDispatch(ctx context.Context, _ *asynq.Task) error {
	ids, err := h.announcements.DueForDispatch(ctx)
	if err != nil {
		h.logger.Error("list due announcements", "error", err)
		return nil
	}
	for _, id := range ids {
		if _, err := h.announcements.Dispatch(ctx, id); err != nil {
			h.logger.Error("dispatch announcement", "error", err, "announcement_id", id)
		}
	}
	return nil
}
