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

// RouteReadiness is whether a product can actually reach a supplier.
//
// Carried alongside the price because for a digital product the two are the
// same question: an unroutable product cannot be sold at any price, and
// discovering that only after taking payment leaves a jamaah holding a paid
// order nothing can deliver.
type RouteReadiness struct {
	// Required is false for anything the operator fulfils themselves — travel
	// packages, and equipment handed over by hand. Those have no supplier to
	// route to, so absent routing is not a fault.
	Required bool

	Exists         bool
	Active         bool
	SupplierStatus string
}

// Refusal explains why this product cannot be sold, or returns empty when it
// can. The three cases are separated deliberately: each is a different fault
// with a different fix, and "tidak bisa dijual" alone sends everyone looking
// in the wrong place.
//
// Every message names TawafiqHub, because routing is platform-owned and none
// of these is something a travel can fix in their own dashboard. A refusal
// that a reader cannot act on is a dead end — it has to say who to ask.
func (r RouteReadiness) Refusal() string {
	if !r.Required {
		return ""
	}
	if !r.Exists {
		return "produk belum diatur routing-nya ke supplier; hubungi TawafiqHub untuk mengaktifkan"
	}
	if !r.Active {
		return "routing produk ini sedang dinonaktifkan oleh TawafiqHub"
	}
	if r.SupplierStatus != "ACTIVE" {
		return "supplier produk ini sedang tidak aktif; TawafiqHub sedang menanganinya"
	}
	return ""
}

// IsPlatformOwned reports a product supplied by TawafiqHub rather than by the
// travel selling it. Those carry no operator and no season, and no tenant may
// edit one.
func (p *Product) IsPlatformOwned() bool {
	return p != nil && p.OperatorID == ""
}

// RoutingRequired reports whether a category is delivered by calling a
// supplier. Equipment is excluded on purpose: it also creates a fulfilment,
// but one a person completes by hand, so it has no route to be missing.
func RoutingRequired(category string) bool {
	return category == "ROAMING_DATA" || category == "PPOB_CREDIT"
}
