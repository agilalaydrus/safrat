package domain

import "time"

type LeaderGroup struct {
	ID              string
	SeasonID        string
	Name            string
	Capacity        int32
	PilgrimCount    int32
	CurrentCity     string
	LastUpdate      *time.Time
	CurrentActivity string
}

type CheckIn struct {
	ID         string
	OperatorID string
	MovementID string
	PilgrimID  string
	Type       string
	CreatedAt  time.Time
}
