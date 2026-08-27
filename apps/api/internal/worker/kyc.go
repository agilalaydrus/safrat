package worker

import (
	"context"
	"log/slog"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskKYCMigrate = "kyc:migrate"

func NewKYCMigrateTask() *asynq.Task {
	return asynq.NewTask(TaskKYCMigrate, nil)
}

// KYCHandler moves identity numbers out of the columns they used to live in,
// encrypting them on the way.
//
// It runs in Go rather than in a migration because the encryption key lives in
// this process, not in the database. A SQL backfill could only copy the
// plaintext across, which would leave the same unencrypted number in two places
// until something came along to fix it — and "until something comes along" is
// how plaintext identity numbers survive for years.
//
// Each record is moved and its old column cleared in one transaction, so no row
// is ever left with the number in both places or in neither.
type KYCHandler struct {
	logger *slog.Logger
	kyc    *repository.KYCRepository
}

func NewKYCHandler(logger *slog.Logger, kyc *repository.KYCRepository) *KYCHandler {
	return &KYCHandler{logger: logger, kyc: kyc}
}

func (h *KYCHandler) HandleMigrate(ctx context.Context, _ *asynq.Task) error {
	moved, err := h.kyc.MigrateLegacyIdentities(ctx, 200)
	if err != nil {
		return err
	}
	if moved > 0 {
		h.logger.Info("moved identity numbers into encrypted records",
			"count", moved, "task", TaskKYCMigrate)
	}
	return nil
}
