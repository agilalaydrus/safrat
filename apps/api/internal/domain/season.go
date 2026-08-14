package domain

import "time"

type SeasonType string

const (
	SeasonTypeHajj  SeasonType = "HAJJ"
	SeasonTypeUmrah SeasonType = "UMRAH"
)

type Season struct {
	ID         string
	OperatorID string
	Name       string
	Type       SeasonType
	StartDate  time.Time
	EndDate    time.Time
	IsActive   bool
	CreatedAt  time.Time
}
