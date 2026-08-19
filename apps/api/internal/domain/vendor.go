package domain

import "time"

type VendorContract struct {
	ID                   string
	OperatorID           string
	SeasonID             string
	VendorName           string
	VendorType           string
	ContractNumber       string
	CommittedUnits       int32
	ConfirmedUnits       int32
	ConfirmationDeadline *time.Time
	RatePerUnitIDR       int64
	TotalValueIDR        int64
	DepositAmountIDR     int64
	DepositPaid          bool
	Status               string
	SLAHealth            string
	Notes                string
	ContactName          string
	ContactPhone         string
	CreatedAt            time.Time
}

type ContractEvent struct {
	ID          string
	ContractID  string
	EventType   string
	Description string
	RecordedBy  string
	CreatedAt   time.Time
}
