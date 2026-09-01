package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const dueDateLayout = "2006-01-02"

type CashFlowService struct {
	operatorRepository *repository.OperatorRepository
	cashFlowRepository *repository.CashFlowRepository
	installments       *repository.InstallmentRepository
	audit              *repository.AuditRepository
	entitlements       *EntitlementChecker
}

func NewCashFlowService(operators *repository.OperatorRepository, cashFlow *repository.CashFlowRepository, installments *repository.InstallmentRepository, audit *repository.AuditRepository, entitlements *EntitlementChecker) *CashFlowService {
	return &CashFlowService{operatorRepository: operators, cashFlowRepository: cashFlow, installments: installments, audit: audit, entitlements: entitlements}
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

func (s *CashFlowService) CreateInstallmentPlan(ctx context.Context, authenticatedOrgID string, req *hajjv1.CreateInstallmentPlanRequest) (*hajjv1.InstallmentPlanDetail, error) {
	if req == nil || !isUUID(req.PilgrimId) || req.GrossAmountIdr <= 0 || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("CashFlowService.CreateInstallmentPlan", apperror.ErrValidation)
	}
	firstDue, err := time.Parse(dueDateLayout, req.FirstDueDate)
	if err != nil {
		return nil, serviceError("CashFlowService.CreateInstallmentPlan", apperror.ErrValidation)
	}
	scheme, count, err := installmentScheme(req.Scheme)
	if err != nil || (scheme == "CASH_BONUS" && (req.CashBonusIdr <= 0 || req.CashBonusIdr >= req.GrossAmountIdr)) ||
		(scheme != "CASH_BONUS" && req.CashBonusIdr != 0) {
		return nil, serviceError("CashFlowService.CreateInstallmentPlan", apperror.ErrValidation)
	}
	payable := req.GrossAmountIdr - req.CashBonusIdr
	if payable < int64(count) {
		return nil, serviceError("CashFlowService.CreateInstallmentPlan", apperror.ErrValidation)
	}
	schedule := buildInstallmentSchedule(scheme, count, payable, firstDue)
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.CreateInstallmentPlan", err)
	}
	if err := s.entitlements.Check(ctx, operator.ID, "installments"); err != nil {
		return nil, serviceError("CashFlowService.CreateInstallmentPlan", err)
	}
	userID := middleware.UserIDFromCtx(ctx)
	detail, created, err := s.installments.CreatePlan(ctx, operator.ID, userID, domain.InstallmentPlanDraft{
		PilgrimID: req.PilgrimId, Scheme: scheme, GrossAmountIDR: req.GrossAmountIdr,
		CashBonusIDR: req.CashBonusIdr, FirstDueDate: firstDue, IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	}, schedule)
	if err != nil {
		return nil, serviceError("CashFlowService.CreateInstallmentPlan", err)
	}
	if created && s.audit != nil {
		_ = s.audit.Write(ctx, operator.ID, userID, "installment_plan_created", "installment_plan", detail.Plan.ID,
			scheme+" · nilai "+formatAuditIDR(detail.Plan.PayableAmountIDR))
	}
	return installmentPlanDetailMessage(detail), nil
}

func (s *CashFlowService) GetPilgrimInstallmentPlan(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetPilgrimInstallmentPlanRequest) (*hajjv1.InstallmentPlanDetail, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("CashFlowService.GetPilgrimInstallmentPlan", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.GetPilgrimInstallmentPlan", err)
	}
	if err := s.entitlements.Check(ctx, operator.ID, "installments"); err != nil {
		return nil, serviceError("CashFlowService.GetPilgrimInstallmentPlan", err)
	}
	detail, err := s.installments.GetPlanByPilgrim(ctx, operator.ID, req.PilgrimId)
	if err != nil {
		return nil, serviceError("CashFlowService.GetPilgrimInstallmentPlan", err)
	}
	return installmentPlanDetailMessage(detail), nil
}

