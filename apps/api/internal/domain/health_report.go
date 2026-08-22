package domain

import "time"

type HealthReport struct {
	ID          string
	PilgrimID   string
	PilgrimName string
	GroupID     string
	GroupName   string
	Severity    string
	Symptoms    string
	ActionTaken string
	Resolved    bool
	ResolvedAt  *time.Time
	CreatedAt   time.Time
}
