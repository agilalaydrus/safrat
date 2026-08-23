package domain

// Cascade event types — the contract between producers (services) and the
// worker relay. Keep the string values stable; they're persisted in
// cascade_events.event_type.
const (
	EventHealthReportCreated = "health.report_created"
)

// HealthReportCreatedPayload is the JSON payload for EventHealthReportCreated.
// The relay uses it to send the BERAT push without re-reading the DB.
type HealthReportCreatedPayload struct {
	Severity    string `json:"severity"`
	PilgrimName string `json:"pilgrim_name"`
}

// CascadeEvent is one row of the transactional outbox — a durable record that
// a cascade-worthy thing happened, drained by the worker relay.
type CascadeEvent struct {
	ID         int64
	OperatorID string
	EventType  string
	EntityID   string
	Payload    []byte // raw JSON
	Attempts   int32
}
