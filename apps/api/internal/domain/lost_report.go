package domain

import "time"

type LostReport struct {
	ID                string
	PilgrimID         string
	PilgrimName       string
	PilgrimPhone      string
	OperatorID        string
	GroupID           string
	GroupName         string
	Latitude          float64
	Longitude         float64
	LastKnownLocation string
	Status            string
	CreatedAt         time.Time
	ResolvedAt        *time.Time
}
