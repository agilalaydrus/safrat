package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/hibiken/asynq"
)

const TaskMarkOverdueVendorPayments = "cashflow:mark_overdue"

// NewMarkOverdueVendorPaymentsTask enqueues the periodic sweep that flips
// PENDING vendor payments whose due_date has passed to OVERDUE.
func NewMarkOverdueVendorPaymentsTask() *asynq.Task {
	return asynq.NewTask(TaskMarkOverdueVendorPayments, nil)
}

type CashFlowHandler struct {
	logger  *slog.Logger
	queries *db.Queries
}

func NewCashFlowHandler(logger *slog.Logger, queries *db.Queries) *CashFlowHandler {
	return &CashFlowHandler{logger: logger, queries: queries}
}

func (h *CashFlowHandler) HandleMarkOverdue(ctx context.Context, _ *asynq.Task) error {
	if err := h.queries.MarkOverdueVendorPayments(ctx); err != nil {
		h.logger.Error("mark overdue vendor payments", "error", err)
	}
	return nil
}
