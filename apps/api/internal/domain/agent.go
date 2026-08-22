package domain

import "time"

type Agent struct {
	ID                string
	OperatorID        string
	Name              string
	Phone             string
	Email             string
	CommissionRate    float64
	Notes             string
	IsActive          bool
	PilgrimCount      int32
	ReferralCode      string
	Tier              string
	ReferredByAgentID string
	CreatedAt         time.Time
	UpdatedAt         time.Time

	NIK                string
	NPWP               string
	Address            string
	DateOfBirth        *time.Time
	PassportNumber     string
	PassportExpiryDate *time.Time
	BankName           string
	BankAccountNumber  string
	BankAccountHolder  string
	KYCStatus          string
	KYCSource          string
	KYCVerifiedBy      string
	KYCVerifiedAt      *time.Time
	KYCRejectionReason string
}

type AgentKYCInput struct {
	NIK                string
	NPWP               string
	Address            string
	DateOfBirth        *time.Time
	PassportNumber     string
	PassportExpiryDate *time.Time
	BankName           string
	BankAccountNumber  string
	BankAccountHolder  string
}

type AgentDocument struct {
	ID         string
	AgentID    string
	DocType    string
	FileURL    string
	FileName   string
	UploadedBy string
	CreatedAt  time.Time
}

type AgentPayout struct {
	AgentID            string
	AgentName          string
	TotalCommissionIDR int64
	PaidOrderCount     int32
	TotalDisbursedIDR  int64
	OutstandingIDR     int64
}

type AgentPayoutEntry struct {
	ID         string
	AmountIDR  int64
	Note       string
	Method     string
	PaidByName string
	CreatedAt  time.Time
}

type OrderCredit struct {
	OrderID     string
	AmountIDR   int64
	ProductName string
	PaidAt      time.Time
}

type PayoutRequest struct {
	ID             string
	AgentID        string
	AgentName      string
	AmountIDR      int64
	Note           string
	Status         string
	ResolutionNote string
	RequestedAt    time.Time
	ResolvedAt     *time.Time
}

type AgentPilgrim struct {
	ID             string
	FullName       string
	PassportNumber string
	Gender         string
	PaymentStatus  string
	DocsComplete   bool
	PilgrimStatus  string
	SeasonName     string
	DepartureDate  *time.Time
}
