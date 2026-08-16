package domain

import "time"

type PilgrimAppInfo struct {
	ID                  string
	FullName            string
	PassportNumber      string
	GroupName           string
	HotelName           string
	RoomNumber          string
	RequiresWheelchair  bool
	SeasonID            string
	OperatorID          string
	KloterID            string
	KloterCode          string
	KloterEmbarkation   string
	KloterFlightNumber  string
	KloterDepartureDate *time.Time
	LinkedGoogleEmail   string
}
