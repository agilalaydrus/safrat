package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
)

// xenditWebhookPayload only lists the fields this handler actually reads —
// Xendit's real payload has many more (payment channel, fees, etc.) that
// nothing here needs yet.
type xenditWebhookPayload struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// NewXenditWebhookHandler is a plain net/http handler, not a Connect RPC —
// Xendit calls back over a normal webhook POST, it doesn't speak Connect.
// Registered directly on the mux in main.go.
func NewXenditWebhookHandler(logger *slog.Logger, orders *repository.OrderRepository, webhookToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !payment.VerifyWebhookToken(webhookToken, r.Header.Get("X-CALLBACK-TOKEN")) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var payload xenditWebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		var err error
		switch payload.Status {
		case "PAID":
			_, err = orders.MarkPaidByInvoiceID(ctx, payload.ID)
		case "EXPIRED":
			_, err = orders.MarkStatusByInvoiceID(ctx, payload.ID, "EXPIRED")
		case "FAILED":
			_, err = orders.MarkStatusByInvoiceID(ctx, payload.ID, "FAILED")
		default:
			// Unrecognized/no-op status (e.g. "PENDING" echoed back) — 200 so
			// Xendit doesn't retry a delivery there's nothing to do with.
		}
		if err != nil {
			// Not found / already transitioned (e.g. a replayed delivery
			// after this invoice already went PAID) is expected, not an
			// error worth 500-ing over — Xendit would just retry forever.
			logger.Info("xendit webhook: order not updated", "invoice_id", payload.ID, "status", payload.Status, "reason", err.Error())
		}
		w.WriteHeader(http.StatusOK)
	}
}
