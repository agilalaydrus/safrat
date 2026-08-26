package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TransferAccount is the bank account operators send manual transfers to. It is
// shown next to the unique amount, because the amount alone is useless without
// somewhere to send it.
type TransferAccount struct {
	BankName      string
	AccountNumber string
	AccountHolder string
}

type SubscriptionService struct {
	repository         *repository.SubscriptionRepository
	operatorRepository *repository.OperatorRepository
	xenditClient       *payment.Client
	transferAccount    TransferAccount
	appBaseURL         string
}

func NewSubscriptionService(subscriptions *repository.SubscriptionRepository, operators *repository.OperatorRepository, xendit *payment.Client, transfer TransferAccount, appBaseURL string) *SubscriptionService {
	return &SubscriptionService{repository: subscriptions, operatorRepository: operators, xenditClient: xendit, transferAccount: transfer, appBaseURL: appBaseURL}
}

func (s *SubscriptionService) GetMine(ctx context.Context, orgID string) (*hajjv1.GetMySubscriptionResponse, error) {
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SubscriptionService.GetMine", err)
	}
	// An operator created before subscriptions existed, or mid-signup, still
	// needs an answer rather than an error.
	if err := s.repository.EnsureForOperator(ctx, operator.ID); err != nil {
		return nil, serviceError("SubscriptionService.GetMine", err)
	}
	access, err := s.repository.GetAccess(ctx, operator.ID)
	if err != nil {
		return nil, serviceError("SubscriptionService.GetMine", err)
	}
	response := &hajjv1.GetMySubscriptionResponse{
		Plan: access.Plan, Status: access.Status,
		AccessUntil: timestamppb.New(access.AccessUntil), Active: access.Allowed,
		TransferBankName:      s.transferAccount.BankName,
		TransferAccountNumber: s.transferAccount.AccountNumber,
		TransferAccountHolder: s.transferAccount.AccountHolder,
	}
	pending, err := s.repository.PendingInvoice(ctx, operator.ID)
	if err == nil {
		response.PendingInvoice = invoiceMessage(pending)
	}
	return response, nil
}

func (s *SubscriptionService) ListMine(ctx context.Context, orgID string, limit int32) (*hajjv1.ListMyInvoicesResponse, error) {
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SubscriptionService.ListMine", err)
	}
	invoices, err := s.repository.ListInvoices(ctx, operator.ID, limit)
	if err != nil {
		return nil, serviceError("SubscriptionService.ListMine", err)
	}
	response := &hajjv1.ListMyInvoicesResponse{Invoices: make([]*hajjv1.SubscriptionInvoice, 0, len(invoices))}
	for _, invoice := range invoices {
		response.Invoices = append(response.Invoices, invoiceMessage(invoice))
	}
	return response, nil
}

func (s *SubscriptionService) CreateInvoice(ctx context.Context, orgID string, request *hajjv1.CreateInvoiceRequest) (*hajjv1.SubscriptionInvoice, error) {
	if request == nil {
		return nil, serviceError("SubscriptionService.CreateInvoice", apperror.ErrValidation)
	}
	plan := strings.ToUpper(strings.TrimSpace(request.Plan))
	if _, err := s.repository.PlanPrice(ctx, plan); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("paket tidak dikenal"))
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("SubscriptionService.CreateInvoice", err)
	}
	// One unpaid invoice at a time. Issuing a second would put two unique
	// amounts in play for the same operator, and a transfer against the older
	// one would look unmatched.
	if existing, err := s.repository.PendingInvoice(ctx, operator.ID); err == nil {
		return invoiceMessage(existing), nil
	}

	if request.Channel == hajjv1.PaymentChannel_PAYMENT_CHANNEL_GATEWAY {
		return s.createGatewayInvoice(ctx, operator.ID, operator.Email, plan)
	}
	invoice, err := s.repository.IssueBankTransferInvoice(ctx, operator.ID, plan)
	if errors.Is(err, repository.ErrTransferAmountUnavailable) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("nomor transfer sedang penuh; gunakan pembayaran otomatis atau coba lagi nanti"))
	}
	if err != nil {
		return nil, serviceError("SubscriptionService.CreateInvoice", err)
	}
	return invoiceMessage(invoice), nil
}

func (s *SubscriptionService) createGatewayInvoice(ctx context.Context, operatorID, email, plan string) (*hajjv1.SubscriptionInvoice, error) {
	if s.xenditClient == nil || !s.xenditClient.Configured() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("pembayaran otomatis belum aktif; gunakan transfer bank"))
	}
	amount, err := s.repository.PlanPrice(ctx, plan)
	if err != nil {
		return nil, serviceError("SubscriptionService.CreateInvoice", err)
	}
	// Unique per attempt, not per operator+plan: an operator who lets one
	// gateway invoice expire and starts another would otherwise reuse the same
	// reference, and the second could settle the first.
	externalID := "sub-" + uuid.NewString()
	created, err := s.xenditClient.CreateInvoice(ctx, payment.CreateInvoiceRequest{
		ExternalID:         externalID,
		Amount:             amount,
		PayerEmail:         email,
		Description:        fmt.Sprintf("Langganan TawafiqHub — paket %s", plan),
		SuccessRedirectURL: s.appBaseURL + "/dashboard/settings?langganan=berhasil",
		FailureRedirectURL: s.appBaseURL + "/dashboard/settings?langganan=gagal",
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("gagal membuat tagihan pembayaran; coba lagi sebentar lagi"))
	}
	invoice, err := s.repository.IssueGatewayInvoice(ctx, operatorID, plan, created.ID, created.InvoiceURL)
	if err != nil {
		return nil, serviceError("SubscriptionService.CreateInvoice", err)
	}
	return invoiceMessage(invoice), nil
}

func invoiceMessage(invoice repository.Invoice) *hajjv1.SubscriptionInvoice {
	channel := hajjv1.PaymentChannel_PAYMENT_CHANNEL_BANK_TRANSFER
	if invoice.Channel == "GATEWAY" {
		channel = hajjv1.PaymentChannel_PAYMENT_CHANNEL_GATEWAY
	}
	return &hajjv1.SubscriptionInvoice{
		Id: invoice.ID, Plan: invoice.Plan, Status: invoice.Status, Channel: channel,
		BaseAmountIdr: invoice.BaseAmount, AmountIdr: invoice.Amount,
		PeriodStart: timestamppb.New(invoice.PeriodStart),
		PeriodEnd:   timestamppb.New(invoice.PeriodEnd),
		DueAt:       timestamppb.New(invoice.DueAt),
		CheckoutUrl: invoice.CheckoutURL,
	}
}
