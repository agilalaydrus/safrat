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
	logger   *slog.Logger
	kyc      *repository.KYCRepository
	pilgrims *repository.PilgrimRepository
}

func NewKYCHandler(logger *slog.Logger, kyc *repository.KYCRepository, pilgrims *repository.PilgrimRepository) *KYCHandler {
	return &KYCHandler{logger: logger, kyc: kyc, pilgrims: pilgrims}
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

	// Passport numbers ride the same sweep. Both are plaintext identity data
	// waiting for a key that lives in this process, and both are moved in
	// batches so a large table is never migrated in one transaction.
	//
	// Errors are reported and not returned: a batch that fails leaves the rows
	// unmigrated and readable, which is the same state as before, and the next
	// pass tries again. Failing the whole task would also abandon the identity
	// numbers that did move.
	if h.pilgrims != nil {
		sealed, passportErr := h.pilgrims.MigrateLegacyPassports(ctx, 200)
		if passportErr != nil {
			h.logger.Error("seal legacy passports", "error", passportErr, "task", TaskKYCMigrate)
			return nil
		}
		if sealed > 0 {
			remaining, _ := h.pilgrims.LegacyPassportCount(ctx)
			h.logger.Info("sealed passport numbers",
				"count", sealed, "remaining", remaining, "task", TaskKYCMigrate)
		}
	}
	return nil
}
