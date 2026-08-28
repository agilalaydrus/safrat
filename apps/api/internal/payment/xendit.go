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
	"net/url"
	"strings"
	"time"
)

// ErrNotConfigured is returned instead of silently no-op'ing, unlike the
// Firebase/Sentry/Resend pattern elsewhere in this app — pretending a
// payment invoice was created when it wasn't is actively dangerous, not
// a benign missing side-effect.
var ErrNotConfigured = errors.New("Xendit belum dikonfigurasi (XENDIT_SECRET_KEY kosong)")

const invoicesURL = "https://api.xendit.co/v2/invoices"
const payoutsURL = "https://api.xendit.co/v3/payouts"

// invoiceValidFor is how long a jamaah has to complete payment.
//
// Set explicitly rather than left to the gateway's default. Paying is not
// instant: a QRIS code has to be scanned in another app, a virtual account
// number has to be carried to a bank or a teller, and somebody buying at night
// may well finish in the morning. A short window turns an ordinary delay into a
// failed transaction and a jamaah who has to start over.
//
// Twenty-four hours is generous on purpose, and costs nothing: an unpaid
// invoice holds no stock and blocks nothing, while the poller keeps checking
// throughout, so a payment made at the twenty-third hour still settles.
const invoiceValidFor = 24 * time.Hour

type Client struct {
	secretKey   string
	invoicesURL string
	payoutsURL  string
	httpClient  *http.Client
}

func NewClient(secretKey string) *Client {
	return &Client{secretKey: secretKey, invoicesURL: invoicesURL, payoutsURL: payoutsURL, httpClient: &http.Client{Timeout: 20 * time.Second}}
}

// NewClientWithEndpoint points the client at a different invoices endpoint.
//
// It exists so tests can drive the real request and response handling against
// a stub rather than either calling Xendit or skipping the path entirely.
// Production always uses NewClient; nothing outside a test should call this.
func NewClientWithEndpoint(secretKey, endpoint string) *Client {
	return &Client{secretKey: secretKey, invoicesURL: endpoint, payoutsURL: endpoint, httpClient: &http.Client{Timeout: 20 * time.Second}}
}

func NewClientWithEndpoints(secretKey, invoiceEndpoint, payoutEndpoint string) *Client {
	return &Client{secretKey: secretKey, invoicesURL: invoiceEndpoint, payoutsURL: payoutEndpoint, httpClient: &http.Client{Timeout: 20 * time.Second}}
}

// Configured reports whether payments can actually be taken. Nil-safe: an
// unconfigured deployment leaves the client nil, and "no client" is precisely
// "not configured" — a panic there would turn a missing environment variable
// into a crash on a checkout request.
func (c *Client) Configured() bool { return c != nil && c.secretKey != "" }

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
		"invoice_duration":     int(invoiceValidFor.Seconds()),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal xendit invoice request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.invoicesURL, bytes.NewReader(body))
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

// InvoiceStatus is what Xendit itself says about an invoice, as opposed to
// what a webhook delivery claims.
type InvoiceStatus struct {
	ID         string
	ExternalID string
	Status     string
	Amount     int64
	PaidAmount int64
}

// FetchInvoice asks Xendit directly for an invoice's current state.
//
// A webhook delivery is a claim made by whoever could reach the endpoint. Even
// with a valid callback token — a static shared secret that travels on every
// delivery and lives in an env file — the safe move is to treat the delivery
// as nothing more than "go and check now", then settle from Xendit's own
// answer over an authenticated outbound TLS connection nobody else can forge.
//
// It also makes a missed delivery survivable: whatever polls this will find
// the payment eventually, where a dropped webhook leaves an order PENDING
// forever with nobody aware of it.
func (c *Client) FetchInvoice(ctx context.Context, invoiceID string) (*InvoiceStatus, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.invoicesURL+"/"+url.PathEscape(invoiceID), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.secretKey+":")))
	response, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("xendit: fetch invoice %s: status %d: %s", invoiceID, response.StatusCode, string(body))
	}
	var payload struct {
		ID         string `json:"id"`
		ExternalID string `json:"external_id"`
		Status     string `json:"status"`
		Amount     int64  `json:"amount"`
		PaidAmount int64  `json:"paid_amount"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("xendit: decode invoice %s: %w", invoiceID, err)
	}
	return &InvoiceStatus{
		ID: payload.ID, ExternalID: payload.ExternalID, Status: payload.Status,
		Amount: payload.Amount, PaidAmount: payload.PaidAmount,
	}, nil
}

type CreatePayoutRequest struct {
	ReferenceID, IdempotencyKey  string
	AmountIDR                    int64
	GivenName, Surname, Phone    string
	AccountHolder, AccountNumber string
	RoutingType, RoutingValue    string
	Description                  string
}

type Payout struct {
	ID                 string
	ReferenceID        string
	Status             string
	AmountIDR          int64
	ProcessorReference string
	FailureCode        string
}

// CreatePayout submits one B2C refund through Xendit Payouts v3. The caller
// owns the durable retry loop and must reuse both ReferenceID and
// IdempotencyKey; Xendit then returns the original payout instead of sending
// the money twice after an uncertain timeout.
func (c *Client) CreatePayout(ctx context.Context, req CreatePayoutRequest) (*Payout, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	if req.ReferenceID == "" || req.IdempotencyKey == "" || req.AmountIDR <= 0 {
		return nil, errors.New("invalid xendit payout request")
	}
	recipient := map[string]any{
		"type": "INDIVIDUAL", "given_name": req.GivenName, "surname": req.Surname,
		"relationship": "CUSTOMER",
		"address":      map[string]any{"country": "ID"},
		"account_details": map[string]any{
			"currency": "IDR", "account_country": "ID", "account_holder_name": req.AccountHolder,
			"account_number": req.AccountNumber, "routing_type_1": req.RoutingType, "routing_value_1": req.RoutingValue,
		},
	}
	if strings.TrimSpace(req.Phone) != "" {
		recipient["details"] = map[string]any{"personal_mobile_number": req.Phone}
	}
	body, err := json.Marshal(map[string]any{
		"reference_id":   req.ReferenceID,
		"recipient":      recipient,
		"payout_details": map[string]any{"source_currency": "IDR", "source_amount": req.AmountIDR, "destination_currency": "IDR"},
		"source_of_fund": "BUSINESS_REVENUE", "purpose_code": "REFUND", "description": req.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal xendit payout request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.payoutsURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-version", "2025-09-01")
	httpReq.Header.Set("idempotency-key", req.IdempotencyKey)
	httpReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.secretKey+":")))
	return c.doPayout(httpReq)
}

func (c *Client) FetchPayout(ctx context.Context, payoutID string) (*Payout, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.payoutsURL+"/"+url.PathEscape(payoutID), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("api-version", "2025-09-01")
	httpReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.secretKey+":")))
	return c.doPayout(httpReq)
}

func (c *Client) doPayout(req *http.Request) (*Payout, error) {
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call xendit payout: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("xendit payout API error (%d): %s", response.StatusCode, string(body))
	}
	var payload struct {
		PayoutID           string `json:"payout_id"`
		ReferenceID        string `json:"reference_id"`
		Status             string `json:"status"`
		SourceAmount       int64  `json:"source_amount"`
		ProcessorReference string `json:"processor_reference"`
		FailureCode        string `json:"failure_code"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse xendit payout response: %w", err)
	}
	return &Payout{ID: payload.PayoutID, ReferenceID: payload.ReferenceID, Status: payload.Status, AmountIDR: payload.SourceAmount, ProcessorReference: payload.ProcessorReference, FailureCode: payload.FailureCode}, nil
}
