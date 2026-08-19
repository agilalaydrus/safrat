package domain

import "time"

type Broadcast struct {
	ID         string
	OperatorID string
	SeasonID   string
	Title      string
	Body       string
	CreatedAt  time.Time
}
