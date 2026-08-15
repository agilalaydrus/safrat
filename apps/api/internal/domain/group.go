package domain

type Group struct {
	ID           string
	SeasonID     string
	OperatorID   string
	Name         string
	Capacity     int32
	PilgrimCount int32
	LeaderID     string
	LeaderName   string
}

type OperatorMember struct {
	UserID string
	Name   string
	Email  string
}
