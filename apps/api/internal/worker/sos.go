package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/service"
	"github.com/hibiken/asynq"
)

const TaskSOSEscalate = "sos:escalate"

// NewSOSEscalateTask enqueues the periodic sweep for stale SOS alerts.
func NewSOSEscalateTask() *asynq.Task {
	return asynq.NewTask(TaskSOSEscalate, nil)
}

// SOSHandler flips unacknowledged SOS alerts to ESCALATED and pushes
// coordinators — see service.SOSService.EscalateStaleAlerts.
type SOSHandler struct {
	logger *slog.Logger
	sos    *service.SOSService
}

func NewSOSHandler(logger *slog.Logger, sos *service.SOSService) *SOSHandler {
	return &SOSHandler{logger: logger, sos: sos}
}

func (h *SOSHandler) HandleEscalate(ctx context.Context, _ *asynq.Task) error {
	if err := h.sos.EscalateStaleAlerts(ctx); err != nil {
		h.logger.Error("escalate SOS alerts", "error", err)
	}
	return nil
}
