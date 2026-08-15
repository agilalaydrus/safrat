package domain

import "time"

type ChatMessage struct {
	ID          string
	OperatorID  string
	GroupID     string
	SenderName  string
	FromPilgrim bool
	Body        string
	CreatedAt   time.Time
}
