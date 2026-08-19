package domain

import "time"

type CertificateData struct {
	PilgrimName    string
	PassportNumber string
	Nationality    string
	SeasonName     string
	SeasonType     SeasonType
	StartDate      time.Time
	EndDate        time.Time
	OperatorName   string
	LicenseNumber  string
	GroupName      string
	LeaderName     string
	HotelsVisited  string
	MakkahHotels   string
	MadinahHotels  string
}
