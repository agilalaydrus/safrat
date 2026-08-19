package domain

import "time"

type InsuranceClaim struct {
	ID                string
	PilgrimID         string
	PilgrimName       string
	PassportNumber    string
	InsuranceProvider string
	InsurancePolicyNo string
	OperatorID        string
	ClaimType         string
	IncidentDate      time.Time
	Description       string
	Status            string
	ClaimAmountIDR    int64
	SettledAmountIDR  int64
	FiledBy           string
	CreatedAt         time.Time
}

type InsuranceClaimExportData struct {
	FullName              string
	PassportNumber        string
	DateOfBirth           time.Time
	Gender                string
	Nationality           string
	Phone                 string
	EmergencyContactName  string
	EmergencyContactPhone string
	BloodType             string
	ChronicConditions     string
	CurrentMedications    string
	InsuranceProvider     string
	InsurancePolicyNo     string
	InsuranceClass        string
	MedicalNotes          string
	SeasonName            string
	SeasonStartDate       time.Time
	SeasonEndDate         time.Time
	OperatorName          string
	OperatorLicenseNumber string
	OperatorPhone         string
	Claim                 InsuranceClaim
}
