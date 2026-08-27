package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/supplier"
)

// FulfilmentService decides what a supplier's answer means for a transaction.
//
// It is the counterpart to payment settlement: that one asks "did the jamaah
// pay", this one asks "did the thing they paid for arrive". Keeping them apart
// is what makes "paid but undelivered" a state the system can hold and show,
// rather than a gap between two booleans.
type FulfilmentService struct {
	fulfilments *repository.FulfilmentRepository
	suppliers   *repository.SupplierRepository
	costs       *repository.SupplierCostRepository
	orders      *repository.OrderRepository
	// http is shared so connections are reused across sends. Per-supplier
	// timeouts are applied to the request context rather than to this client,
	// which is why it carries none of its own.
	http *http.Client
	// queue is the fast path: a jamaah buying pulsa expects it in seconds, and
	// waiting for a periodic sweep would not meet that. Nil is valid — without
	// Redis every fulfilment simply waits for the sweep, which is slower but
	// never wrong.
	queue FulfilmentQueue
}

// FulfilmentQueue hands one order to the worker for immediate sending.
type FulfilmentQueue interface {
	EnqueueDispatch(ctx context.Context, orderID string) error
}

// AttachQueue enables immediate dispatch. Without it the service still works,
// just at sweep latency.
func (s *FulfilmentService) AttachQueue(queue FulfilmentQueue) {
	s.queue = queue
}

func NewFulfilmentService(fulfilments *repository.FulfilmentRepository, suppliers *repository.SupplierRepository,
	costs *repository.SupplierCostRepository, orders *repository.OrderRepository) *FulfilmentService {
	return &FulfilmentService{
		fulfilments: fulfilments, suppliers: suppliers, costs: costs, orders: orders,
		http: &http.Client{},
	}
}

