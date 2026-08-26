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

// CommissionEntry is one movement in an agent's commission ledger. Kind is
// EARNED, REVERSED or ADJUSTMENT; a reversal carries a negative amount.
type CommissionEntry struct {
	ID          string
	AmountIDR   int64
	Kind        string
	Note        string
	ProductName string
	CreatedAt   time.Time
}

// ReferredCustomerRecap is what one referred jamaah transacted, net of
// refunds, together with the commission it produced.
type ReferredCustomerRecap struct {
	PilgrimID          string
	PilgrimName        string
	OrderCount         int32
	RefundedOrderCount int32
	TotalPaidIDR       int64
	RefundedIDR        int64
	CommissionIDR      int64
	LastTransactionAt  time.Time
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
	SeasonID       string
	SeasonName     string
	DepartureDate  *time.Time
}
