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
}

type SeasonAnalytics struct {
	TotalPilgrims  int64
	PaidCount      int64
	DPCount        int64
	UnpaidCount    int64
	DocsComplete   int64
	CheckedInCount int64
	RoomsAllocated int64
	SeatsAssigned  int64
}
