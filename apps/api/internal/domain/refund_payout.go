package domain

import "time"

// RefundPayoutRequest is an instruction to return part of the balance held for
// a pilgrim. PAID requests are mirrored by one negative WITHDRAWAL ledger
// entry; REQUESTED/PROCESSING requests reserve funds but do not move them.
type RefundPayoutRequest struct {
	ID                string
	OperatorID        string
	PilgrimID         string
	PilgrimName       string
	PilgrimPhone      string
	AmountIDR         int64
	Method            string
	Note              string
	Status            string
	IdempotencyKey    string
	RequestedByUserID string
	ProcessedByUserID string
	ResolutionNote    string
	PaymentReference  string
	ProcessingAt      *time.Time
	ResolvedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PilgrimBalanceEntry struct {
	ID        string
	AmountIDR int64
	Kind      string
	Note      string
	OrderID   string
	CreatedAt time.Time
}
