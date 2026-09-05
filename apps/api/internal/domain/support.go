package domain

import "time"

type SupportTicket struct {
	ID          string
	OperatorID  string
	// OperatorName is only ever populated on the platform-side listing (C5,
	// TUGAS-PANEL-SAAS.md) — an operator reading their own tickets already
	// knows who they are.
	OperatorName string
	Subject      string
	Priority    string // LOW | MEDIUM | HIGH | URGENT
	Status      string // OPEN | IN_PROGRESS | RESOLVED | CLOSED
	CreatedByID string
	CreatedAt   time.Time
	ResolvedAt  *time.Time
}

// responseTargets is how long the platform has to make the first move on a
// ticket of each priority, read off the design's "prioritas dan target
// waktu respons" — fixed rather than configurable, so the number on screen
// always means the same thing regardless of who is looking at it.
var responseTargets = map[string]time.Duration{
	"URGENT": 1 * time.Hour,
	"HIGH":   4 * time.Hour,
	"MEDIUM": 24 * time.Hour,
	"LOW":    72 * time.Hour,
}

// ResponseDueAt is computed, never stored — a stored deadline would drift
// the moment the priority-to-target mapping ever changed, silently
// mismatching every ticket already on file.
func (t SupportTicket) ResponseDueAt() time.Time {
	target, ok := responseTargets[t.Priority]
	if !ok {
		target = responseTargets["MEDIUM"]
	}
	return t.CreatedAt.Add(target)
}

// ResponseOverdue is false once a ticket has moved past OPEN — a ticket
// already being worked on isn't waiting on a first response anymore, and one
// already resolved or closed answered on time by definition of being done.
func (t SupportTicket) ResponseOverdue() bool {
	if t.Status != "OPEN" {
		return false
	}
	return time.Now().After(t.ResponseDueAt())
}

type SupportTicketMessage struct {
	ID               string
	TicketID         string
	Body             string
	AuthorUserID     string
	AuthorName       string
	AuthorIsPlatform bool
	CreatedAt        time.Time
}
