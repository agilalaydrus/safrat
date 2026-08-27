package domain

import "time"

type Product struct {
	ID           string
	OperatorID   string
	SeasonID     string
	Name         string
	Category     string
	Type         string
	PriceIDR     int64
	DurationDays int32
	Description  string
	Inclusions   []string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// Basis points (1500 = 15.00%). Integers so the split is exact integer
	// arithmetic; a float multiplier lost a rupiah on about one order in two
	// hundred, always downward.
	// SupplierCostIDR is what this product costs to supply, when known. Nil
	// means nobody has said and nothing has been observed — the price cannot
	// be checked against a floor that does not exist.
	// Code is what a person quotes. NominalIDR is the face value the customer
	// receives, nil when the product has none — a travel package delivers a
	// journey, not an amount.
	Code               string
	NominalIDR         *int64
	SupplierCostIDR    *int64
	SupplierCostSource string
	// BasePriceIDR is what TawafiqHub charges the travel for this product,
	// and the floor every other level is added to. Nil means unset, which is
	// refused at sale — a product with no base has no price.
	//
	// Distinct from SupplierCostIDR, which is what TawafiqHub pays. The gap
	// between them is the platform's own margin.
	BasePriceIDR *int64

	PlatformMarginBps  int32
	OperatorMarginBps  int32
	AgentMarginBps     int32
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

// PriceLevels is the per-unit price built from the bottom up. Every level is
// present on every product; a level nobody has touched is zero, and zero is a
// real setting rather than a missing one.
type PriceLevels struct {
	// BasePriceIDR is what TawafiqHub charges the travel. Nil means unset,
	// which is refused — distinct from zero, which would mean free.
	BasePriceIDR *int64

	// OperatorMarkupIDR is what the travel adds. Base + this is the agent
	// price.
	OperatorMarkupIDR int64

	// AgentMarkupIDR is the agent level. Part of the jamaah price whether or
	// not a referrer exists, so a referred jamaah is never quoted more than a
	// walk-in for the same product.
	AgentMarkupIDR int64

	// Configured records that a markup row was actually found. Without it the
	// two zero markups above are indistinguishable from an unconfigured
	// product.
	Configured bool
}
