package repository

import (
	"context"
	"sort"

	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
)

// BranchRepository owns reporting boundaries as well as branch data. A
// branch head's identity is resolved here, so every caller gets the same
// least-privilege result without relying on handler checks.
type BranchRepository struct{ queries *db.Queries }

func NewBranchRepository(queries *db.Queries) *BranchRepository {
	return &BranchRepository{queries: queries}
}

func (r *BranchRepository) List(ctx context.Context, operatorID string, includeInactive bool) ([]domain.Branch, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListBranches(ctx, db.ListBranchesParams{OperatorID: opUUID, Column2: includeInactive, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	items := make([]domain.Branch, 0, len(rows))
	for _, row := range rows {
		items = append(items, branchFromRow(row))
	}
	return items, nil
}

func branchFromRow(row db.ListBranchesRow) domain.Branch {
	return domain.Branch{ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), Name: row.Name, City: row.City, TargetPilgrims: row.TargetPilgrims, TargetRevenueIDR: row.TargetRevenueIdr, HeadUserID: row.HeadUserID, Phone: row.Phone, BankName: row.BankName, AccountNumber: row.AccountNumber, AccountHolder: row.AccountHolder, IsActive: row.IsActive, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}

func (r *BranchRepository) GetPerformance(ctx context.Context, operatorID, seasonID string) (*domain.BranchPerformanceReport, error) {
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
	rows, err := r.queries.ListBranchPerformance(ctx, db.ListBranchPerformanceParams{
		OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: branchID,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	trendRows, err := r.queries.ListBranchPerformanceTrends(ctx, db.ListBranchPerformanceTrendsParams{
		OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: branchID,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	trends := make(map[string][]domain.BranchTrendPoint, len(rows))
	for _, row := range trendRows {
		month := ""
		if row.Month.Valid {
			month = row.Month.Time.Format("2006-01")
		}
		trends[uuidString(row.BranchID)] = append(trends[uuidString(row.BranchID)], domain.BranchTrendPoint{
			Month: month, RevenueIDR: row.RevenueIdr, PilgrimCount: row.PilgrimCount,
		})
	}
	report := &domain.BranchPerformanceReport{Branches: make([]domain.BranchPerformance, 0, len(rows))}
	for _, row := range rows {
		branch := domain.BranchPerformance{
			BranchID: uuidString(row.BranchID), Name: row.BranchName, City: row.BranchCity,
			TargetPilgrims: row.TargetPilgrims, TargetRevenueIDR: row.TargetRevenueIdr,
			RevenueIDR: row.RevenueIdr, PilgrimCount: row.PilgrimCount, AgentCount: row.AgentCount,
			Trend: trends[uuidString(row.BranchID)],
		}
		branch.RevenueAchievementPct = percentage(branch.RevenueIDR, branch.TargetRevenueIDR)
		branch.PilgrimAchievementPct = percentage(int64(branch.PilgrimCount), int64(branch.TargetPilgrims))
		branch.CollectionPct = percentage(int64(row.PaidCount), int64(branch.PilgrimCount))
		branch.DocumentsReadyPct = percentage(int64(row.DocumentsReadyCount), int64(branch.PilgrimCount))
		branch.Score = 0.7*branch.RevenueAchievementPct + 0.3*branch.PilgrimAchievementPct
		report.NetworkRevenueIDR += branch.RevenueIDR
		report.NetworkPilgrimCount += branch.PilgrimCount
		if branch.TargetRevenueIDR > 0 && branch.RevenueAchievementPct < 100 || branch.TargetPilgrims > 0 && branch.PilgrimAchievementPct < 100 {
			report.BelowTargetCount++
		}
		report.Branches = append(report.Branches, branch)
	}
	sort.SliceStable(report.Branches, func(i, j int) bool {
		if report.Branches[i].Score == report.Branches[j].Score {
			return report.Branches[i].Name < report.Branches[j].Name
		}
		return report.Branches[i].Score > report.Branches[j].Score
	})
	return report, nil
}

func percentage(numerator, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}
