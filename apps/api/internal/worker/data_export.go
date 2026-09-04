package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
)

const TaskDataExportProcess = "data-export:process"

// exportRetention is how long a finished export's download link works. Long
// enough that a busy owner does not miss it, short enough that a link is not
// a permanent, forgettable copy of the tenant's own data sitting in an inbox.
const exportRetention = 7 * 24 * time.Hour

type dataExportBuilder interface {
	BuildExport(ctx context.Context, operatorID string) ([]byte, error)
}

type dataExportStorage interface {
	PutDataExport(ctx context.Context, operatorID, exportID string, data []byte) (string, error)
	DeleteDataExport(ctx context.Context, objectKey string) error
}

func NewDataExportProcessTask() *asynq.Task {
	return asynq.NewTask(TaskDataExportProcess, nil, asynq.MaxRetry(1))
}

func NewDataExportExpireTask() *asynq.Task {
	return asynq.NewTask("data-export:expire", nil, asynq.MaxRetry(1))
}

const TaskDataExportExpire = "data-export:expire"

type DataExportHandler struct {
	logger   *slog.Logger
	registry *repository.DataExportRepository
	builder  dataExportBuilder
	storage  dataExportStorage
	now      func() time.Time
}

func NewDataExportHandler(logger *slog.Logger, registry *repository.DataExportRepository, builder dataExportBuilder, storage dataExportStorage) *DataExportHandler {
	return &DataExportHandler{logger: logger, registry: registry, builder: builder, storage: storage, now: time.Now}
}

// HandleProcess claims at most one export per tick. An export is a
// heavyweight, occasional request — a person clicking a button, not a queue
// under load — so there is no batching to be gained from claiming more, and
// claiming one at a time keeps a single slow export from blocking the ones
// behind it in the same tick.
func (h *DataExportHandler) HandleProcess(ctx context.Context, _ *asynq.Task) error {
	if h.storage == nil {
		// Unconfigured storage is not a fault: local development runs without
		// R2 credentials, and an export request left PENDING is the honest
		// state — it says "not available here" rather than failing loudly on
		// a feature nobody asked to disable.
		return nil
	}
	claimed, ok, err := h.registry.ClaimNext(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	data, err := h.builder.BuildExport(ctx, claimed.OperatorID)
	if err != nil {
		h.logger.Error("build data export", "export_id", claimed.ID, "operator_id", claimed.OperatorID, "error", err)
		if markErr := h.registry.MarkFailed(ctx, claimed.ID, "gagal menyusun berkas ekspor"); markErr != nil {
			h.logger.Error("mark data export failed", "export_id", claimed.ID, "error", markErr)
		}
		return err
	}

	key, err := h.storage.PutDataExport(ctx, claimed.OperatorID, claimed.ID, data)
	if err != nil {
		h.logger.Error("upload data export", "export_id", claimed.ID, "error", err)
		if markErr := h.registry.MarkFailed(ctx, claimed.ID, "gagal mengunggah berkas ekspor"); markErr != nil {
			h.logger.Error("mark data export failed", "export_id", claimed.ID, "error", markErr)
		}
		return err
	}

	if err := h.registry.MarkReady(ctx, claimed.ID, key, int64(len(data)), h.now().Add(exportRetention)); err != nil {
		h.logger.Error("mark data export ready", "export_id", claimed.ID, "error", err)
		return err
	}
	h.logger.Info("data export ready", "export_id", claimed.ID, "operator_id", claimed.OperatorID, "bytes", len(data))
	return nil
}

// HandleExpire deletes the file behind a lapsed download link and marks the
// row so it stops claiming to still work. Run far less often than processing
// — an export is not urgent to clean up, and this only has to run before
// anyone actually notices a link is dead.
func (h *DataExportHandler) HandleExpire(ctx context.Context, _ *asynq.Task) error {
	if h.storage == nil {
		return nil
	}
	expired, err := h.registry.ListExpired(ctx, 50)
	if err != nil {
		return err
	}
	for _, export := range expired {
		if export.ObjectKey != "" {
			if err := h.storage.DeleteDataExport(ctx, export.ObjectKey); err != nil {
				h.logger.Error("delete expired data export object", "export_id", export.ID, "error", err)
				continue
			}
		}
		if err := h.registry.MarkExpired(ctx, export.ID); err != nil {
			h.logger.Error("mark data export expired", "export_id", export.ID, "error", err)
		}
	}
	return nil
}
