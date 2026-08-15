package domain

import "time"

type Pilgrim struct {
	ID                 string
	SeasonID           string
	OperatorID         string
	GroupID            string
	FullName           string
	PassportNumber     string
	Nationality        string
	DateOfBirth        time.Time
	Gender             string
	PhotoURL           string
	Phone              string
	EmergencyContact   string
	PreferredLang      string
	MedicalNotes       string
	RequiresWheelchair bool
	MahramID           string
	IsSubstituted      bool
	SubstitutedByID    string
	AppAccessCode      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	LastLat            *float64
	LastLng            *float64
	LastLocationAt     *time.Time
}

type PilgrimInput struct {
	SeasonID           string
	GroupID            string
	FullName           string
	PassportNumber     string
	Nationality        string
	DateOfBirth        time.Time
	Gender             string
	PhotoURL           string
	Phone              string
	EmergencyContact   string
	PreferredLang      string
	MedicalNotes       string
	RequiresWheelchair bool
	MahramID           string
}
