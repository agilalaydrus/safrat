package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskCommissionReconcile = "commission:reconcile"

// NewCommissionReconcileTask enqueues a sweep that credits any paid order whose
// commission never reached the ledger.
func NewCommissionReconcileTask() *asynq.Task {
	return asynq.NewTask(TaskCommissionReconcile, nil)
}

// CommissionHandler repairs gaps between paid orders and the commission ledger.
//
// The write path already records commission when an order is paid, and does so
// idempotently. This exists because that path can still be missed entirely:
// migrations run before the API restarts, so an order paid in that window is
// marked PAID by a binary that predates the ledger, and no later event ever
// revisits it. The agent simply never gets credited, silently and permanently
// — the worst failure shape money code has.
//
// So the ledger is not left to depend on one write path being reached. This
// reconciles the two representations and heals the difference, which also
// covers causes nobody predicted.
type CommissionHandler struct {
	logger *slog.Logger
	ledger *repository.LedgerRepository
}

func NewCommissionHandler(logger *slog.Logger, ledger *repository.LedgerRepository) *CommissionHandler {
	return &CommissionHandler{logger: logger, ledger: ledger}
}

func (h *CommissionHandler) HandleReconcile(ctx context.Context, _ *asynq.Task) error {
	repaired, err := h.ledger.ReconcileEarnedCommission(ctx)
	if err != nil {
		return err
	}
	// Silent when there is nothing to do — this runs on a schedule, and a log
	// line every pass would bury the one that matters.
	if repaired > 0 {
		h.logger.Warn("credited commission that the payment path missed",
			"entries", repaired, "task", TaskCommissionReconcile)
	}
	return nil
}
