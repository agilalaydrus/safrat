package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/events"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
)

const TaskCascadeDispatch = "cascade:dispatch"

const (
	// After cascadeMaxAttempts failed tries an event stops being claimed and
	// stays as a visible dead-letter row (last_error populated) for ops to
	// inspect, rather than looping forever.
	cascadeMaxAttempts = 5
	cascadeBatchLimit  = 50
)

func NewCascadeDispatchTask() *asynq.Task {
	return asynq.NewTask(TaskCascadeDispatch, nil)
}

// OperatorPusher is the slice of the push notifier the relay needs — kept as a
// local interface so the worker doesn't depend on the service package.
type OperatorPusher interface {
	NotifyOperatorStaff(ctx context.Context, operatorID, title, body string) error
	NotifyGroupPilgrims(ctx context.Context, operatorID, groupID, branchID, title, body string) error
	NotifyKloterPilgrims(ctx context.Context, operatorID, kloterID, title, body string) error
}

type JourneyCascader interface {
	BulkUpdateForGroupAs(ctx context.Context, operatorID, groupID, status, updatedByUserID, notes string) (int32, error)
	BulkUpdateForKloterAs(ctx context.Context, operatorID, kloterID, status, updatedByUserID, notes string) (int32, error)
}

type FinanceMailer interface {
	SendHTML(context.Context, string, string, string, string) error
}

// OutboxHandler drains the cascade_events outbox and performs each event's
// side-effect. Delivery is at-least-once (claim increments attempts before the
// side-effect runs), so side-effects must be idempotent.
type OutboxHandler struct {
	logger   *slog.Logger
	outbox   *repository.OutboxRepository
	push     OperatorPusher
	journeys JourneyCascader
	eventBus *events.Bus
	queries  *db.Queries
	mailer   FinanceMailer
}

func NewOutboxHandler(logger *slog.Logger, outbox *repository.OutboxRepository, push OperatorPusher, journeys JourneyCascader, bus *events.Bus, queries *db.Queries, mailer FinanceMailer) *OutboxHandler {
	return &OutboxHandler{logger: logger, outbox: outbox, push: push, journeys: journeys, eventBus: bus, queries: queries, mailer: mailer}
}

func (h *OutboxHandler) HandleDispatch(ctx context.Context, _ *asynq.Task) error {
	events, err := h.outbox.Claim(ctx, cascadeMaxAttempts, cascadeBatchLimit)
	if err != nil {
		return err
	}
	for _, ev := range events {
		if err := h.dispatch(ctx, ev); err != nil {
			h.logger.Error("cascade dispatch failed", "id", ev.ID, "type", ev.EventType, "attempts", ev.Attempts, "error", err)
			_ = h.outbox.MarkFailed(ctx, ev.ID, err.Error())
			continue
		}
		if err := h.outbox.MarkProcessed(ctx, ev.ID); err != nil {
			h.logger.Error("cascade mark processed", "id", ev.ID, "error", err)
		}
	}
	return nil
}

func (h *OutboxHandler) dispatch(ctx context.Context, ev domain.CascadeEvent) error {
	switch ev.EventType {
	case domain.EventHealthReportCreated:
		var payload domain.HealthReportCreatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err
		}
		if payload.Severity == "BERAT" && h.push != nil {
			return h.push.NotifyOperatorStaff(ctx, ev.OperatorID, "⚠ Laporan Kesehatan BERAT", fmt.Sprintf("%s — perlu perhatian segera.", payload.PilgrimName))
		}
		return nil
	case domain.EventGroupCityUpdated:
		var payload domain.GroupCityUpdatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err
		}
		if payload.GroupID == "" {
			return fmt.Errorf("group city event missing group_id")
		}
		if payload.JourneyStatus != "" && h.journeys != nil {
			if _, err := h.journeys.BulkUpdateForGroupAs(ctx, ev.OperatorID, payload.GroupID, payload.JourneyStatus, payload.UpdatedBy, payload.Notes); err != nil {
				return err
			}
		}
		if payload.NotificationBody != "" && h.push != nil {
			if err := h.push.NotifyGroupPilgrims(ctx, ev.OperatorID, payload.GroupID, "", "Tawafiq Hub", payload.NotificationBody); err != nil {
				return err
			}
		}
		h.eventBus.Publish(ev.OperatorID, "journey", payload.GroupID)
		return nil
	case domain.EventKloterStatusUpdated:
		var payload domain.KloterStatusUpdatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err
		}
		if payload.KloterID == "" {
			return fmt.Errorf("kloter status event missing kloter_id")
		}
		if payload.JourneyStatus != "" && h.journeys != nil {
			notes := fmt.Sprintf("Cascade dari kloter %s -> %s", payload.KloterCode, payload.Status)
			if _, err := h.journeys.BulkUpdateForKloterAs(ctx, ev.OperatorID, payload.KloterID, payload.JourneyStatus, payload.UpdatedBy, notes); err != nil {
				return err
			}
		}
		if payload.NotificationBody != "" && h.push != nil {
			if err := h.push.NotifyKloterPilgrims(ctx, ev.OperatorID, payload.KloterID, "Tawafiq Hub", payload.NotificationBody); err != nil {
				return err
			}
		}
		h.eventBus.Publish(ev.OperatorID, "journey", payload.KloterID)
		return nil
	case domain.EventRitualBulkCompleted:
		var payload domain.RitualBulkCompletedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err
		}
		if payload.GroupID == "" {
			return fmt.Errorf("ritual bulk event missing group_id")
		}
		if payload.NotificationBody != "" && h.push != nil {
			if err := h.push.NotifyGroupPilgrims(ctx, ev.OperatorID, payload.GroupID, payload.BranchID, "Tawafiq Hub", payload.NotificationBody); err != nil {
				return err
			}
		}
		h.eventBus.Publish(ev.OperatorID, "ritual", payload.GroupID)
		return nil
	case domain.EventInstallmentReceipt:
		return h.sendInstallmentReceipt(ctx, ev)
	case domain.EventInstallmentReminder:
		return h.sendInstallmentReminder(ctx, ev)
	default:
		return fmt.Errorf("unsupported cascade event type %q", ev.EventType)
	}
}

