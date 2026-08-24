package domain

// Cascade event types — the contract between producers (services) and the
// worker relay. Keep the string values stable; they're persisted in
// cascade_events.event_type.
const (
	EventHealthReportCreated = "health.report_created"
	EventGroupCityUpdated    = "group.city_updated"
	EventKloterStatusUpdated = "kloter.status_updated"
	EventRitualBulkCompleted = "ritual.bulk_completed"
)

// HealthReportCreatedPayload is the JSON payload for EventHealthReportCreated.
// The relay uses it to send the BERAT push without re-reading the DB.
type HealthReportCreatedPayload struct {
	Severity    string `json:"severity"`
	PilgrimName string `json:"pilgrim_name"`
}

type GroupCityUpdatedPayload struct {
	GroupID          string `json:"group_id"`
	City             string `json:"city"`
	JourneyStatus    string `json:"journey_status,omitempty"`
	Notes            string `json:"notes,omitempty"`
	UpdatedBy        string `json:"updated_by,omitempty"`
	NotificationBody string `json:"notification_body"`
}

type KloterStatusUpdatedPayload struct {
	KloterID         string `json:"kloter_id"`
	KloterCode       string `json:"kloter_code"`
	Status           string `json:"status"`
	JourneyStatus    string `json:"journey_status,omitempty"`
	UpdatedBy        string `json:"updated_by,omitempty"`
	NotificationBody string `json:"notification_body,omitempty"`
}

type RitualBulkCompletedPayload struct {
	GroupID          string `json:"group_id"`
	RitualID         string `json:"ritual_id"`
	CompletedCount   int32  `json:"completed_count"`
	NotificationBody string `json:"notification_body"`
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
