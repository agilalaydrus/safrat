package repository

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProfitLossRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewProfitLossRepository(pool *pgxpool.Pool) *ProfitLossRepository {
	return &ProfitLossRepository{pool: pool, queries: db.New(pool)}
}

func (r *ProfitLossRepository) ByPeriod(ctx context.Context, operatorID string, since time.Time) ([]domain.PeriodFigure, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ProfitLossByPeriod(ctx, db.ProfitLossByPeriodParams{OperatorID: op, PaidAt: pgtype.Timestamptz{Time: since, Valid: true}})
	if err != nil {
		return nil, databaseError(err)
	}
	out := make([]domain.PeriodFigure, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PeriodFigure{
			PeriodStart: row.PeriodStart.Time, RevenueIDR: row.RevenueIdr, CostIDR: row.CostIdr,
			PlatformAmountIDR: row.PlatformAmountIdr, AgentCommissionIDR: row.AgentCommissionIdr,
			UnitCount: row.UnitCount, OrdersMissingCost: row.OrdersMissingCost, RevenueMissingCostIDR: row.RevenueMissingCostIdr,
		})
	}
	return out, nil
}

func (r *ProfitLossRepository) ByBranch(ctx context.Context, operatorID string, since time.Time) ([]domain.BranchFigure, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ProfitLossByBranch(ctx, db.ProfitLossByBranchParams{OperatorID: op, PaidAt: pgtype.Timestamptz{Time: since, Valid: true}})
	if err != nil {
		return nil, databaseError(err)
	}
	out := make([]domain.BranchFigure, 0, len(rows))
	for _, row := range rows {
		name := row.BranchName
		if name == "" {
			name = "Kantor Pusat"
		}
		out = append(out, domain.BranchFigure{
			BranchID: nullableUUIDString(row.BranchID), BranchName: name, RevenueIDR: row.RevenueIdr,
			OperatorAmountIDR: row.OperatorAmountIdr, CostIDR: row.CostIdr, TargetRevenueIDR: row.TargetRevenueIdr,
		})
	}
	return out, nil
}

func (r *ProfitLossRepository) ByAgent(ctx context.Context, operatorID string, since time.Time) ([]domain.AgentFigure, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ProfitLossByAgent(ctx, db.ProfitLossByAgentParams{OperatorID: op, PaidAt: pgtype.Timestamptz{Time: since, Valid: true}})
	if err != nil {
		return nil, databaseError(err)
	}
	out := make([]domain.AgentFigure, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.AgentFigure{
			AgentID: uuidString(row.AgentID), AgentName: row.AgentName, RevenueIDR: row.RevenueIdr,
			CommissionIDR: row.CommissionIdr, OrderCount: row.OrderCount,
		})
	}
	return out, nil
}

const exportRowsQuery = `
SELECT
  o.id, o.paid_at, p.full_name, pr.name,
  COALESCE(b.name, 'Kantor Pusat'), COALESCE(a.name, ''),
  o.quantity, o.total_price_idr, pr.supplier_cost_idr, o.platform_amount_idr, o.agent_commission_idr
FROM orders o
JOIN pilgrims p ON p.id = o.pilgrim_id
JOIN products pr ON pr.id = o.product_id
LEFT JOIN branches b ON b.id = o.branch_id
LEFT JOIN agents a ON a.id = o.agent_id
WHERE o.operator_id = $1 AND o.status = 'PAID' AND o.paid_at >= $2
ORDER BY o.paid_at`

// StreamExport calls emit once per order, in paid_at order, without ever
// materializing the full result set as a slice — rows.Next() pulls one row
// at a time straight off the wire. This is the difference that matters: a
// sqlc :many query buffers every row into memory before returning any of
// them, which is exactly the shape of bug that took a competitor's PDF
// export down at 1,240 pilgrims (see the task file).
func (r *ProfitLossRepository) StreamExport(ctx context.Context, operatorID string, since time.Time, emit func(domain.ExportRow) error) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, exportRowsQuery, op, since)
	if err != nil {
		return databaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id, pilgrimName, productName, branchName, agentName  string
			paidAt                                               pgtype.Timestamptz
			quantity                                             int32
			totalPriceIDR, platformAmountIDR, agentCommissionIDR int64
			supplierCostIDR                                      *int64
			orderID                                              pgtype.UUID
		)
		if err := rows.Scan(&orderID, &paidAt, &pilgrimName, &productName, &branchName, &agentName,
			&quantity, &totalPriceIDR, &supplierCostIDR, &platformAmountIDR, &agentCommissionIDR); err != nil {
			return databaseError(err)
		}
		id = uuidString(orderID)
		row := domain.ExportRow{
			OrderID: id, PaidAt: paidAt.Time, PilgrimName: pilgrimName, ProductName: productName,
			BranchName: branchName, AgentName: agentName, Quantity: quantity, RevenueIDR: totalPriceIDR,
			PlatformAmountIDR: platformAmountIDR, AgentCommissionIDR: agentCommissionIDR,
		}
		if supplierCostIDR != nil {
			row.CostKnown = true
			row.CostIDR = *supplierCostIDR * int64(quantity)
		}
		if err := emit(row); err != nil {
			return err
		}
	}
	return databaseError(rows.Err())
}
