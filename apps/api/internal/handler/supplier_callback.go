package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hajj-saas/api/internal/service"
)

// NewSupplierCallbackHandler receives a supplier's asynchronous result.
//
// A plain net/http route, not a Connect RPC: suppliers post whatever shape they
// like — JSON, form bodies, plain text — and would not speak Connect if asked.
// The body is read by that supplier's own rules rather than parsed here, so a
// supplier changing shape is a row edit rather than a deploy.
//
// Authenticated by a per-supplier token in the path. That token identifies
// *which* supplier is calling, so it must be unique across them — a shared one
// would let any supplier settle another's transactions.
func NewSupplierCallbackHandler(logger *slog.Logger, fulfilments *service.FulfilmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/webhooks/supplier/")
		if token == "" || strings.Contains(token, "/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Bounded: a supplier posting an unbounded body should not be able to
		// exhaust memory, and no legitimate result is anywhere near this size.
		body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// The reference can arrive in the query string or in a JSON body.
		// Suppliers differ, and both are common enough to accept.
		reference := r.URL.Query().Get("ref")
		if reference == "" {
			reference = r.URL.Query().Get("reference")
		}
		if reference == "" {
			var payload struct {
				Reference  string `json:"reference"`
				Ref        string `json:"ref"`
				OrderID    string `json:"order_id"`
				ExternalID string `json:"external_id"`
			}
			if json.Unmarshal(body, &payload) == nil {
				reference = firstNonEmpty(payload.Reference, payload.Ref, payload.OrderID, payload.ExternalID)
			}
		}

		result, err := fulfilments.ApplyCallback(r.Context(), token, reference, string(body))
		if err != nil {
			// 200 regardless of what we made of it, deliberately. A supplier
			// that gets an error retries, and a retry cannot fix a reference we
			// do not recognise or a rule we have not written — it only buries
			// the original in noise. The record is in the logs either way.
			logger.Warn("supplier callback not applied", "error", err.Error(), "reference", reference)
			w.WriteHeader(http.StatusOK)
			return
		}
		if len(result.Skipped) > 0 {
			logger.Warn("supplier rules skipped during callback",
				"order_id", result.OrderID, "skipped", len(result.Skipped))
		}
		logger.Info("supplier callback applied",
			"order_id", result.OrderID, "outcome", string(result.Outcome), "status", result.Status)
		w.WriteHeader(http.StatusOK)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