// Dispatch sends one pending fulfilment to its supplier and applies whatever
// comes back.
//
// The claim comes first and is a conditional UPDATE, so a second worker holding
// the same order finds nothing to claim and stops. Everything after that point
// has already been counted as an attempt: a request that was sent and then lost
// must never look like one that was never sent, because the difference is
// whether the jamaah's pulsa is already on its way.
//
// Any answer at all is logged with the request beside it, redacted. A response
// no rule recognises leaves the fulfilment for a human rather than failing it —
// the supplier may well have delivered.
func (s *FulfilmentService) Dispatch(ctx context.Context, pending repository.PendingDispatch) error {
	claimed, err := s.fulfilments.Claim(ctx, pending.OrderID)
	if err != nil {
		return err
	}
	if !claimed {
		// Somebody else has it, or it is no longer pending. Not an error.
		return nil
	}

	endpoint, err := s.suppliers.EndpointFor(ctx, pending.SupplierID)
	if err != nil {
		return s.failDispatch(ctx, pending, "", "supplier tidak dapat dihubungi: "+err.Error())
	}
	built, err := supplier.Build(endpoint, supplier.Order{
		Reference: pending.OrderID, SKU: pending.SupplierSKU,
		AmountIDR: pending.AmountIDR, Destination: pending.Destination,
	})
	if err != nil {
		// Configuration is wrong — a bad base URL, an unknown signature recipe,
		// an address pointing inside the network. Not something a retry fixes,
		// so it goes straight to a human.
		return s.failDispatch(ctx, pending, "", "permintaan tidak dapat dibentuk: "+err.Error())
	}

	timeout := endpoint.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := s.http.Do(built.Request.WithContext(sendCtx))
	if err != nil {
		// The request may have arrived. Treating a transport failure as a
		// definite non-delivery is how a jamaah gets refunded for pulsa that
		// was sent, so this waits for a person or a later callback.
		return s.failDispatch(ctx, pending, built.Loggable,
			"tidak ada jawaban dari supplier: "+err.Error())
	}
	defer func() { _ = response.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	body := string(raw)

	rules, err := s.suppliers.ActiveRulesFor(ctx, pending.SupplierID)
	if err != nil {
		return err
	}
	reading := supplier.Read(rules, body)

	status := ""
	switch reading.Outcome {
	case supplier.OutcomeSuccess:
		status = "DELIVERED"
	case supplier.OutcomeFailed:
		status = "FAILED"
	case supplier.OutcomePending:
		// Accepted, not yet delivered. A callback or the stuck sweep decides.
	default:
		status = "NEEDS_REVIEW"
	}
	if status != "" {
		if _, err := s.fulfilments.Settle(ctx, pending.OrderID, status, reading.Reference, failureText(reading)); err != nil {
			return err
		}
	}
	if reading.CostIDR != nil && status == "DELIVERED" {
		if err := s.recordCost(ctx, pending.OrderID, pending.SupplierID, *reading.CostIDR, reading.Reference); err != nil {
			// The delivery stands regardless; only the price floor is missing.
			sentry.CaptureException(fmt.Errorf("FulfilmentService.Dispatch: record cost: %w", err))
		}
	}

	httpStatus := int32(response.StatusCode)
	s.logExchange(ctx, pending.SupplierID, pending.OrderID, repository.LogEntry{
		Direction: "REQUEST", Endpoint: built.Endpoint, RequestBody: built.Loggable,
		ResponseBody: body, HTTPStatus: &httpStatus, Outcome: string(reading.Outcome),
		SupplierReference: reading.Reference, CostIDR: reading.CostIDR,
		Error: skippedSummary(reading.SkippedRules),
	}, reading.RuleID)
	return nil
}

// failDispatch parks a fulfilment a person has to look at, and records why.
//
// Never FAILED: everything reaching here is either a configuration fault or an
// unanswered request, and neither proves the supplier did not deliver.
func (s *FulfilmentService) failDispatch(ctx context.Context, pending repository.PendingDispatch, request, reason string) error {
	if _, err := s.fulfilments.Settle(ctx, pending.OrderID, "NEEDS_REVIEW", "", reason); err != nil {
		return err
	}
	s.logExchange(ctx, pending.SupplierID, pending.OrderID, repository.LogEntry{
		Direction: "REQUEST", RequestBody: request, Outcome: string(supplier.OutcomeUnmatched), Error: reason,
	}, "")
	return nil
}

// Open records that a paid order now owes a delivery.
//
// Called from the payment path. A product with no active route opens the
// fulfilment anyway, as NEEDS_REVIEW — the jamaah has paid either way, and a
// paid order with no record of owing anything is precisely how a transaction
// disappears.
func (s *FulfilmentService) Open(ctx context.Context, orderID, operatorID string) {
	if s == nil || s.fulfilments == nil {
		return
	}
	supplierID := ""
	if route, err := s.suppliers.RouteForOrder(ctx, orderID); err == nil {
		supplierID = route.SupplierID
	}
	created, err := s.fulfilments.Open(ctx, orderID, operatorID, supplierID)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("FulfilmentService.Open: %w", err))
		return
	}
	if created && supplierID != "" && s.queue != nil {
		// Sent now, not at the next sweep. A failure here is survivable and
		// deliberately not treated as a reason to fail anything: the payment
		// has settled, the row already records that a delivery is owed, and the
		// sweep will find it. Losing a payment because Redis blinked would be
		// far worse than pulsa arriving a minute late.
		if err := s.queue.EnqueueDispatch(ctx, orderID); err != nil {
			sentry.CaptureException(fmt.Errorf("FulfilmentService.Open: enqueue dispatch: %w", err))
		}
	}
	if created && supplierID == "" {
		// Nothing can be sent, so somebody has to decide what happens. Saying
		// so immediately beats leaving it PENDING against a supplier that does
		// not exist.
		if _, err := s.fulfilments.Settle(ctx, orderID, "NEEDS_REVIEW", "",
			"Produk belum punya routing supplier aktif"); err != nil {
			sentry.CaptureException(fmt.Errorf("FulfilmentService.Open: flag unrouted: %w", err))
		}
	}
}

// CallbackResult is what a supplier's asynchronous notification was understood
// to mean, for logging and for the caller's reply.
type CallbackResult struct {
	OrderID   string
	Outcome   supplier.Outcome
	Status    string
	RuleID    string
	Skipped   []supplier.SkippedRule
	Reference string
}

