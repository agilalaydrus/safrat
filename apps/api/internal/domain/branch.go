package domain

import "time"

type Branch struct {
	ID, OperatorID, Name, City                                string
	TargetPilgrims                                            int32
	TargetRevenueIDR                                          int64
	HeadUserID, Phone, BankName, AccountNumber, AccountHolder string
	IsActive                                                  bool
	CreatedAt, UpdatedAt                                      time.Time
}

type BranchTrendPoint struct {
	Month        string
	RevenueIDR   int64
	PilgrimCount int32
}

type BranchPerformance struct {
	BranchID              string
	Name                  string
	City                  string
	TargetPilgrims        int32
	TargetRevenueIDR      int64
	RevenueIDR            int64
	PilgrimCount          int32
	AgentCount            int32
	RevenueAchievementPct float64
	PilgrimAchievementPct float64
	Score                 float64
	CollectionPct         float64
	DocumentsReadyPct     float64
	Trend                 []BranchTrendPoint
}

type BranchPerformanceReport struct {
	Branches            []BranchPerformance
	NetworkRevenueIDR   int64
	NetworkPilgrimCount int32
	BelowTargetCount    int32
}
