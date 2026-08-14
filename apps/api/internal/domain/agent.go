package domain

import "time"

type Agent struct {
	ID             string
	OperatorID     string
	Name           string
	Phone          string
	Email          string
	CommissionRate float64
	Notes          string
	IsActive       bool
	PilgrimCount   int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
