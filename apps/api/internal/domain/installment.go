package domain

import "time"

// InstallmentPlan is the frozen commercial agreement for one pilgrim. Amounts
// and schedule never mutate; current paid/outstanding/status are projections
// from append-only payment entries.
type InstallmentPlan struct {
	ID                   string
	OperatorID           string
	SeasonID             string
	PilgrimID            string
	PilgrimName          string
	BranchID             string
	Scheme               string
	GrossAmountIDR       int64
	CashBonusIDR         int64
	PayableAmountIDR     int64
	PaidAmountIDR        int64
	OutstandingAmountIDR int64
	Status               string
	CreatedAt            time.Time
}

type Installment struct {
	ID                   string
	PlanID               string
	Number               int32
	Label                string
	DueDate              time.Time
	AmountDueIDR         int64
	PaidAmountIDR        int64
	OutstandingAmountIDR int64
	Status               string
	DaysOverdue          int32
}

type InstallmentPayment struct {
	ID                string
	PlanID            string
	InstallmentID     string
	Kind              string
	AmountIDR         int64
	Method            string
	Reference         string
	Note              string
	OriginalPaymentID string
	VerifiedByUserID  string
	ReceiptNumber     string
	CreatedAt         time.Time
}

type InstallmentPlanDetail struct {
	Plan         InstallmentPlan
	Installments []Installment
	Payments     []InstallmentPayment
}

type InstallmentScheduleDraft struct {
	Number       int32
	Label        string
	DueDate      time.Time
	AmountDueIDR int64
}

type InstallmentPlanDraft struct {
	PilgrimID      string
	Scheme         string
	GrossAmountIDR int64
	CashBonusIDR   int64
	FirstDueDate   time.Time
	IdempotencyKey string
}

type InstallmentReceivableFilter struct {
	SeasonID string
	Status   string
	Search   string
	Limit    int32
	Offset   int32
}

type InstallmentReceivableResult struct {
	Plans                  []InstallmentPlan
	TotalCount             int64
	TotalReceivableIDR     int64
	TotalOverdueIDR        int64
	DueNext7DaysIDR        int64
	UnverifiedPaymentCount int64
	CollectionRateBPS      int32
}
