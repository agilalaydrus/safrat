package domain

import "time"

type VendorPayment struct {
	ID          string
	OperatorID  string
	SeasonID    string
	VendorName  string
	Category    string
	Description string
	AmountIDR   int64
	DueDate     time.Time
	Status      string
	PaidAt      *time.Time
	CreatedAt   time.Time
}

type CashFlowSummary struct {
	TotalCollectedIDR   int64
	TotalCommittedIDR   int64
	TotalPaidOutIDR     int64
	TotalOutstandingIDR int64
	TotalOverdueIDR     int64
	DueNext30DaysIDR    int64
	UnpaidPilgrimCount  int64
}

type MonthlyProjectionEntry struct {
	Month                time.Time
	VendorObligationsIDR int64
	PaymentCount         int64
}
