package repository

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
)

type CashFlowRepository struct {
	queries *db.Queries
}

func NewCashFlowRepository(queries *db.Queries) *CashFlowRepository {
	return &CashFlowRepository{queries: queries}
}

func (r *CashFlowRepository) CreatePayment(ctx context.Context, operatorID, seasonID, vendorName, category, description string, amountIDR int64, dueDate time.Time) (*domain.VendorPayment, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateVendorPayment(ctx, db.CreateVendorPaymentParams{
		OperatorID: opUUID, SeasonID: seasonUUID, VendorName: vendorName, Category: category,
		Description: description, AmountIdr: amountIDR, DueDate: pgDate(&dueDate),
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toVendorPayment(row), nil
}

func (r *CashFlowRepository) ListPayments(ctx context.Context, operatorID, seasonID string) ([]*domain.VendorPayment, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListVendorPayments(ctx, db.ListVendorPaymentsParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.VendorPayment, 0, len(rows))
	for _, row := range rows {
		result = append(result, toVendorPayment(row))
	}
	return result, nil
}

func (r *CashFlowRepository) UpdatePaymentStatus(ctx context.Context, operatorID, id, status string) (*domain.VendorPayment, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpdateVendorPaymentStatus(ctx, db.UpdateVendorPaymentStatusParams{ID: idUUID, OperatorID: opUUID, Status: status})
	if err != nil {
		return nil, databaseError(err)
	}
	return toVendorPayment(row), nil
}

func (r *CashFlowRepository) DeletePayment(ctx context.Context, operatorID, id string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	return databaseError(r.queries.DeleteVendorPayment(ctx, db.DeleteVendorPaymentParams{ID: idUUID, OperatorID: opUUID}))
}

func (r *CashFlowRepository) GetSummary(ctx context.Context, operatorID, seasonID string) (*domain.CashFlowSummary, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	collected, err := r.queries.GetSeasonPaidTotal(ctx, db.GetSeasonPaidTotalParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	commitment, err := r.queries.GetVendorCommitmentSummary(ctx, db.GetVendorCommitmentSummaryParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	unpaidCount, err := r.queries.CountUnpaidPilgrims(ctx, db.CountUnpaidPilgrimsParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.CashFlowSummary{
		TotalCollectedIDR: collected, TotalCommittedIDR: commitment.TotalCommittedIdr,
		TotalPaidOutIDR: commitment.TotalPaidOutIdr, TotalOutstandingIDR: commitment.TotalOutstandingIdr,
		TotalOverdueIDR: commitment.TotalOverdueIdr, DueNext30DaysIDR: commitment.DueNext30DaysIdr,
		UnpaidPilgrimCount: unpaidCount,
	}, nil
}

func (r *CashFlowRepository) GetMonthlyProjection(ctx context.Context, operatorID, seasonID string) ([]*domain.MonthlyProjectionEntry, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.GetMonthlyProjection(ctx, db.GetMonthlyProjectionParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.MonthlyProjectionEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.MonthlyProjectionEntry{Month: row.Month.Time, VendorObligationsIDR: row.VendorObligationsIdr, PaymentCount: row.PaymentCount})
	}
	return result, nil
}

func toVendorPayment(value db.VendorPayment) *domain.VendorPayment {
	payment := &domain.VendorPayment{
		ID: uuidString(value.ID), OperatorID: uuidString(value.OperatorID), SeasonID: uuidString(value.SeasonID),
		VendorName: value.VendorName, Category: value.Category, Description: value.Description,
		AmountIDR: value.AmountIdr, DueDate: value.DueDate.Time, Status: value.Status, CreatedAt: value.CreatedAt.Time,
	}
	if value.PaidAt.Valid {
		t := value.PaidAt.Time
		payment.PaidAt = &t
	}
	return payment
}
