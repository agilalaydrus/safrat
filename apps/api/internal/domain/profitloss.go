package domain

import "time"

// PeriodFigure is one calendar month's P&L. CostIDR only reflects orders
// whose product has a known supplier_cost_idr — OrdersMissingCost and
// RevenueMissingCostIDR carry what that leaves out.
type PeriodFigure struct {
	PeriodStart           time.Time
	RevenueIDR            int64
	CostIDR               int64
	PlatformAmountIDR     int64
	AgentCommissionIDR    int64
	UnitCount             int32
	OrdersMissingCost     int32
	RevenueMissingCostIDR int64
}

// GrossProfitIDR is revenue minus known cost.
func (p PeriodFigure) GrossProfitIDR() int64 { return p.RevenueIDR - p.CostIDR }

// NetProfitIDR is what the operator actually keeps: revenue minus platform
// fee minus agent commission minus known cost. Equivalent to
// operator_amount_idr - CostIDR, since total_price_idr always splits into
// exactly those three shares.
func (p PeriodFigure) NetProfitIDR() int64 {
	return p.RevenueIDR - p.PlatformAmountIDR - p.AgentCommissionIDR - p.CostIDR
}

func (p PeriodFigure) GrossMarginPct() float64 {
	if p.RevenueIDR == 0 {
		return 0
	}
	return float64(p.GrossProfitIDR()) / float64(p.RevenueIDR) * 100
}

func (p PeriodFigure) NetProfitPerUnitIDR() int64 {
	if p.UnitCount == 0 {
		return 0
	}
	return p.NetProfitIDR() / int64(p.UnitCount)
}

type BranchFigure struct {
	BranchID          string
	BranchName        string
	RevenueIDR        int64
	OperatorAmountIDR int64
	CostIDR           int64
	TargetRevenueIDR  int64
}

func (b BranchFigure) NetProfitIDR() int64 { return b.OperatorAmountIDR - b.CostIDR }

func (b BranchFigure) TargetAchievedPct() float64 {
	if b.TargetRevenueIDR == 0 {
		return 0
	}
	return float64(b.RevenueIDR) / float64(b.TargetRevenueIDR) * 100
}

type AgentFigure struct {
	AgentID       string
	AgentName     string
	RevenueIDR    int64
	CommissionIDR int64
	OrderCount    int32
}

// ExportRow is one order in the streamed P&L export.
type ExportRow struct {
	OrderID            string
	PaidAt             time.Time
	PilgrimName        string
	ProductName        string
	BranchName         string
	AgentName          string
	Quantity           int32
	RevenueIDR         int64
	CostKnown          bool
	CostIDR            int64
	PlatformAmountIDR  int64
	AgentCommissionIDR int64
}

func (r ExportRow) NetProfitIDR() int64 {
	if !r.CostKnown {
		return 0
	}
	return r.RevenueIDR - r.PlatformAmountIDR - r.AgentCommissionIDR - r.CostIDR
}
