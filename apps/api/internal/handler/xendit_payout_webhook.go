package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/service"
)

type xenditPayoutWebhook struct {
	Event string `json:"event"`
	Data  struct {
		PayoutID string `json:"payout_id"`
	} `json:"data"`
}

// A callback only wakes reconciliation. The payout is fetched through the
// authenticated API before a ledger movement is accepted.
func NewXenditPayoutWebhookHandler(logger *slog.Logger, payouts *service.RefundPayoutService, webhookToken string, source *WebhookSourceGuard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !source.Allows(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if !payment.VerifyWebhookToken(webhookToken, r.Header.Get("X-CALLBACK-TOKEN")) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var payload xenditPayoutWebhook
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&payload); err != nil || payload.Data.PayoutID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := payouts.SettleGatewayPayout(r.Context(), payload.Data.PayoutID); err != nil {
			logger.Error("xendit payout webhook: reconciliation failed", "payout_id", payload.Data.PayoutID, "event", payload.Event, "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
