package service

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const dueDateLayout = "2006-01-02"

type CashFlowService struct {
	operatorRepository *repository.OperatorRepository
	cashFlowRepository *repository.CashFlowRepository
}

func NewCashFlowService(operators *repository.OperatorRepository, cashFlow *repository.CashFlowRepository) *CashFlowService {
	return &CashFlowService{operatorRepository: operators, cashFlowRepository: cashFlow}
}

func (s *CashFlowService) CreatePayment(ctx context.Context, authenticatedOrgID string, req *hajjv1.CreateVendorPaymentRequest) (*hajjv1.VendorPayment, error) {
	if req == nil || !isUUID(req.SeasonId) || req.AmountIdr <= 0 {
		return nil, serviceError("CashFlowService.CreatePayment", apperror.ErrValidation)
	}
	dueDate, err := time.Parse(dueDateLayout, req.DueDate)
	if err != nil {
		return nil, serviceError("CashFlowService.CreatePayment", apperror.ErrValidation)
	}
	category := req.Category
	if category == "" {
		category = "HOTEL"
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.CreatePayment", err)
	}
	payment, err := s.cashFlowRepository.CreatePayment(ctx, operator.ID, req.SeasonId, req.VendorName, category, req.Description, req.AmountIdr, dueDate)
	if err != nil {
		return nil, serviceError("CashFlowService.CreatePayment", err)
	}
	return vendorPaymentMessage(payment), nil
}

func (s *CashFlowService) ListPayments(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListVendorPaymentsRequest) (*hajjv1.ListVendorPaymentsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("CashFlowService.ListPayments", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.ListPayments", err)
	}
	payments, err := s.cashFlowRepository.ListPayments(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("CashFlowService.ListPayments", err)
	}
	result := &hajjv1.ListVendorPaymentsResponse{Payments: make([]*hajjv1.VendorPayment, 0, len(payments))}
	for _, payment := range payments {
		result.Payments = append(result.Payments, vendorPaymentMessage(payment))
	}
	return result, nil
}

func (s *CashFlowService) UpdatePaymentStatus(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdateVendorPaymentStatusRequest) (*hajjv1.VendorPayment, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("CashFlowService.UpdatePaymentStatus", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.UpdatePaymentStatus", err)
	}
	payment, err := s.cashFlowRepository.UpdatePaymentStatus(ctx, operator.ID, req.Id, req.Status)
	if err != nil {
		return nil, serviceError("CashFlowService.UpdatePaymentStatus", err)
	}
	return vendorPaymentMessage(payment), nil
}

func (s *CashFlowService) DeletePayment(ctx context.Context, authenticatedOrgID string, req *hajjv1.DeleteVendorPaymentRequest) (*hajjv1.DeleteVendorPaymentResponse, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("CashFlowService.DeletePayment", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.DeletePayment", err)
	}
	if err := s.cashFlowRepository.DeletePayment(ctx, operator.ID, req.Id); err != nil {
		return nil, serviceError("CashFlowService.DeletePayment", err)
	}
	return &hajjv1.DeleteVendorPaymentResponse{}, nil
}

// GetSummary computes net_position itself — total_collected_idr minus
// total_outstanding_idr — rather than trusting anything from the client
// or leaving it to a DB column.
func (s *CashFlowService) GetSummary(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetCashFlowSummaryRequest) (*hajjv1.CashFlowSummary, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("CashFlowService.GetSummary", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.GetSummary", err)
	}
	summary, err := s.cashFlowRepository.GetSummary(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("CashFlowService.GetSummary", err)
	}
	return &hajjv1.CashFlowSummary{
		TotalCollectedIdr: summary.TotalCollectedIDR, TotalCommittedIdr: summary.TotalCommittedIDR,
		TotalPaidOutIdr: summary.TotalPaidOutIDR, TotalOutstandingIdr: summary.TotalOutstandingIDR,
		TotalOverdueIdr: summary.TotalOverdueIDR, DueNext_30DaysIdr: summary.DueNext30DaysIDR,
		UnpaidPilgrimCount: summary.UnpaidPilgrimCount,
		NetPositionIdr:     summary.TotalCollectedIDR - summary.TotalOutstandingIDR,
	}, nil
}

func (s *CashFlowService) GetMonthlyProjection(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetMonthlyProjectionRequest) (*hajjv1.GetMonthlyProjectionResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("CashFlowService.GetMonthlyProjection", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.GetMonthlyProjection", err)
	}
	entries, err := s.cashFlowRepository.GetMonthlyProjection(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("CashFlowService.GetMonthlyProjection", err)
	}
	result := &hajjv1.GetMonthlyProjectionResponse{Months: make([]*hajjv1.MonthlyProjectionEntry, 0, len(entries))}
	for _, entry := range entries {
		result.Months = append(result.Months, &hajjv1.MonthlyProjectionEntry{
			Month: entry.Month.Format("2006-01"), VendorObligationsIdr: entry.VendorObligationsIDR, PaymentCount: int32(entry.PaymentCount),
		})
	}
	return result, nil
}

func vendorPaymentMessage(value *domain.VendorPayment) *hajjv1.VendorPayment {
	payment := &hajjv1.VendorPayment{
		Id: value.ID, SeasonId: value.SeasonID, VendorName: value.VendorName, Category: value.Category,
		Description: value.Description, AmountIdr: value.AmountIDR, DueDate: value.DueDate.Format(dueDateLayout),
		Status: value.Status, CreatedAt: timestamppb.New(value.CreatedAt),
	}
	if value.PaidAt != nil {
		payment.PaidAt = timestamppb.New(*value.PaidAt)
	}
	return payment
}
