package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/service"
)

// bankFeedMaxBody bounds what a feed can post. A scraper with a bug can send a
// whole page; this is a batch of numbers.
const bankFeedMaxBody = 1 << 20

type bankFeedPayload struct {
	Source    string `json:"source"`
	Mutations []struct {
		ExternalID  string `json:"external_id"`
		AmountIDR   int64  `json:"amount_idr"`
		Description string `json:"description"`
		OccurredAt  string `json:"occurred_at"`
	} `json:"mutations"`
}

// NewBankFeedHandler receives credits from a bank API poller or a scraper.
//
// Authenticated by HMAC over the body, not by a bearer token in a header. A
// token proves only that the sender knows the token; a signature over the body
// also proves the body was not altered between them and us — which matters
// because this input settles invoices.
//
// Requests are refused when BANK_FEED_SECRET is unset rather than accepted
// unauthenticated. An endpoint that grants subscription access must not be open
// because a variable was forgotten.
func NewBankFeedHandler(logger *slog.Logger, mutations *service.BankMutationService, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(secret) == "" {
			logger.Error("bank feed called but BANK_FEED_SECRET is not set")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, bankFeedMaxBody))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))
		// Constant time: a byte-by-byte compare leaks how much of a guess was
		// right, which is enough to forge a signature given enough attempts.
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Signature")), []byte(expected)) != 1 {
			logger.Warn("bank feed signature rejected", "remote", r.RemoteAddr)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var payload bankFeedPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload.Source != "API" && payload.Source != "SCRAPER" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		recorded, matched := 0, 0
		for _, m := range payload.Mutations {
			occurred, parseErr := time.Parse(time.RFC3339, m.OccurredAt)
			if parseErr != nil {
				occurred = time.Now()
			}
			result, err := mutations.Ingest(r.Context(), payload.Source, m.ExternalID, m.AmountIDR, m.Description, occurred)
			if err != nil {
				// One bad entry must not discard the rest of the batch: the
				// others are money that arrived, and dropping them would leave
				// it unaccounted for.
				logger.Error("bank mutation rejected", "external_id", m.ExternalID, "error", err)
				continue
			}
			if result.Recorded {
				recorded++
			}
			if result.Matched {
				matched++
			}
		}

		logger.Info("bank feed processed", "source", payload.Source,
			"received", len(payload.Mutations), "recorded", recorded, "matched", matched)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{
			"received": len(payload.Mutations), "recorded": recorded, "matched": matched,
		})
	}
}
