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
	// Only meaningful when Category == "TRAVEL_PACKAGE".
	ItineraryDays   []ItineraryDay
	HotelIDs        []string
	HotelNames      []string // parallel to HotelIDs, for display without a second lookup
	DefaultKloterID string
}

type ItineraryDay struct {
	DayNumber     int32
	Title         string
	City          string
	Activities    string
	MealBreakfast bool
	MealLunch     bool
	MealDinner    bool
}
