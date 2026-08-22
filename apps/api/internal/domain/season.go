package domain

import "time"

type SeasonType string

const (
	SeasonTypeHajj            SeasonType = "HAJJ"
	SeasonTypeUmrahReguler    SeasonType = "UMRAH_REGULER"
	SeasonTypeUmrahRajab      SeasonType = "UMRAH_RAJAB"
	SeasonTypeUmrahRamadhan   SeasonType = "UMRAH_RAMADHAN"
	SeasonTypeUmrahSyawal     SeasonType = "UMRAH_SYAWAL"
	SeasonTypeUmrahDzulqaidah SeasonType = "UMRAH_DZULQAIDAH"
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
	Capacity   int32
	Slug       string
}

type SeasonAnalytics struct {
	TotalPilgrims         int64
	PaidCount             int64
	DPCount               int64
	UnpaidCount           int64
	DocsComplete          int64
	CheckedInCount        int64
	RoomsAllocated        int64
	SeatsAssigned         int64
	WheelchairCount       int64
	UnassignedGroupCount  int64
	UnassignedKloterCount int64
	OrderCount            int32
	TotalRevenueIDR       int64
}

type PaymentMonthPoint struct {
	Month       string // "2026-03"
	PaidCount   int64
	DPCount     int64
	UnpaidCount int64
}

type AgentSeasonStat struct {
	AgentName      string
	PilgrimCount   int64
	CommissionRate float64
}

type KloterFillStat struct {
	KloterCode   string
	PilgrimCount int32
	Capacity     int32
}

type HotelOccupancyStat struct {
	HotelName string
	City      string
	Capacity  int32
	Allocated int32
}
