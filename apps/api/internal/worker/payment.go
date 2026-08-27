package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/hibiken/asynq"
)

const TaskPaymentPoll = "payment:poll"

// settlementGraceMinutes is how long an order is left alone before the poller
// looks at it. A webhook normally arrives within seconds; polling immediately
// would mean every checkout in the system gets an extra API call for nothing.
const settlementGraceMinutes = 5

// settlementBatch bounds one sweep, so a backlog is worked through over
// several runs instead of one very long outbound-call storm.
const settlementBatch = 100

func NewPaymentPollTask() *asynq.Task {
	return asynq.NewTask(TaskPaymentPoll, nil)
}

// PaymentHandler asks the gateway about transactions that are still waiting.
//
// Without it a dropped webhook delivery is permanent: the order sits PENDING
// forever, the jamaah has paid, and nobody is told. Polling turns the webhook
// into an optimisation — it makes settlement fast — rather than the only thing
// standing between a payment and the books.
//
// It shares OrderService.SettleFromGateway with the webhook path, so there is
// one definition of what settling means and one place the amount is checked.
type PaymentHandler struct {
	logger *slog.Logger
	orders *repository.OrderRepository
	// service carries the settlement rules; the repository here only supplies
	// the list of what to ask about.
	service *service.OrderService
}

func NewPaymentHandler(logger *slog.Logger, orders *repository.OrderRepository, orderService *service.OrderService) *PaymentHandler {
	return &PaymentHandler{logger: logger, orders: orders, service: orderService}
}

func (h *PaymentHandler) HandlePoll(ctx context.Context, _ *asynq.Task) error {
	waiting, err := h.orders.ListAwaitingSettlement(ctx, settlementGraceMinutes, settlementBatch)
	if err != nil {
		return err
	}
	checked := make([]string, 0, len(waiting))
	for _, order := range waiting {
		// One failure must not stop the sweep: the next order may be a
		// payment that has been sitting unsettled for hours.
		if err := h.service.SettleFromGateway(ctx, order.InvoiceID); err != nil {
			h.logger.Error("poll settlement", "order_id", order.OrderID, "invoice_id", order.InvoiceID, "error", err)
		}
		// Marked either way. An order that could not be reached must still go
		// to the back of the queue, or one unreachable invoice would be
		// retried on every pass while everything behind it waits.
		checked = append(checked, order.OrderID)
	}
	if err := h.orders.MarkGatewayChecked(ctx, checked); err != nil {
		h.logger.Error("record gateway checks", "error", err)
	}
	if len(waiting) > 0 {
		h.logger.Info("polled pending transactions", "count", len(waiting), "task", TaskPaymentPoll)
	}
	return nil
}
