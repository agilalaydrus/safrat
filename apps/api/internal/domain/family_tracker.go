package domain

import "time"

// FamilyStatus is deliberately minimal — see family_tracker.proto for the
// privacy rationale. Never add a field here without checking it against
// that list first.
type FamilyStatus struct {
	FirstName      string
	PaymentStatus  string
	HotelCheckedIn bool
	PilgrimStatus  string
	SeasonName     string
	DepartureDate  time.Time
	GroupName      string
	LeaderName     string
	LastLocationAt *time.Time
	HasActiveSOS   bool
}
