package domain

import "time"

type WaitlistEntry struct {
	ID         string
	OperatorID string
	SeasonID   string
	FullName   string
	Email      string
	Phone      string
	ProductID  string
	Position   int32
	Status     string
	PromotedAt *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
}
