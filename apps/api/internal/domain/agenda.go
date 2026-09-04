package domain

import "time"

// AgendaEvent is an internal event on the combined calendar — a head-office
// meeting, an internal briefing, anything that is not a manasik session or a
// kloter movement (those are read live from their own tables; see
// AgendaService.ListAgenda).
type AgendaEvent struct {
	ID         string
	OperatorID string
	BranchID   string // "" = head office ("pusat")
	SeasonID   string // "" = shows on every season's agenda
	Title      string
	Location   string
	StartsAt   time.Time
	EndsAt     time.Time // zero if open-ended
	Notes      string
	BranchName string
	CreatedAt  time.Time
}

// AgendaItem is one row on the merged timeline, whichever of the three
// sources it came from.
type AgendaItem struct {
	ID         string
	Kind       string // INTERNAL | MANASIK | DEPARTURE | RETURN
	Title      string
	Location   string
	StartsAt   time.Time
	EndsAt     time.Time
	KloterID   string
	KloterCode string
	BranchID   string
	BranchName string
	Notes      string
}
