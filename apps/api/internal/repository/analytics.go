package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

// AnalyticsRepository is the richer, breakdown-shaped counterpart to
// SeasonRepository.GetAnalytics (which stays as the single-row summary) —
// each method here is one chart/table on the Analytics tab.
type AnalyticsRepository struct{ queries *db.Queries }

func NewAnalyticsRepository(queries *db.Queries) *AnalyticsRepository {
	return &AnalyticsRepository{queries: queries}
}

func (r *AnalyticsRepository) ListPaymentTimeline(ctx context.Context, operatorID, seasonID string) ([]domain.PaymentMonthPoint, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	branchID, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.GetPaymentTimelineByMonth(ctx, db.GetPaymentTimelineByMonthParams{OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: branchID})
	if err != nil {
		return nil, err
	}
	result := make([]domain.PaymentMonthPoint, 0, len(rows))
	for _, row := range rows {
		month := ""
		if row.Month.Valid {
			month = row.Month.Time.Format("2006-01")
		}
		result = append(result, domain.PaymentMonthPoint{Month: month, PaidCount: row.Paid, DPCount: row.Dp, UnpaidCount: row.Unpaid})
	}
	return result, nil
}

func (r *AnalyticsRepository) ListAgentStats(ctx context.Context, operatorID, seasonID string) ([]domain.AgentSeasonStat, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	branchID, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.GetAgentSeasonStats(ctx, db.GetAgentSeasonStatsParams{OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: branchID})
	if err != nil {
		return nil, err
	}
	result := make([]domain.AgentSeasonStat, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.AgentSeasonStat{AgentName: row.AgentName, PilgrimCount: row.PilgrimCount, CommissionRate: row.CommissionRate})
	}
	return result, nil
}

func (r *AnalyticsRepository) ListKloterFill(ctx context.Context, operatorID, seasonID string) ([]domain.KloterFillStat, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	branchID, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListKloterFillForSeason(ctx, db.ListKloterFillForSeasonParams{OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: branchID})
	if err != nil {
		return nil, err
	}
	result := make([]domain.KloterFillStat, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.KloterFillStat{KloterCode: row.KloterCode, PilgrimCount: row.KPilgrimCount, Capacity: row.KCapacity})
	}
	return result, nil
}

func (r *AnalyticsRepository) ListHotelOccupancy(ctx context.Context, operatorID, seasonID string) ([]domain.HotelOccupancyStat, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	branchID, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListHotelOccupancyForSeason(ctx, db.ListHotelOccupancyForSeasonParams{OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: branchID})
	if err != nil {
		return nil, err
	}
	result := make([]domain.HotelOccupancyStat, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.HotelOccupancyStat{HotelName: row.HotelName, City: row.City, Capacity: row.Capacity, Allocated: row.Allocated})
	}
	return result, nil
}
