package domain

import "time"

type SOSAlert struct {
	ID             string
	OperatorID     string
	PilgrimID      string
	PilgrimName    string
	Status         string
	AcknowledgedBy string
	AcknowledgedAt *time.Time
	ResolvedBy     string
	ResolvedAt     *time.Time
	Notes          string
	CreatedAt      time.Time
	Lat            *float64
	Lng            *float64
}