func (s *CashFlowService) ListInstallmentReceivables(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListInstallmentReceivablesRequest) (*hajjv1.ListInstallmentReceivablesResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("CashFlowService.ListInstallmentReceivables", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.ListInstallmentReceivables", err)
	}
	if err := s.entitlements.Check(ctx, operator.ID, "installments"); err != nil {
		return nil, serviceError("CashFlowService.ListInstallmentReceivables", err)
	}
	result, err := s.installments.ListReceivables(ctx, operator.ID, domain.InstallmentReceivableFilter{
		SeasonID: req.SeasonId, Status: req.Status, Search: req.Search, Limit: req.Limit, Offset: req.Offset,
	})
	if err != nil {
		return nil, serviceError("CashFlowService.ListInstallmentReceivables", err)
	}
	response := &hajjv1.ListInstallmentReceivablesResponse{
		Plans: make([]*hajjv1.InstallmentPlan, 0, len(result.Plans)), TotalCount: result.TotalCount,
		TotalReceivableIdr: result.TotalReceivableIDR, TotalOverdueIdr: result.TotalOverdueIDR,
		DueNext_7DaysIdr: result.DueNext7DaysIDR, UnverifiedPaymentCount: result.UnverifiedPaymentCount,
		CollectionRateBps: result.CollectionRateBPS,
	}
	for _, plan := range result.Plans {
		response.Plans = append(response.Plans, installmentPlanMessage(plan))
	}
	return response, nil
}

func (s *CashFlowService) RecordInstallmentPayment(ctx context.Context, authenticatedOrgID string, req *hajjv1.RecordInstallmentPaymentRequest) (*hajjv1.RecordInstallmentPaymentResponse, error) {
	if req == nil || !isUUID(req.InstallmentId) || req.AmountIdr <= 0 || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("CashFlowService.RecordInstallmentPayment", apperror.ErrValidation)
	}
	method, err := installmentPaymentMethod(req.Method)
	if err != nil {
		return nil, serviceError("CashFlowService.RecordInstallmentPayment", err)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.RecordInstallmentPayment", err)
	}
	if err := s.entitlements.Check(ctx, operator.ID, "installments"); err != nil {
		return nil, serviceError("CashFlowService.RecordInstallmentPayment", err)
	}
	userID := middleware.UserIDFromCtx(ctx)
	payment, created, err := s.installments.RecordPayment(ctx, operator.ID, userID, req.InstallmentId, req.AmountIdr,
		method, req.Reference, req.Note, strings.TrimSpace(req.IdempotencyKey))
	if err != nil {
		return nil, serviceError("CashFlowService.RecordInstallmentPayment", err)
	}
	detail, err := s.installments.GetPlanByID(ctx, operator.ID, payment.PlanID)
	if err != nil {
		return nil, serviceError("CashFlowService.RecordInstallmentPayment", err)
	}
	if created && s.audit != nil {
		_ = s.audit.Write(ctx, operator.ID, userID, "installment_payment_recorded", "installment_payment", payment.ID,
			payment.ReceiptNumber+" · "+formatAuditIDR(payment.AmountIDR))
	}
	return &hajjv1.RecordInstallmentPaymentResponse{
		Payment: installmentPaymentMessage(*payment), Detail: installmentPlanDetailMessage(detail), Created: created,
	}, nil
}

