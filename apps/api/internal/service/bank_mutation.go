package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/repository"
)

// BankMutationService takes credits from a bank feed and settles what they pay
// for.
//
// The feed is untrusted input. It is a bank's API on a good day and a scraper
// reading a web page on a normal one, so nothing here believes a mutation
// because it says so: a credit settles an invoice only when its amount matches
// one exactly, and every credit is recorded whether it matched or not.
//
// Manual confirmation stays. When the API is down or the scraper breaks —
// which it will, because it is reading somebody else's HTML — a person reads
// the statement and settles the invoice by hand. Automation removes the routine
// case; it does not remove the fallback.
// OperatorNotifier tells a travel's staff something happened. An interface
// rather than the pusher itself, so settlement does not depend on Firebase
// being configured — with no notifier the money still moves.
type OperatorNotifier interface {
	NotifyOperatorStaff(ctx context.Context, operatorID, title, body string) error
}

type BankMutationService struct {
	subscriptions *repository.SubscriptionRepository
	audit         *repository.AuditRepository
	notifier      OperatorNotifier
}

func NewBankMutationService(subscriptions *repository.SubscriptionRepository, audit *repository.AuditRepository, notifier OperatorNotifier) *BankMutationService {
	return &BankMutationService{subscriptions: subscriptions, audit: audit, notifier: notifier}
}

// tellOperator says a transfer landed.
//
// Until this existed a travel transferred money and then waited with no signal
// at all — and with automatic matching, an invoice can now settle at three in
// the morning with nobody watching. Silence after a payment reads as a payment
// that did not arrive.
//
// Failures are logged by the pusher and never returned. The money has already
// moved and the access is already granted; refusing to finish that because a
// notification could not be delivered would undo a settlement over a message.
func (s *BankMutationService) tellOperator(ctx context.Context, operatorID string, amountIDR int64) {
	if s.notifier == nil || operatorID == "" {
		return
	}
	_ = s.notifier.NotifyOperatorStaff(ctx, operatorID,
		"Pembayaran diterima",
		"Transfer "+rupiah(amountIDR)+" sudah kami terima dan langganan Anda diperpanjang.")
}

// IngestResult says what happened to one credit, in the words the feed's
// operator needs: was it new, and did it pay for anything.
type IngestResult struct {
	MutationID string
	Recorded   bool
	Matched    bool
	InvoiceID  string
}

// Ingest records a credit and settles the invoice it pays for, if any.
//
// Order matters and is deliberate: record first, match second. A credit that
// matches nothing is the most important row this produces — money arrived and
// nothing accounts for it — so it must be stored before any matching is
// attempted, or a matching failure would lose the fact that money came in.
func (s *BankMutationService) Ingest(ctx context.Context, source, externalID string, amountIDR int64, description string, occurredAt time.Time) (IngestResult, error) {
	switch source {
	case "API", "SCRAPER", "MANUAL":
	default:
		return IngestResult{}, apperror.ErrValidation
	}
	if amountIDR <= 0 || strings.TrimSpace(externalID) == "" {
		return IngestResult{}, apperror.ErrValidation
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	mutation, recorded, err := s.subscriptions.RecordMutation(ctx, repository.BankMutation{
		ExternalID: externalID, Source: source, AmountIDR: amountIDR,
		Description: description, OccurredAt: occurredAt,
	})
	if err != nil {
		return IngestResult{}, err
	}

	result := IngestResult{MutationID: mutation.ID, Recorded: recorded}

	// A redelivery of something already settled is not an error and not a
	// second settlement. Reporting what the row already says is the whole
	// point of storing it.
	if mutation.Status != "UNMATCHED" {
		result.Matched = mutation.Status == "MATCHED"
		result.InvoiceID = mutation.MatchedInvoiceID
		return result, nil
	}

	// Exact amounts only. The unique suffix is the entire identifying
	// mechanism, so anything approximate would be guessing which travel paid —
	// and crediting the wrong one is far worse than leaving a credit for a
	// person to place.
	invoiceID, operatorID, _, err := s.subscriptions.FindPayableByAmount(ctx, amountIDR)
	if errors.Is(err, apperror.ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	settled, err := s.subscriptions.SettleInvoiceWithMutation(ctx, mutation.ID, invoiceID, "", "cocok otomatis dari "+source)
	if err != nil {
		return result, err
	}
	if !settled {
		// Something else claimed one of the two rows first. The credit stays
		// unmatched, which is honest: it is still unaccounted for.
		return result, nil
	}

	_ = s.audit.Write(ctx, operatorID, "system", "bank_transfer_matched", "subscription_invoice", invoiceID,
		"cocok otomatis dengan mutasi "+externalID+" dari "+source)
	s.tellOperator(ctx, operatorID, amountIDR)

	result.Matched = true
	result.InvoiceID = invoiceID
	return result, nil
}

// SettleManually attaches a recorded credit to a specific invoice.
//
// This is the fallback the automatic path cannot replace: a scraper that
// mis-read an amount, a transfer that arrived with a fee deducted, a bank that
// numbered two entries the same. A person decides, and the decision is tied to
// a credit that actually exists rather than to a number somebody typed —
// which is what separates this from confirming by amount alone.
// SettleManuallyFor is SettleManually with the operator known, so the travel
// can be told. Split out because the caller already looked the operator up and
// a second lookup after settlement would find nothing — the invoice is no
// longer pending by then.
func (s *BankMutationService) SettleManuallyFor(ctx context.Context, mutationID, invoiceID, userID, note, operatorID string, amountIDR int64) error {
	if err := s.SettleManually(ctx, mutationID, invoiceID, userID, note); err != nil {
		return err
	}
	s.tellOperator(ctx, operatorID, amountIDR)
	return nil
}

func (s *BankMutationService) SettleManually(ctx context.Context, mutationID, invoiceID, userID, note string) error {
	if !isUUID(mutationID) || !isUUID(invoiceID) || strings.TrimSpace(note) == "" {
		return connect.NewError(connect.CodeInvalidArgument,
			errors.New("sertakan alasan: pencocokan manual tidak dikonfirmasi oleh apa pun di luar sistem"))
	}
	settled, err := s.subscriptions.SettleInvoiceWithMutation(ctx, mutationID, invoiceID, userID, strings.TrimSpace(note))
	if err != nil {
		return err
	}
	if !settled {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("mutasi atau tagihan ini sudah tidak terbuka; muat ulang daftarnya"))
	}
	return nil
}
