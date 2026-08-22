package domain

import "time"

type SOSAlertMini struct {
	ID          string
	PilgrimName string
	GroupID     string // internal use only — matching to GroupMonitoringCard.HasActiveSos, not exposed on the wire
	GroupName   string
	Status      string
	CreatedAt   time.Time
}

type HealthReportMini struct {
	ID          string
	PilgrimName string
	GroupName   string
	Severity    string
	Symptoms    string
	CreatedAt   time.Time
}

// GroupRitualProgress lets the caller compute a completion % without a
// per-group round trip — TemplateCount 0 means no ritual templates exist
// yet for this season type, a valid state distinct from "0% complete".
type GroupRitualProgress struct {
	GroupID        string
	TemplateCount  int32
	PilgrimCount   int32
	CompletedCount int32
}

type ReturnTimelineItem struct {
	KloterID      string
	KloterCode    string
	ReturnAt      time.Time
	TotalPilgrims int32
	ReadyCount    int32
}
