package domain

import "time"

type Agent struct {
	ID                string
	OperatorID        string
	Name              string
	Phone             string
	Email             string
	CommissionRate    float64
	Notes             string
	IsActive          bool
	PilgrimCount      int32
	ReferralCode      string
	Tier              string
	ReferredByAgentID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AgentPayout struct {
	AgentID            string
	AgentName          string
	TotalCommissionIDR int64
	PaidOrderCount     int32
	TotalDisbursedIDR  int64
	OutstandingIDR     int64
}
