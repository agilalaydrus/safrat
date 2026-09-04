package service

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultProfitLossMonths = 5

type ProfitLossService struct {
	operatorRepository   *repository.OperatorRepository
	profitLossRepository *repository.ProfitLossRepository
}

func NewProfitLossService(operators *repository.OperatorRepository, profitLoss *repository.ProfitLossRepository) *ProfitLossService {
	return &ProfitLossService{operatorRepository: operators, profitLossRepository: profitLoss}
}

// windowStart is the first instant of the month N-1 months before the
// current one, so a 5-month window always includes the current month plus
// the four before it — never a partial 6th month from rounding.
func windowStart(months int32) time.Time {
	if months <= 0 {
		months = defaultProfitLossMonths
	}
	now := time.Now().UTC()
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return firstOfThisMonth.AddDate(0, -int(months-1), 0)
}

var monthNames = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

func periodLabel(t time.Time) string {
	return fmt.Sprintf("%s %d", monthNames[t.Month()-1], t.Year())
}

func periodFigureMessage(p domain.PeriodFigure) *hajjv1.PeriodFigure {
	return &hajjv1.PeriodFigure{
		PeriodLabel: periodLabel(p.PeriodStart), PeriodStart: timestamppb.New(p.PeriodStart),
		RevenueIdr: p.RevenueIDR, CostIdr: p.CostIDR, GrossProfitIdr: p.GrossProfitIDR(),
		GrossMarginPct: p.GrossMarginPct(), NetProfitIdr: p.NetProfitIDR(), UnitCount: p.UnitCount,
		NetProfitPerUnitIdr: p.NetProfitPerUnitIDR(), OrdersMissingCost: p.OrdersMissingCost,
		RevenueMissingCostIdr: p.RevenueMissingCostIDR,
	}
}

func (s *ProfitLossService) GetProfitLossReport(ctx context.Context, orgID string, req *hajjv1.GetProfitLossReportRequest) (*hajjv1.GetProfitLossReportResponse, error) {
	if req == nil {
		return nil, serviceError("ProfitLossService.GetProfitLossReport", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProfitLossService.GetProfitLossReport", err)
	}
	since := windowStart(req.Months)

	periods, err := s.profitLossRepository.ByPeriod(ctx, op.ID, since)
	if err != nil {
		return nil, serviceError("ProfitLossService.GetProfitLossReport", err)
	}
	branches, err := s.profitLossRepository.ByBranch(ctx, op.ID, since)
	if err != nil {
		return nil, serviceError("ProfitLossService.GetProfitLossReport", err)
	}
	agents, err := s.profitLossRepository.ByAgent(ctx, op.ID, since)
	if err != nil {
		return nil, serviceError("ProfitLossService.GetProfitLossReport", err)
	}

	response := &hajjv1.GetProfitLossReportResponse{}
	var total domain.PeriodFigure
	for _, p := range periods {
		response.Periods = append(response.Periods, periodFigureMessage(p))
		total.RevenueIDR += p.RevenueIDR
		total.CostIDR += p.CostIDR
		total.PlatformAmountIDR += p.PlatformAmountIDR
		total.AgentCommissionIDR += p.AgentCommissionIDR
		total.UnitCount += p.UnitCount
		total.OrdersMissingCost += p.OrdersMissingCost
		total.RevenueMissingCostIDR += p.RevenueMissingCostIDR
	}
	total.PeriodStart = since
	response.WindowTotal = periodFigureMessage(total)
	// The window total's own label is the range, not one month — overwrite
	// what periodFigureMessage guessed from PeriodStart alone.
	response.WindowTotal.PeriodLabel = fmt.Sprintf("%d bulan terakhir", len(periods))
	if len(periods) == 0 {
		response.WindowTotal.PeriodLabel = "Tidak ada transaksi pada rentang ini"
	}

	var totalNetProfit int64
	for _, b := range branches {
		totalNetProfit += b.NetProfitIDR()
	}
	for _, b := range branches {
		contribution := 0.0
		if totalNetProfit != 0 {
			contribution = float64(b.NetProfitIDR()) / float64(totalNetProfit) * 100
		}
		response.Branches = append(response.Branches, &hajjv1.BranchFigure{
			BranchId: b.BranchID, BranchName: b.BranchName, RevenueIdr: b.RevenueIDR, NetProfitIdr: b.NetProfitIDR(),
			TargetRevenueIdr: b.TargetRevenueIDR, TargetAchievedPct: b.TargetAchievedPct(), NetProfitContributionPct: contribution,
		})
	}
	for _, a := range agents {
		response.Agents = append(response.Agents, &hajjv1.AgentFigure{
			AgentId: a.AgentID, AgentName: a.AgentName, RevenueIdr: a.RevenueIDR, CommissionIdr: a.CommissionIDR, OrderCount: a.OrderCount,
		})
	}
	return response, nil
}

func (s *ProfitLossService) StreamProfitLossExport(ctx context.Context, orgID string, req *hajjv1.StreamProfitLossExportRequest, stream *connect.ServerStream[hajjv1.ProfitLossExportRow]) error {
	if req == nil {
		return serviceError("ProfitLossService.StreamProfitLossExport", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("ProfitLossService.StreamProfitLossExport", err)
	}
	since := windowStart(req.Months)
	err = s.profitLossRepository.StreamExport(ctx, op.ID, since, func(row domain.ExportRow) error {
		return stream.Send(&hajjv1.ProfitLossExportRow{
			OrderId: row.OrderID, PaidAt: timestamppb.New(row.PaidAt), PilgrimName: row.PilgrimName,
			ProductName: row.ProductName, BranchName: row.BranchName, AgentName: row.AgentName,
			Quantity: row.Quantity, RevenueIdr: row.RevenueIDR, CostKnown: row.CostKnown, CostIdr: row.CostIDR,
			PlatformAmountIdr: row.PlatformAmountIDR, AgentCommissionIdr: row.AgentCommissionIDR, NetProfitIdr: row.NetProfitIDR(),
		})
	})
	if err != nil {
		return serviceError("ProfitLossService.StreamProfitLossExport", err)
	}
	return nil
}
