package domain

import "time"

type CancellationPolicy struct {
	ID         string
	OperatorID string
	SeasonID   string
	Name       string
	MinDays    int32
	RefundPct  float64
	SortOrder  int32
}

type PilgrimCancellation struct {
	ID              string
	PilgrimID       string
	PilgrimName     string
	OperatorID      string
	SeasonID        string
	Reason          string
	DaysBefore      int32
	RefundPct       float64
	RefundAmountIDR int64
	TotalPaidIDR    int64
	CancelledBy     string
	CancelledAt     time.Time
	PolicyID        string
}

type CancellationPreview struct {
	PilgrimID       string
	PilgrimName     string
	DaysBefore      int32
	RefundPct       float64
	TotalPaidIDR    int64
	RefundAmountIDR int64
	PolicyName      string
}
