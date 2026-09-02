package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskSubscriptionSweep = "subscription:sweep"

// NewSubscriptionSweepTask enqueues the periodic subscription housekeeping.
func NewSubscriptionSweepTask() *asynq.Task {
	return asynq.NewTask(TaskSubscriptionSweep, nil)
}

// SubscriptionHandler closes invoices nobody paid and marks lapsed
// subscriptions.
//
// Expiring invoices is not cosmetic. A pending bank transfer holds its unique
// amount through a partial unique index, and that amount is what ties an
// incoming mutation to an invoice. Without this sweep, every abandoned invoice
// keeps its suffix forever, the pool of 999 drains, and issuing a transfer
// eventually fails outright.
// renewalLeadTime is how far ahead a subscription is billed. Long enough that
// a bank transfer has time to arrive and be matched before access runs out —
// a bill issued the day access ends is a bill that arrives too late to act on.
const renewalLeadTime = 7 * 24 * time.Hour

// staleCreditAge is when an unmatched credit stops being "recent" and starts
// being a problem. A travel that paid yesterday is waiting; one that paid three
// days ago is wondering whether anybody noticed.
const staleCreditAge = 48 * time.Hour

type subscriptionSweeper interface {
	ExpireOverdueInvoices(context.Context) (int64, error)
	MarkLapsed(context.Context) (int64, error)
	ListDueForRenewal(context.Context, time.Duration) ([]repository.RenewalDue, error)
	IssueBillingPeriod(ctx context.Context, operatorID, plan string, periodStart time.Time, expectedBase int64, actorUserID string) (repository.Invoice, string, bool, error)
	CountStaleUnmatched(context.Context, time.Duration) (int, int64, error)
}

type SubscriptionHandler struct {
	logger  *slog.Logger
	sweeper subscriptionSweeper
}

func NewSubscriptionHandler(logger *slog.Logger, subscriptions *repository.SubscriptionRepository) *SubscriptionHandler {
	return &SubscriptionHandler{logger: logger, sweeper: subscriptions}
}

func (h *SubscriptionHandler) HandleSweep(ctx context.Context, _ *asynq.Task) error {
	expired, err := h.sweeper.ExpireOverdueInvoices(ctx)
	if err != nil {
		// Reported, not returned: a retry storm would not help, and the next
		// tick does the same work.
		h.logger.Error("expire overdue invoices", "error", err)
		return nil
	}
	// Access is already governed by access_until, so this only keeps the status
	// readable. It never grants or removes access on its own.
	lapsed, err := h.sweeper.MarkLapsed(ctx)
	if err != nil {
		h.logger.Error("mark lapsed subscriptions", "error", err)
		return nil
	}
	issued := h.issueRenewals(ctx)
	h.warnAboutStaleCredits(ctx)

	if expired > 0 || lapsed > 0 || issued > 0 {
		h.logger.Info("subscription sweep",
			"invoices_expired", expired, "subscriptions_lapsed", lapsed, "renewals_issued", issued)
	}
	return nil
}

// issueRenewals bills subscriptions before their access runs out.
//
// Nothing did this: the sweep expired invoices and marked subscriptions lapsed,
// and never asked anybody to pay for the next period. A subscription simply
// stopped, quietly, and revenue ended without anybody deciding it should.
//
// One at a time, per operator, guarded in the query rather than here — two
// outstanding invoices would put two unique amounts in play for one operator,
// and a transfer against the older one would arrive looking unmatched.
func (h *SubscriptionHandler) issueRenewals(ctx context.Context) int {
	due, err := h.sweeper.ListDueForRenewal(ctx, renewalLeadTime)
	if err != nil {
		h.logger.Error("list subscriptions due for renewal", "error", err)
		return 0
	}
	issued := 0
	for _, item := range due {
		invoice, _, created, err := h.sweeper.IssueBillingPeriod(ctx, item.OperatorID, item.Plan,
			item.PeriodStart, item.BaseAmount, "system")
		if err != nil {
			// One failure must not stop the rest: every operator skipped here
			// is one nobody is billing.
			h.logger.Error("issue renewal invoice",
				"operator_id", item.OperatorID, "plan", item.Plan, "error", err)
			continue
		}
		if !created {
			continue
		}
		h.logger.Info("renewal invoice issued",
			"operator_id", item.OperatorID, "plan", item.Plan, "amount_idr", invoice.Amount)
		issued++
	}
	return issued
}

// warnAboutStaleCredits says when money has arrived that nothing claimed.
//
// The unmatched queue is visible in the admin panel and only to somebody who
// opens it. A credit sitting there is a travel who believes they have paid
// while the system disagrees, so it needs to reach a log somebody watches
// rather than waiting to be found.
func (h *SubscriptionHandler) warnAboutStaleCredits(ctx context.Context) {
	count, total, err := h.sweeper.CountStaleUnmatched(ctx, staleCreditAge)
	if err != nil {
		h.logger.Error("count stale unmatched credits", "error", err)
		return
	}
	if count > 0 {
		h.logger.Warn("bank credits unaccounted for",
			"count", count, "total_idr", total, "older_than_hours", int(staleCreditAge.Hours()),
			"action", "cocokkan di panel admin tab Transfer, atau tandai bukan pembayaran langganan")
	}
}
