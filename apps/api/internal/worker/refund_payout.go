package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/hibiken/asynq"
)

const TaskRefundPayoutDispatch = "refund-payout:dispatch"

func NewRefundPayoutDispatchTask() *asynq.Task { return asynq.NewTask(TaskRefundPayoutDispatch, nil) }

type RefundPayoutHandler struct {
	logger  *slog.Logger
	payouts *repository.RefundPayoutRepository
	service *service.RefundPayoutService
}

func NewRefundPayoutHandler(logger *slog.Logger, payouts *repository.RefundPayoutRepository, payoutService *service.RefundPayoutService) *RefundPayoutHandler {
	return &RefundPayoutHandler{logger: logger, payouts: payouts, service: payoutService}
}
func (h *RefundPayoutHandler) HandleDispatch(ctx context.Context, _ *asynq.Task) error {
	requests, err := h.payouts.ListGatewayWork(ctx, 25)
	if err != nil {
		return err
	}
	for _, request := range requests {
		if err := h.service.DispatchGatewayPayout(ctx, request.ID); err != nil {
			h.logger.Error("dispatch refund payout", "request_id", request.ID, "error", err)
		}
	}
	if len(requests) > 0 {
		h.logger.Info("processed refund payout batch", "count", len(requests), "task", TaskRefundPayoutDispatch)
	}
	return nil
}
