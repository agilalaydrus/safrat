package domain

import "time"

// OrderRefund is one recorded refund event against a paid order. Refunds
// accumulate: an order may be refunded in parts, and the sum of its refunds
// can never exceed what was paid.
type OrderRefund struct {
	ID                    string
	OperatorID            string
	OrderID               string
	AmountIDR             int64
	CommissionReversedIDR int64
	Reason                string
	CreatedByUserID       string
	CreatedAt             time.Time
}

// RefundableOrder is the subset of an order the refund path reasons about,
// read under a lock so two concurrent refunds cannot both believe the same
// money is still available to return.
type RefundableOrder struct {
	ID                 string
	PilgrimID          string
	AgentID            string
	TotalPriceIDR      int64
	AgentCommissionIDR int64
	Status             string
	RefundedIDR        int64
	CommissionReversed int64
}
