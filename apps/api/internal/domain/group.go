package domain

import "time"

type Group struct {
	ID           string
	SeasonID     string
	OperatorID   string
	Name         string
	Capacity     int32
	PilgrimCount int32
	LeaderID     string
	LeaderName   string
	KloterID     string
	CurrentCity  string
	Status       string
	LastUpdate   *time.Time
}

type OperatorMember struct {
	UserID string
	Name   string
	Email  string
}

// Muttawwif is one distinct leader aggregated across every group they lead
// for one operator — see GroupRepository.ListMuttawwif, which turns N
// (leader, group) rows from the query into this shape.
type Muttawwif struct {
	UserID    string
	Name      string
	Email     string
	Phone     string
	Groups    []LeaderGroup
	AgentID   string
	KYCStatus string
}
