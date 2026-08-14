package domain

import "time"

type Operator struct {
	ID              string
	BetterAuthOrgID string
	Name            string
	Country         string
	Email           string
	LicenseNumber   string
	CreatedAt       time.Time
}

type AuditLog struct {
	ID          string
	Action      string
	EntityID    string
	Description string
	CreatedAt   time.Time
}
