package domain

import "time"

type Order struct {
	ID                 string
	OperatorID         string
	SeasonID           string
	PilgrimID          string
	PilgrimName        string
	BuyerAgentID       string
	BuyerKind          string
	BuyerName          string
	ProductID          string
	ProductName        string
	AgentID            string
	AgentName          string
	Quantity           int32
	UnitPriceIDR       int64
	BasePriceIDR       int64
	OperatorMarkupIDR  int64
	AgentMarkupIDR     int64
	TotalPriceIDR      int64
	PlatformAmountIDR  int64
	OperatorAmountIDR  int64
	AgentCommissionIDR int64
	Status             string
	XenditInvoiceID    string
	XenditInvoiceURL   string
	PaidAmountIDR      *int64
	HeldReason         string
	ReceiptNumber      string
	Destination        string
	PaidAt             *time.Time
	CreatedAt          time.Time
}

// PilgrimTransaction is one line in a jamaah's own transaction history.
type PilgrimTransaction struct {
	OrderID       string
	ProductName   string
	Quantity      int32
	AmountIDR     int64
	Status        string
	CreatedAt     time.Time
	PaidAt        *time.Time
	RefundedIDR   int64
	RefundedAt    *time.Time
	RefundReason  string
	CheckoutURL   string
	ReceiptNumber string
	OperatorName  string
}
