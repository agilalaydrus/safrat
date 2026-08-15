package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/hibiken/asynq"
)

const TaskTierRecalculateAll = "tier:recalculate_all"

// NewTierRecalculateAllTask enqueues a sweep across every operator. It's what the
// periodic scheduler in cmd/worker submits.
func NewTierRecalculateAllTask() *asynq.Task {
	return asynq.NewTask(TaskTierRecalculateAll, nil)
}

// TierHandler recomputes agent tiers. Payout is deliberately not part of this
// worker — it needs order/revenue data that doesn't exist without Module 7.
type TierHandler struct {
	logger    *slog.Logger
	operators *repository.OperatorRepository
	agents    *service.AgentService
}

func NewTierHandler(logger *slog.Logger, operators *repository.OperatorRepository, agents *service.AgentService) *TierHandler {
	return &TierHandler{logger: logger, operators: operators, agents: agents}
}

func (h *TierHandler) HandleRecalculateAll(ctx context.Context, _ *asynq.Task) error {
	operatorIDs, err := h.operators.ListIDs(ctx)
	if err != nil {
		return err
	}
	for _, operatorID := range operatorIDs {
		if err := h.agents.RecalculateTiers(ctx, operatorID); err != nil {
			h.logger.Error("recalculate tiers", "operator_id", operatorID, "error", err)
		}
	}
	return nil
}
