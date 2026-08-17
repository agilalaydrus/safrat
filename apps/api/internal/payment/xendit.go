// Package payment wraps Xendit's Invoice API — the simplest integration
// Xendit offers: create an invoice, redirect the payer to Xendit's own
// hosted checkout page (VA/QRIS/e-wallet/card all handled there, none of
// it touches this codebase), then get a webhook when it's paid. No SDK
// dependency — it's two HTTP calls (create invoice, verify webhook token)
// against a stable, documented REST API.
package payment

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrNotConfigured is returned instead of silently no-op'ing, unlike the
// Firebase/Sentry/Resend pattern elsewhere in this app — pretending a
// payment invoice was created when it wasn't is actively dangerous, not
// a benign missing side-effect.
var ErrNotConfigured = errors.New("Xendit belum dikonfigurasi (XENDIT_SECRET_KEY kosong)")

const invoicesURL = "https://api.xendit.co/v2/invoices"

type Client struct {
	secretKey  string
	httpClient *http.Client
}

func NewClient(secretKey string) *Client {
	return &Client{secretKey: secretKey, httpClient: &http.Client{}}
}

func (c *Client) Configured() bool { return c.secretKey != "" }

type CreateInvoiceRequest struct {
	ExternalID         string // our order id — Xendit echoes this back on the webhook
	Amount             int64  // IDR, whole rupiah (no decimals)
	PayerEmail         string
	Description        string
	SuccessRedirectURL string
	FailureRedirectURL string
}

type Invoice struct {
	ID         string `json:"id"`
	InvoiceURL string `json:"invoice_url"`
	Status     string `json:"status"`
}

func (c *Client) CreateInvoice(ctx context.Context, req CreateInvoiceRequest) (*Invoice, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	body, err := json.Marshal(map[string]any{
		"external_id":          req.ExternalID,
		"amount":               req.Amount,
		"payer_email":          req.PayerEmail,
		"description":          req.Description,
		"currency":             "IDR",
		"success_redirect_url": req.SuccessRedirectURL,
		"failure_redirect_url": req.FailureRedirectURL,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal xendit invoice request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, invoicesURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build xendit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Xendit auth: HTTP Basic with the secret key as username, empty password.
	httpReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.secretKey+":")))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call xendit: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read xendit response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("xendit API error (%d): %s", resp.StatusCode, string(respBody))
	}
	var invoice Invoice
	if err := json.Unmarshal(respBody, &invoice); err != nil {
		return nil, fmt.Errorf("parse xendit response: %w", err)
	}
	return &invoice, nil
}

// VerifyWebhookToken checks the X-CALLBACK-TOKEN header Xendit sends on
// every webhook delivery against the token configured for this account
// (Xendit Dashboard > Settings > Webhooks) — not a signature, a shared
// secret comparison, which is Xendit's own documented mechanism.
func VerifyWebhookToken(configuredToken, headerToken string) bool {
	if configuredToken == "" || headerToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(configuredToken), []byte(headerToken)) == 1
}