func (h *OutboxHandler) sendInstallmentReceipt(ctx context.Context, ev domain.CascadeEvent) error {
	if h.mailer == nil {
		return fmt.Errorf("SMTP is not configured")
	}
	op, err := pgUUIDWorker(ev.OperatorID)
	if err != nil {
		return err
	}
	entity, err := pgUUIDWorker(ev.EntityID)
	if err != nil {
		return err
	}
	row, err := h.queries.GetInstallmentReceiptDelivery(ctx, db.GetInstallmentReceiptDeliveryParams{ID: entity, OperatorID: op})
	if err != nil {
		return err
	}
	to := strings.TrimSpace(row.PilgrimEmail.String)
	if to == "" {
		return fmt.Errorf("pilgrim %s has no email address", row.PilgrimName)
	}
	body := financeEmailShell("Kwitansi Pembayaran", fmt.Sprintf("<p>Assalamualaikum %s,</p><p>Pembayaran Anda kepada <strong>%s</strong> telah diterima.</p><table role=\"presentation\" style=\"width:100%%;border-collapse:collapse\"><tr><td>Nomor kwitansi</td><td><strong>%s</strong></td></tr><tr><td>Nominal</td><td><strong>%s</strong></td></tr><tr><td>Tanggal</td><td>%s</td></tr></table><p>Simpan email ini sebagai bukti pembayaran.</p>", html.EscapeString(row.PilgrimName), html.EscapeString(row.OperatorName), html.EscapeString(row.ReceiptNumber), formatWorkerIDR(row.AmountIdr), row.CreatedAt.Time.Format("02 Jan 2006 15:04 MST")))
	return h.mailer.SendHTML(ctx, to, "Kwitansi "+row.ReceiptNumber+" - "+row.OperatorName, body, "finance-receipt-"+ev.EntityID+"@tawafiqhub.id")
}

func (h *OutboxHandler) sendInstallmentReminder(ctx context.Context, ev domain.CascadeEvent) error {
	if h.mailer == nil {
		return fmt.Errorf("SMTP is not configured")
	}
	op, err := pgUUIDWorker(ev.OperatorID)
	if err != nil {
		return err
	}
	entity, err := pgUUIDWorker(ev.EntityID)
	if err != nil {
		return err
	}
	row, err := h.queries.GetInstallmentReminderDelivery(ctx, db.GetInstallmentReminderDeliveryParams{ID: entity, OperatorID: op})
	if err != nil {
		return err
	}
	to := strings.TrimSpace(row.PilgrimEmail.String)
	if to == "" {
		return fmt.Errorf("pilgrim %s has no email address", row.PilgrimName)
	}
	outstanding := row.PayableAmountIdr.Int64 - row.PaidAmountIdr
	due := "segera"
	if row.NextDueDate.Valid {
		due = row.NextDueDate.Time.Format("02 Jan 2006")
	}
	body := financeEmailShell("Pengingat Pembayaran", fmt.Sprintf("<p>Assalamualaikum %s,</p><p>Ini adalah pengingat pembayaran dari <strong>%s</strong>.</p><p>Sisa pembayaran Anda <strong>%s</strong>, dengan jadwal terdekat pada <strong>%s</strong>.</p><p>Jika pembayaran sudah dilakukan, kirimkan bukti kepada petugas travel agar dapat diverifikasi.</p>", html.EscapeString(row.PilgrimName), html.EscapeString(row.OperatorName), formatWorkerIDR(outstanding), due))
	return h.mailer.SendHTML(ctx, to, "Pengingat pembayaran - "+row.OperatorName, body, "finance-reminder-"+fmt.Sprint(ev.ID)+"@tawafiqhub.id")
}

func financeEmailShell(title, content string) string {
	return "<!doctype html><html><body style=\"margin:0;background:#f7f4ec;font-family:Arial,sans-serif;color:#26332d\"><table role=\"presentation\" width=\"100%\" style=\"padding:32px 16px\"><tr><td align=\"center\"><table role=\"presentation\" width=\"520\" style=\"max-width:100%;background:#fff;border:1px solid #e5dfd0;border-radius:14px;overflow:hidden\"><tr><td style=\"background:#0d3d27;padding:22px 28px;color:#fff;font-size:20px;font-weight:700\">Tawafiq Hub</td></tr><tr><td style=\"padding:28px\"><h1 style=\"font-size:20px;margin:0 0 16px\">" + html.EscapeString(title) + "</h1><div style=\"font-size:14px;line-height:1.65\">" + content + "</div></td></tr></table></td></tr></table></body></html>"
}
func formatWorkerIDR(value int64) string {
	raw := fmt.Sprintf("%d", value)
	for index := len(raw) - 3; index > 0; index -= 3 {
		raw = raw[:index] + "." + raw[index:]
	}
	return "Rp " + raw
}
func pgUUIDWorker(value string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}
