package domain

import "time"

type Product struct {
	ID                string
	OperatorID        string
	SeasonID          string
	Name              string
	Category          string
	Type              string
	PriceIDR          int64
	DurationDays      int32
	Description       string
	Inclusions        []string
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	PlatformMarginPct float64
	OperatorMarginPct float64
	AgentMarginPct    float64
}