func (s *CashFlowService) ReverseInstallmentPayment(ctx context.Context, authenticatedOrgID string, req *hajjv1.ReverseInstallmentPaymentRequest) (*hajjv1.ReverseInstallmentPaymentResponse, error) {
	if req == nil || !isUUID(req.PaymentId) || strings.TrimSpace(req.Reason) == "" || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("CashFlowService.ReverseInstallmentPayment", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("CashFlowService.ReverseInstallmentPayment", err)
	}
	if err := s.entitlements.Check(ctx, operator.ID, "installments"); err != nil {
		return nil, serviceError("CashFlowService.ReverseInstallmentPayment", err)
	}
	userID := middleware.UserIDFromCtx(ctx)
	reversal, created, err := s.installments.ReversePayment(ctx, operator.ID, userID, req.PaymentId, req.Reason, strings.TrimSpace(req.IdempotencyKey))
	if err != nil {
		return nil, serviceError("CashFlowService.ReverseInstallmentPayment", err)
	}
	detail, err := s.installments.GetPlanByID(ctx, operator.ID, reversal.PlanID)
	if err != nil {
		return nil, serviceError("CashFlowService.ReverseInstallmentPayment", err)
	}
	if created && s.audit != nil {
		_ = s.audit.Write(ctx, operator.ID, userID, "installment_payment_reversed", "installment_payment", reversal.ID,
			reversal.ReceiptNumber+" · "+strings.TrimSpace(req.Reason))
	}
	return &hajjv1.ReverseInstallmentPaymentResponse{
		Reversal: installmentPaymentMessage(*reversal), Detail: installmentPlanDetailMessage(detail), Created: created,
	}, nil
}

func installmentScheme(value hajjv1.InstallmentScheme) (string, int, error) {
	switch value {
	case hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_FULL:
		return "FULL", 1, nil
	case hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_DP_50:
		return "DP_50", 2, nil
	case hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_6X:
		return "INSTALLMENT_6X", 6, nil
	case hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_12X:
		return "INSTALLMENT_12X", 12, nil
	case hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_CASH_BONUS:
		return "CASH_BONUS", 1, nil
	default:
		return "", 0, apperror.ErrValidation
	}
}

func buildInstallmentSchedule(scheme string, count int, payable int64, firstDue time.Time) []domain.InstallmentScheduleDraft {
	result := make([]domain.InstallmentScheduleDraft, 0, count)
	if scheme == "DP_50" {
		first := (payable + 1) / 2
		return []domain.InstallmentScheduleDraft{
			{Number: 1, Label: "DP 50%", DueDate: firstDue, AmountDueIDR: first},
			{Number: 2, Label: "Pelunasan", DueDate: addMonthsClamped(firstDue, 1), AmountDueIDR: payable - first},
		}
	}
	base, remainder := payable/int64(count), payable%int64(count)
	for index := 0; index < count; index++ {
		amount := base
		if int64(index) < remainder {
			amount++
		}
		label := "Pelunasan"
		if scheme == "CASH_BONUS" {
			label = "Pelunasan tunai"
		} else if count > 1 {
			label = "Cicilan " + strconv.Itoa(index+1) + " dari " + strconv.Itoa(count)
		}
		result = append(result, domain.InstallmentScheduleDraft{
			Number: int32(index + 1), Label: label, DueDate: addMonthsClamped(firstDue, index), AmountDueIDR: amount,
		})
	}
	return result
}

func addMonthsClamped(value time.Time, months int) time.Time {
	target := time.Date(value.Year(), value.Month()+time.Month(months), 1, 0, 0, 0, 0, value.Location())
	lastDay := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, value.Location()).Day()
	day := value.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(target.Year(), target.Month(), day, 0, 0, 0, 0, value.Location())
}

func installmentPaymentMethod(value hajjv1.InstallmentPaymentMethod) (string, error) {
	switch value {
	case hajjv1.InstallmentPaymentMethod_INSTALLMENT_PAYMENT_METHOD_CASH:
		return "CASH", nil
	case hajjv1.InstallmentPaymentMethod_INSTALLMENT_PAYMENT_METHOD_BANK_TRANSFER:
		return "BANK_TRANSFER", nil
	case hajjv1.InstallmentPaymentMethod_INSTALLMENT_PAYMENT_METHOD_XENDIT:
		return "XENDIT", nil
	default:
		return "", apperror.ErrValidation
	}
}

