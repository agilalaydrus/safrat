package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
)

// xenditWebhookPayload only lists the fields this handler actually reads —
// Xendit's real payload has many more (payment channel, fees, etc.) that
// nothing here needs yet.
type xenditWebhookPayload struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Amounts are deliberately absent. The delivery is not trusted to say how much
// was paid — that comes from Xendit's own API in SettleFromGateway — so
// parsing it here would only invite somebody to use it.

// NewXenditWebhookHandler is a plain net/http handler, not a Connect RPC —
// Xendit calls back over a normal webhook POST, it doesn't speak Connect.
// Registered directly on the mux in main.go.
func NewXenditWebhookHandler(logger *slog.Logger, orders *repository.OrderRepository, orderService *service.OrderService, subscriptions *repository.SubscriptionRepository, webhookToken string, source *WebhookSourceGuard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Source first, so a delivery from an unexpected address is refused
		// before its body is read or its token compared at all.
		if !source.Allows(r) {
			logger.Warn("xendit webhook: rejected delivery from unexpected source", "address", clientAddress(r))
			w.WriteHeader(http.StatusForbidden)
			return
		}
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

		// One webhook endpoint serves two things now: pilgrim orders and
		// operator subscriptions. A subscription invoice is recognised by the
		// gateway id we stored when issuing it, so the two can never be
		// confused — an id belongs to one or the other, never both.
		if handled := settleSubscription(ctx, logger, subscriptions, payload); handled {
			w.WriteHeader(http.StatusOK)
			return
		}

		var err error
		// The delivery's own status field is deliberately not acted on. It is
		// a claim by whoever reached this URL; SettleFromGateway asks Xendit
		// what actually happened and settles from that answer. The only thing
		// taken from the payload is which invoice to go and look at.
		//
		// FAILED is the exception: Xendit does not keep a "failed" invoice to
		// fetch, so it is applied directly. It only ever closes an order and
		// reverses commission — the direction that cannot pay anyone.
		if payload.Status == "FAILED" {
			err = orderService.MarkStatusByInvoiceID(ctx, payload.ID, "FAILED")
		} else {
			err = orderService.SettleFromGateway(ctx, payload.ID)
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

// settleSubscription applies a delivery to an operator subscription and reports
// whether it belonged to one. Settlement itself is idempotent, so a redelivered
// payment cannot buy a second period.
func settleSubscription(ctx context.Context, logger *slog.Logger, subscriptions *repository.SubscriptionRepository, payload xenditWebhookPayload) bool {
	if subscriptions == nil {
		return false
	}
	var err error
	switch payload.Status {
	case "PAID":
		err = subscriptions.MarkPaidByExternalID(ctx, payload.ID)
	case "EXPIRED":
		err = subscriptions.CloseByExternalID(ctx, payload.ID, "EXPIRED")
	case "FAILED":
		err = subscriptions.CloseByExternalID(ctx, payload.ID, "CANCELLED")
	default:
		return false
	}
	if errors.Is(err, apperror.ErrNotFound) {
		// Not a subscription invoice — fall through to the order path.
		return false
	}
	if err != nil {
		// Logged rather than surfaced: returning non-200 makes Xendit retry
		// forever, and the invoice can be reconciled from the dashboard.
		logger.Error("xendit webhook: subscription not settled", "invoice_id", payload.ID, "status", payload.Status, "error", err)
	}
	return true
}
