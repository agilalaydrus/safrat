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
	// AgentID is the agent who referred this jamaah — the referral that earns
	// commission on everything they buy, whoever places the order.
	AgentID               string
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

	NIK                string
	Address            string
	KYCStatus          string
	KYCSource          string
	KYCVerifiedBy      string
	KYCVerifiedAt      *time.Time
	KYCRejectionReason string
	DocumentsKTP       bool
	DocumentsSelfie    bool

	PlaceOfBirth  string
	MaritalStatus string
	Occupation    string
	FatherName    string

	DocumentsKK          bool
	DocumentsMahramProof bool

	InsuranceStartDate           *time.Time
	InsuranceEndDate             *time.Time
	InsuranceBeneficiaryName     string
	InsuranceBeneficiaryRelation string

	DocumentsVisa  bool
	VisaNumber     string
	VisaExpiryDate *time.Time
}

type PilgrimDocumentChecklistInput struct {
	Passport       bool
	Photo          bool
	Vaccine        bool
	KTP            bool
	KK             bool
	MahramProof    bool
	Visa           bool
	PassportExpiry *time.Time
	VaccineDate    *time.Time
	VisaNumber     string
	VisaExpiry     *time.Time
}

type PilgrimKYCInput struct {
	NIK           string
	Address       string
	PlaceOfBirth  string
	MaritalStatus string
	Occupation    string
	FatherName    string
}

type PilgrimInsuranceInput struct {
	Provider            string
	PolicyNo            string
	Class               string
	BloodType           string
	ChronicConditions   string
	CurrentMedications  string
	StartDate           *time.Time
	EndDate             *time.Time
	BeneficiaryName     string
	BeneficiaryRelation string
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
	AgentID            string
}
