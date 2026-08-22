package domain

import "time"

// JourneyStatuses is the canonical, ordered end-to-end pilgrim trip
// lifecycle. Umrah trips use a subset of the same order (skip the
// Hajj-only Arafah/Muzdalifah/Mina/BackInMakkah steps) — status transitions
// are enforced as monotonically forward through this order, not "exactly
// the next index", so a shorter Umrah lifecycle is never blocked.
var JourneyStatuses = []string{
	"REGISTERED", "DOCUMENT_VERIFIED", "PRE_DEPARTURE", "DEPARTED_INDONESIA",
	"IN_TRANSIT", "ARRIVED_SAUDI", "IN_MADINAH", "IN_MAKKAH", "IN_ARAFAH",
	"IN_MUZDALIFAH", "IN_MINA", "BACK_IN_MAKKAH", "PRE_DEPARTURE_SAUDI",
	"DEPARTED_SAUDI", "IN_TRANSIT_RETURN", "ARRIVED_INDONESIA", "COMPLETED",
}

func JourneyStatusIndex(status string) int {
	for i, s := range JourneyStatuses {
		if s == status {
			return i
		}
	}
	return -1
}

type PilgrimJourneyStatus struct {
	PilgrimID     string
	Status        string
	UpdatedByName string
	UpdatedAt     time.Time
	Notes         string
}
