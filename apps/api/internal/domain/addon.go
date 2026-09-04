package domain

import "time"

// AddonItem is one entry in a season's add-on catalog — a fixed-price extra
// like an executive seat or a zamzam water shipment.
type AddonItem struct {
	ID           string
	OperatorID   string
	SeasonID     string
	Name         string
	UnitPriceIDR int64
	IsActive     bool
	CreatedAt    time.Time
}

// PilgrimAddon is one pilgrim's holding of one add-on type. UnitPriceIDR is a
// snapshot taken at assignment time — it does not track AddonItem's price
// afterward, so a later catalog price change never rewrites what this
// pilgrim already agreed to pay.
type PilgrimAddon struct {
	ID           string
	OperatorID   string
	PilgrimID    string
	PilgrimName  string
	AddonItemID  string
	AddonName    string
	GroupName    string
	Quantity     int32
	UnitPriceIDR int64
	TotalIDR     int64
	Paid         bool
	Notes        string
	CreatedAt    time.Time
}