// ApplyCallback settles a fulfilment from what a supplier posted back.
//
// The body is read by the supplier's own rules rather than a parser written for
// them, so a supplier changing shape is a row edit. Whatever happens, the raw
// body is logged beside the reading: a rule that turns out to be wrong can then
// be re-read against what actually arrived.
//
// An unreadable response becomes NEEDS_REVIEW, never FAILED. Treating it as a
// failure would refund a jamaah for something the supplier may well have
// delivered, and that is not reversible.
func (s *FulfilmentService) ApplyCallback(ctx context.Context, token, reference, body string) (*CallbackResult, error) {
	supplierID, err := s.fulfilments.SupplierByCallbackToken(ctx, token)
	if err != nil {
		return nil, err
	}
	orderID, err := s.fulfilments.FindOrderByReference(ctx, supplierID, strings.TrimSpace(reference))
	if err != nil {
		// Logged even though it settles nothing: a callback we cannot place is
		// either a supplier bug or a reference we failed to store, and both are
		// invisible without a record.
		s.logExchange(ctx, supplierID, "", repository.LogEntry{
			Direction: "CALLBACK", ResponseBody: body, Outcome: string(supplier.OutcomeUnmatched),
			Error: "tidak ada transaksi dengan referensi " + reference,
		}, "")
		return nil, err
	}

	rules, err := s.suppliers.ActiveRulesFor(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	reading := supplier.Read(rules, body)

	result := &CallbackResult{
		OrderID: orderID, Outcome: reading.Outcome, RuleID: reading.RuleID,
		Skipped: reading.SkippedRules, Reference: reading.Reference,
	}
	switch reading.Outcome {
	case supplier.OutcomeSuccess:
		result.Status = "DELIVERED"
	case supplier.OutcomeFailed:
		result.Status = "FAILED"
	case supplier.OutcomePending:
		// Still out. Nothing to settle, and nothing to worry about yet — the
		// stuck sweep will notice if it stays this way.
		result.Status = ""
	default:
		result.Status = "NEEDS_REVIEW"
	}

	if result.Status != "" {
		if _, err := s.fulfilments.Settle(ctx, orderID, result.Status, reading.Reference, failureText(reading)); err != nil {
			return nil, err
		}
	}

	// What the supplier charged, when they say. Recorded against the order, so
	// a retried callback reports the same purchase rather than a second one.
	if reading.CostIDR != nil && result.Status == "DELIVERED" {
		if err := s.recordCost(ctx, orderID, supplierID, *reading.CostIDR, reading.Reference); err != nil {
			sentry.CaptureException(fmt.Errorf("FulfilmentService.ApplyCallback: record cost: %w", err))
		}
	}

	s.logExchange(ctx, supplierID, orderID, repository.LogEntry{
		Direction: "CALLBACK", ResponseBody: body, Outcome: string(reading.Outcome),
		SupplierReference: reading.Reference, CostIDR: reading.CostIDR,
		Error: skippedSummary(reading.SkippedRules),
	}, reading.RuleID)
	return result, nil
}

func (s *FulfilmentService) recordCost(ctx context.Context, orderID, supplierID string, cost int64, reference string) error {
	order, err := s.orders.GetAny(ctx, orderID)
	if err != nil {
		return err
	}
	return s.costs.RecordObservation(ctx, order.OperatorID, order.ProductID, orderID, cost, reference)
}

func (s *FulfilmentService) logExchange(ctx context.Context, supplierID, orderID string, entry repository.LogEntry, ruleID string) {
	if err := s.suppliers.RecordExchange(ctx, entry, supplierID, orderID, ruleID); err != nil {
		sentry.CaptureException(fmt.Errorf("FulfilmentService: record exchange: %w", err))
	}
}

// ResolveManually is a human closing a case the supplier never made readable.
func (s *FulfilmentService) ResolveManually(ctx context.Context, orderID, status, userID, note string) error {
	switch status {
	case "DELIVERED", "FAILED":
	default:
		return connect.NewError(connect.CodeInvalidArgument,
			errors.New("hanya dapat diselesaikan sebagai terkirim atau gagal"))
	}
	if err := s.fulfilments.Resolve(ctx, orderID, status, userID, note); err != nil {
		if errors.Is(err, apperror.ErrFailedPrecondition) {
			return connect.NewError(connect.CodeFailedPrecondition,
				errors.New("hanya fulfilment yang masih terbuka yang dapat diselesaikan"))
		}
		return err
	}
	return nil
}

func failureText(reading supplier.Reading) string {
	if reading.Outcome == supplier.OutcomeUnmatched {
		return "Respons supplier tidak dikenali aturan mana pun"
	}
	return ""
}

// skippedSummary turns unusable rules into something a person reading the log
// will notice. Coverage quietly disappearing is how a supplier drifts into
// producing nothing but unreadable answers with nobody realising the rules
// stopped working rather than the supplier changing.
func skippedSummary(skipped []supplier.SkippedRule) string {
	if len(skipped) == 0 {
		return ""
	}
	parts := make([]string, 0, len(skipped))
	for _, rule := range skipped {
		parts = append(parts, rule.RuleID+": "+rule.Reason)
	}
	return "Aturan dilewati — " + strings.Join(parts, "; ")
}
