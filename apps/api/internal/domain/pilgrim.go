package domain

import "time"

type Pilgrim struct {
	ID                    string
	SeasonID              string
	OperatorID            string
	GroupID               string
	FullName              string
	PassportNumber        string
	Nationality           string
	DateOfBirth           time.Time
	Gender                string
	PhotoURL              string
	Phone                 string
	EmergencyContact      string
	PreferredLang         string
	MedicalNotes          string
	RequiresWheelchair    bool
	MahramID              string
	IsSubstituted         bool
	SubstitutedByID       string
	AppAccessCode         string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	LastLat               *float64
	LastLng               *float64
	LastLocationAt        *time.Time
	KloterID              string
	Email                 string
	HasAccount            bool
	PaymentStatus         string
	PaymentReceiptURL     string
	PaymentNotes          string
	EmergencyContactName  string
	EmergencyContactPhone string
	PassportExpiryDate    *time.Time
	VaccineMeningitisDate *time.Time
	HotelCheckedIn        bool
	DocumentsPassport     bool
	DocumentsPhoto        bool
	DocumentsVaccine      bool
	// ACTIVE | CANCELLED — independent of IsSubstituted (a substituted
	// pilgrim was replaced by someone else; a cancelled one has no
	// replacement at all).
	Status string

	InsuranceProvider  string
	InsurancePolicyNo  string
	InsuranceClass     string
	BloodType          string
	ChronicConditions  string
	CurrentMedications string
}

type PilgrimDocument struct {
	ID             string
	PilgrimID      string
	DocType        string
	FileURL        string
	FileName       string
	UploadedBy     string
	CreatedAt      time.Time
	PilgrimName    string
	PassportNumber string
}

type Substitution struct {
	OriginalID             string
	OriginalName           string
	OriginalPassportNumber string
	NewID                  string
	NewName                string
	Reason                 string
	SubstitutedAt          time.Time
}

type PilgrimStats struct {
	Total              int32
	Substituted        int32
	RequiresWheelchair int32
	UnassignedGroup    int32
	UnassignedKloter   int32
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
	KloterID           string
	Email              string
}
