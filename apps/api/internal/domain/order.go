package domain

import "time"

type Order struct {
	ID                 string
	OperatorID         string
	SeasonID           string
	PilgrimID          string
	PilgrimName        string
	ProductID          string
	ProductName        string
	AgentID            string
	AgentName          string
	Quantity           int32
	UnitPriceIDR       int64
	TotalPriceIDR      int64
	PlatformAmountIDR  int64
	OperatorAmountIDR  int64
	AgentCommissionIDR int64
	Status             string
	XenditInvoiceID    string
	XenditInvoiceURL   string
	PaidAt             *time.Time
	CreatedAt          time.Time
}
