package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskPlanOverrideExpire = "platform:plan-override-expire"

func NewPlanOverrideExpireTask() *asynq.Task {
	return asynq.NewTask(TaskPlanOverrideExpire, nil)
}

type planOverrideExpirer interface {
	ExpirePlanOverrides(context.Context) (int32, error)
}

type PlanOverrideHandler struct {
	logger  *slog.Logger
	expirer planOverrideExpirer
}

func NewPlanOverrideHandler(logger *slog.Logger, platform *repository.PlatformRepository) *PlanOverrideHandler {
	return &PlanOverrideHandler{logger: logger, expirer: platform}
}

func (h *PlanOverrideHandler) HandleExpire(ctx context.Context, _ *asynq.Task) error {
	count, err := h.expirer.ExpirePlanOverrides(ctx)
	if err != nil {
		h.logger.Error("expire platform plan overrides", "error", err)
		return err
	}
	if count > 0 {
		h.logger.Info("platform plan overrides expired", "count", count)
	}
	return nil
}
