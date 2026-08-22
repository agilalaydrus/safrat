package domain

import "time"

type Kloter struct {
	ID            string
	SeasonID      string
	OperatorID    string
	Code          string
	Embarkation   string
	FlightNumber  string
	DepartureDate *time.Time
	Capacity      int32
	PilgrimCount  int32
	Status        string
	Notes         string
}
