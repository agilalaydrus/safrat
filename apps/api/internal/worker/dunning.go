package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskSubscriptionDunning = "billing:dunning"

func NewSubscriptionDunningTask() *asynq.Task {
	return asynq.NewTask(TaskSubscriptionDunning, nil)
}

type dunningRunner interface {
	Settings(context.Context) (repository.DunningSettings, error)
	DueDunning(context.Context, repository.DunningSettings) ([]repository.DunningStep, error)
	RecordDunning(context.Context, repository.DunningStep) (bool, error)
}

// DunningHandler chases subscriptions whose access has lapsed.
//
// Every step is claimed by a primary key before its message is enqueued, so
// running twice sends nothing twice. One failing step does not abandon the
// rest: a single agency with a broken row must not stop everybody else from
// being chased.
type DunningHandler struct {
	logger *slog.Logger
	runner dunningRunner
	dryRun bool
}

// NewDunningHandler builds the handler. dryRun claims and records steps
// without... nothing: claiming is what produces the message. See HandleRun.
func NewDunningHandler(logger *slog.Logger, subscriptions *repository.SubscriptionRepository, dryRun bool) *DunningHandler {
	return &DunningHandler{logger: logger, runner: subscriptions, dryRun: dryRun}
}

// HandleRun sends the reminders that are due.
//
// In dry-run mode it reports what it would send and writes nothing at all.
// Billing messages reach real customers, and one demand sent to an agency that
// has already paid costs more than a week of delay — so the first run of this
// in an environment should be a dry one, compared against a list made by hand.
func (h *DunningHandler) HandleRun(ctx context.Context, _ *asynq.Task) error {
	settings, err := h.runner.Settings(ctx)
	if err != nil {
		h.logger.Error("read dunning settings", "error", err)
		return err
	}
	steps, err := h.runner.DueDunning(ctx, settings)
	if err != nil {
		h.logger.Error("list due dunning", "error", err)
		return err
	}
	if h.dryRun {
		for _, step := range steps {
			h.logger.Info("dunning dry run", "operator", step.OperatorName,
				"stage", step.Stage, "days_overdue", step.DaysOverdue, "suspend", step.Suspend)
		}
		h.logger.Info("dunning dry run complete", "would_send", len(steps))
		return nil
	}

	sent, suspended := 0, 0
	for _, step := range steps {
		created, err := h.runner.RecordDunning(ctx, step)
		if err != nil {
			h.logger.Error("record dunning", "operator", step.OperatorName, "stage", step.Stage, "error", err)
			continue
		}
		if !created {
			continue
		}
		sent++
		if step.Suspend {
			suspended++
		}
	}
	if sent > 0 {
		h.logger.Info("dunning sent", "steps", sent, "suspended", suspended)
	}
	return nil
}
