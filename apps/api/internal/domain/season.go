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
