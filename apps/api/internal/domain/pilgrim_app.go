package domain

import "time"

type PilgrimAppInfo struct {
	ID                 string
	FullName           string
	PassportNumber     string
	GroupName          string
	HotelName          string
	RoomNumber         string
	RequiresWheelchair bool
	SeasonID           string
	// AgentID is the referring agent, carried here so a jamaah's own checkout
	// credits the referral the same way every other lane does.
	AgentID             string
	OperatorID          string
	KloterID            string
	KloterCode          string
	KloterEmbarkation   string
	KloterFlightNumber  string
	KloterDepartureDate *time.Time
	LinkedGoogleEmail   string
	Phone               string
	NIK                 string
	Address             string
	KYCStatus           string
	KYCRejectionReason  string
	PlaceOfBirth        string
	MaritalStatus       string
	Occupation          string
	FatherName          string
	Status              string
	PaymentStatus       string
	HotelStays          []HotelStay
}

// HotelStay is one room allocation, for pilgrims with more than one hotel
// (e.g. Makkah + Madinah) — HotelName/RoomNumber above are just the most
// recent one, shown as a display summary.
type HotelStay struct {
	HotelName  string
	RoomNumber string
	RoomType   string
}