func installmentPlanDetailMessage(value *domain.InstallmentPlanDetail) *hajjv1.InstallmentPlanDetail {
	if value == nil {
		return nil
	}
	result := &hajjv1.InstallmentPlanDetail{Plan: installmentPlanMessage(value.Plan)}
	result.Installments = make([]*hajjv1.Installment, 0, len(value.Installments))
	for _, item := range value.Installments {
		result.Installments = append(result.Installments, &hajjv1.Installment{
			Id: item.ID, PlanId: item.PlanID, InstallmentNumber: item.Number, Label: item.Label,
			DueDate: item.DueDate.Format(dueDateLayout), AmountDueIdr: item.AmountDueIDR,
			PaidAmountIdr: item.PaidAmountIDR, OutstandingAmountIdr: item.OutstandingAmountIDR,
			Status: item.Status, DaysOverdue: item.DaysOverdue,
		})
	}
	result.Payments = make([]*hajjv1.InstallmentPayment, 0, len(value.Payments))
	for _, payment := range value.Payments {
		result.Payments = append(result.Payments, installmentPaymentMessage(payment))
	}
	return result
}

func installmentPlanMessage(value domain.InstallmentPlan) *hajjv1.InstallmentPlan {
	progress := int32(0)
	if value.PayableAmountIDR > 0 {
		progress = int32(value.PaidAmountIDR * 100 / value.PayableAmountIDR)
		if progress > 100 {
			progress = 100
		}
	}
	return &hajjv1.InstallmentPlan{
		Id: value.ID, OperatorId: value.OperatorID, SeasonId: value.SeasonID, PilgrimId: value.PilgrimID,
		PilgrimName: value.PilgrimName, BranchId: value.BranchID, Scheme: installmentSchemeMessage(value.Scheme),
		GrossAmountIdr: value.GrossAmountIDR, CashBonusIdr: value.CashBonusIDR, PayableAmountIdr: value.PayableAmountIDR,
		PaidAmountIdr: value.PaidAmountIDR, OutstandingAmountIdr: value.OutstandingAmountIDR,
		ProgressPercent: progress, Status: value.Status, CreatedAt: timestamppb.New(value.CreatedAt),
	}
}

func installmentSchemeMessage(value string) hajjv1.InstallmentScheme {
	switch value {
	case "FULL":
		return hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_FULL
	case "DP_50":
		return hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_DP_50
	case "INSTALLMENT_6X":
		return hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_6X
	case "INSTALLMENT_12X":
		return hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_12X
	case "CASH_BONUS":
		return hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_CASH_BONUS
	default:
		return hajjv1.InstallmentScheme_INSTALLMENT_SCHEME_UNSPECIFIED
	}
}

func installmentPaymentMessage(value domain.InstallmentPayment) *hajjv1.InstallmentPayment {
	method := hajjv1.InstallmentPaymentMethod_INSTALLMENT_PAYMENT_METHOD_UNSPECIFIED
	switch value.Method {
	case "CASH":
		method = hajjv1.InstallmentPaymentMethod_INSTALLMENT_PAYMENT_METHOD_CASH
	case "BANK_TRANSFER":
		method = hajjv1.InstallmentPaymentMethod_INSTALLMENT_PAYMENT_METHOD_BANK_TRANSFER
	case "XENDIT":
		method = hajjv1.InstallmentPaymentMethod_INSTALLMENT_PAYMENT_METHOD_XENDIT
	}
	return &hajjv1.InstallmentPayment{
		Id: value.ID, PlanId: value.PlanID, InstallmentId: value.InstallmentID, Kind: value.Kind,
		AmountIdr: value.AmountIDR, Method: method, Reference: value.Reference, Note: value.Note,
		OriginalPaymentId: value.OriginalPaymentID, VerifiedByUserId: value.VerifiedByUserID,
		ReceiptNumber: value.ReceiptNumber, CreatedAt: timestamppb.New(value.CreatedAt),
	}
}

func formatAuditIDR(value int64) string {
	return "Rp " + formatGroupedInteger(value)
}

func formatGroupedInteger(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}
	digits := strconv.FormatInt(value, 10)
	for index := len(digits) - 3; index > 0; index -= 3 {
		digits = digits[:index] + "." + digits[index:]
	}
	if negative {
		return "-" + digits
	}
	return digits
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
