package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrderService struct {
	operatorRepository *repository.OperatorRepository
	pilgrimRepository  *repository.PilgrimRepository
	productRepository  *repository.ProductRepository
	orderRepository    *repository.OrderRepository
	auditRepository    *repository.AuditRepository
	xenditClient       *payment.Client
	ledgerRepository   *repository.LedgerRepository
	// appBaseURL is where Xendit redirects the pilgrim's browser back to
	// after payment — CORS_ALLOWED_ORIGIN doubles as this app's canonical
	// web origin, so no separate env var.
	appBaseURL string
}

func NewOrderService(operators *repository.OperatorRepository, pilgrims *repository.PilgrimRepository, products *repository.ProductRepository, orders *repository.OrderRepository, audit *repository.AuditRepository, ledger *repository.LedgerRepository, xendit *payment.Client, appBaseURL string) *OrderService {
	return &OrderService{operatorRepository: operators, pilgrimRepository: pilgrims, productRepository: products, orderRepository: orders, auditRepository: audit, ledgerRepository: ledger, xenditClient: xendit, appBaseURL: appBaseURL}
}

// computeSplit derives the platform/operator/agent commission split for a
// quantity of a product — shared by CreateOrder (pilgrim self-checkout) and
// CreateManualOrder (admin-side), so the two lanes can never diverge on how
// money gets split.
func computeSplit(product *domain.Product, quantity int32) (totalPrice, platformAmount, operatorAmount int64) {
	totalPrice = product.PriceIDR * int64(quantity)
	// Rounds down — a fraction of a rupiah has nowhere to go, and
	// under-crediting by a fraction is the safe direction for a split
	// that must sum to <= total, never over.
	platformAmount = int64(float64(totalPrice) * product.PlatformMarginPct)
	operatorAmount = int64(float64(totalPrice) * product.OperatorMarginPct)
	return totalPrice, platformAmount, operatorAmount
}

// CreateOrder runs through the public (app_access_code) lane, same as the
// rest of PilgrimAppService — a pilgrim checks out from their own device,
// no Better Auth session. Computes the platform/operator/agent commission
// split from the product's configured margins and freezes it onto the
// order row before ever calling Xendit, so the split is correct even if
// the invoice creation call fails.
func (s *OrderService) CreateOrder(ctx context.Context, req *hajjv1.CreateOrderRequest) (*hajjv1.CreateOrderResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" || strings.TrimSpace(req.ProductId) == "" || req.Quantity < 1 {
		return nil, serviceError("OrderService.CreateOrder", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrder", apperror.ErrNotFound)
	}
	product, err := s.productRepository.GetByID(ctx, info.OperatorID, req.ProductId)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrder", apperror.ErrNotFound)
	}
	if !product.IsActive {
		return nil, serviceError("OrderService.CreateOrder", apperror.ErrFailedPrecondition)
	}
	// Checked before creating the order row, not after — an order nobody
	// can ever pay for (Xendit unconfigured) shouldn't exist at all, not
	// sit forever as PENDING.
	if !s.xenditClient.Configured() {
		return nil, serviceError("OrderService.CreateOrder", fmt.Errorf("%w: %w", apperror.ErrFailedPrecondition, payment.ErrNotConfigured))
	}

	totalPrice, platformAmount, operatorAmount := computeSplit(product, req.Quantity)
	// No agent attribution for a pilgrim's own self-checkout in this
	// pass — agentCommission = 0 whenever there's no agent, per §7.
	agentCommission := int64(0)
	agentID := ""

	order, err := s.orderRepository.Create(ctx, info.OperatorID, info.SeasonID, info.ID, req.ProductId, agentID, req.Quantity, product.PriceIDR, totalPrice, platformAmount, operatorAmount, agentCommission)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrder", err)
	}

	invoice, err := s.xenditClient.CreateInvoice(ctx, payment.CreateInvoiceRequest{
		ExternalID:         order.ID,
		Amount:             totalPrice,
		Description:        fmt.Sprintf("%s — %s", product.Name, info.FullName),
		SuccessRedirectURL: s.appBaseURL + "/pilgrim/" + req.AppAccessCode + "/products?order=success",
		FailureRedirectURL: s.appBaseURL + "/pilgrim/" + req.AppAccessCode + "/products?order=failed",
	})
	if err != nil {
		return nil, serviceError("OrderService.CreateOrder", fmt.Errorf("create xendit invoice: %w", err))
	}
	if err := s.orderRepository.SetXenditInvoice(ctx, order.ID, invoice.ID, invoice.InvoiceURL); err != nil {
		return nil, serviceError("OrderService.CreateOrder", err)
	}
	order.XenditInvoiceID = invoice.ID
	order.XenditInvoiceURL = invoice.InvoiceURL
	order.ProductName = product.Name
	order.PilgrimName = info.FullName
	return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: invoice.InvoiceURL}, nil
}

// CreateManualOrder is the authenticated, operator-staff lane — every
// identifier (pilgrim, product) is re-derived and ownership-checked against
// this operator server-side, exactly like the public CreateOrder never
// trusts a client-supplied price. The XENDIT_LINK path is identical to a
// pilgrim's own checkout, just initiated by staff. The CASH/BANK_TRANSFER
// paths skip the gateway entirely and mark the order PAID immediately on
// the operator's word — there's no independent confirmation backing that,
// so it's unconditionally audit-logged with who did it and their note.
func (s *OrderService) CreateManualOrder(ctx context.Context, orgID string, req *hajjv1.CreateManualOrderRequest) (*hajjv1.CreateOrderResponse, error) {
	if req == nil || !isUUID(req.PilgrimId) || !isUUID(req.ProductId) || req.Quantity < 1 || req.Quantity > 20 {
		return nil, serviceError("OrderService.CreateManualOrder", apperror.ErrValidation)
	}
	method := manualOrderMethodToDB(req.PaymentMethod)
	if method == "" {
		return nil, serviceError("OrderService.CreateManualOrder", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.CreateManualOrder", err)
	}
	pilgrim, err := s.pilgrimRepository.Get(ctx, op.ID, req.PilgrimId)
	if err != nil {
		return nil, serviceError("OrderService.CreateManualOrder", err)
	}
	product, err := s.productRepository.GetByID(ctx, op.ID, req.ProductId)
	if err != nil {
		return nil, serviceError("OrderService.CreateManualOrder", apperror.ErrNotFound)
	}
	if !product.IsActive {
		return nil, serviceError("OrderService.CreateManualOrder", apperror.ErrFailedPrecondition)
	}
	if method == "XENDIT_LINK" && !s.xenditClient.Configured() {
		return nil, serviceError("OrderService.CreateManualOrder", fmt.Errorf("%w: %w", apperror.ErrFailedPrecondition, payment.ErrNotConfigured))
	}

	totalPrice, platformAmount, operatorAmount := computeSplit(product, req.Quantity)
	// No agent attribution here either — see CreateOrder. A future "sold by
	// this agent" concept can thread an agent_id through this same request
	// without touching the split logic.
	order, err := s.orderRepository.Create(ctx, op.ID, pilgrim.SeasonID, pilgrim.ID, req.ProductId, "", req.Quantity, product.PriceIDR, totalPrice, platformAmount, operatorAmount, 0)
	if err != nil {
		return nil, serviceError("OrderService.CreateManualOrder", err)
	}
	order.ProductName = product.Name
	order.PilgrimName = pilgrim.FullName

	userID := middleware.UserIDFromCtx(ctx)
	checkoutURL := ""
	if method == "XENDIT_LINK" {
		invoice, err := s.xenditClient.CreateInvoice(ctx, payment.CreateInvoiceRequest{
			ExternalID:         order.ID,
			Amount:             totalPrice,
			Description:        fmt.Sprintf("%s — %s", product.Name, pilgrim.FullName),
			SuccessRedirectURL: s.appBaseURL + "/dashboard/orders?order=success",
			FailureRedirectURL: s.appBaseURL + "/dashboard/orders?order=failed",
		})
		if err != nil {
			return nil, serviceError("OrderService.CreateManualOrder", fmt.Errorf("create xendit invoice: %w", err))
		}
		if err := s.orderRepository.SetXenditInvoice(ctx, order.ID, invoice.ID, invoice.InvoiceURL); err != nil {
			return nil, serviceError("OrderService.CreateManualOrder", err)
		}
		order.XenditInvoiceID = invoice.ID
		order.XenditInvoiceURL = invoice.InvoiceURL
		checkoutURL = invoice.InvoiceURL
	} else {
		paid, err := s.orderRepository.MarkPaidManually(ctx, op.ID, order.ID)
		if err != nil {
			return nil, serviceError("OrderService.CreateManualOrder", err)
		}
		order.Status = paid.Status
		order.PaidAt = paid.PaidAt
		s.applyPaidSideEffects(ctx, product, paid)
	}
	_ = s.auditRepository.Write(ctx, op.ID, userID, "manual_order_created", "order", order.ID,
		fmt.Sprintf("%s x%d — Rp%d via %s%s", product.Name, req.Quantity, totalPrice, method, noteSuffix(req.Note)))
	return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: checkoutURL}, nil
}

// applyPaidSideEffects runs the TRAVEL_PACKAGE auto-kloter-assign cascade
// (see product.default_kloter_id) once an order actually reaches PAID —
// best-effort: a failure here never undoes the payment, it's just logged.
func (s *OrderService) applyPaidSideEffects(ctx context.Context, product *domain.Product, order *domain.Order) {
	// Commission is earned the moment an order is paid, and is recorded as a
	// ledger entry rather than inferred from the order's status. A refund then
	// posts an explicit reversal instead of quietly changing what history says.
	//
	// Keyed by the order, so a redelivered webhook or a re-run job records the
	// same earning once. Failures are reported rather than returned: the
	// payment itself is already settled, and refusing here would only make the
	// provider retry a settlement that succeeded.
	if s.ledgerRepository != nil && order.AgentID != "" && order.AgentCommissionIDR > 0 {
		if err := s.ledgerRepository.AppendCommission(ctx, repository.CommissionEntry{
			OperatorID: order.OperatorID, AgentID: order.AgentID,
			AmountIDR: order.AgentCommissionIDR, Kind: "EARNED", OrderID: order.ID,
			Note: "Komisi dari pesanan lunas", IdempotencyKey: "order-earned-" + order.ID,
		}); err != nil {
			sentry.CaptureException(fmt.Errorf("OrderService.applyPaidSideEffects: append commission: %w", err))
		}
	}
	if product == nil || product.Category != "TRAVEL_PACKAGE" || product.DefaultKloterID == "" {
		return
	}
	if err := s.pilgrimRepository.AssignKloterIfUnset(ctx, order.OperatorID, order.PilgrimID, product.DefaultKloterID); err != nil {
		sentry.CaptureException(fmt.Errorf("OrderService.applyPaidSideEffects: assign kloter: %w", err))
	}
}

// MarkPaidByInvoiceID is called from the Xendit webhook handler (a plain
// net/http handler, not a Connect RPC — see handler/xendit_webhook.go) —
// wraps the repository call so the same paid-order cascade above also
// fires for self-checkout / manual-XENDIT_LINK orders, not just the
// CASH/BANK_TRANSFER path in CreateManualOrder.
func (s *OrderService) MarkPaidByInvoiceID(ctx context.Context, invoiceID string) error {
	order, err := s.orderRepository.MarkPaidByInvoiceID(ctx, invoiceID)
	if err != nil {
		return err
	}
	product, err := s.productRepository.GetByID(ctx, order.OperatorID, order.ProductID)
	if err == nil {
		s.applyPaidSideEffects(ctx, product, order)
	}
	return nil
}

func manualOrderMethodToDB(m hajjv1.ManualOrderPaymentMethod) string {
	switch m {
	case hajjv1.ManualOrderPaymentMethod_MANUAL_ORDER_PAYMENT_METHOD_XENDIT_LINK:
		return "XENDIT_LINK"
	case hajjv1.ManualOrderPaymentMethod_MANUAL_ORDER_PAYMENT_METHOD_CASH:
		return "CASH"
	case hajjv1.ManualOrderPaymentMethod_MANUAL_ORDER_PAYMENT_METHOD_BANK_TRANSFER:
		return "BANK_TRANSFER"
	default:
		return ""
	}
}

func noteSuffix(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return " — " + note
}

func (s *OrderService) ListOrders(ctx context.Context, orgID string, req *hajjv1.ListOrdersRequest) (*hajjv1.ListOrdersResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("OrderService.ListOrders", apperror.ErrValidation)
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.ListOrders", err)
	}
	orders, err := s.orderRepository.ListBySeason(ctx, op.ID, req.SeasonId, req.Limit, req.Offset)
	if err != nil {
		return nil, serviceError("OrderService.ListOrders", err)
	}
	count, err := s.orderRepository.CountBySeason(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("OrderService.ListOrders", err)
	}
	result := &hajjv1.ListOrdersResponse{Orders: make([]*hajjv1.Order, 0, len(orders)), TotalCount: count}
	for _, order := range orders {
		result.Orders = append(result.Orders, orderMessage(order))
	}
	return result, nil
}

func (s *OrderService) GetOrder(ctx context.Context, orgID string, req *hajjv1.GetOrderRequest) (*hajjv1.Order, error) {
	if req == nil || strings.TrimSpace(req.OrderId) == "" {
		return nil, serviceError("OrderService.GetOrder", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.GetOrder", err)
	}
	order, err := s.orderRepository.Get(ctx, op.ID, req.OrderId)
	if err != nil {
		return nil, serviceError("OrderService.GetOrder", err)
	}
	return orderMessage(order), nil
}

func orderMessage(o *domain.Order) *hajjv1.Order {
	result := &hajjv1.Order{
		Id: o.ID, OperatorId: o.OperatorID, SeasonId: o.SeasonID, PilgrimId: o.PilgrimID, PilgrimName: o.PilgrimName,
		ProductId: o.ProductID, ProductName: o.ProductName, AgentId: o.AgentID, AgentName: o.AgentName,
		Quantity: o.Quantity, UnitPriceIdr: o.UnitPriceIDR, TotalPriceIdr: o.TotalPriceIDR,
		PlatformAmountIdr: o.PlatformAmountIDR, OperatorAmountIdr: o.OperatorAmountIDR, AgentCommissionIdr: o.AgentCommissionIDR,
		Status: o.Status, CheckoutUrl: o.XenditInvoiceURL, CreatedAt: timestamppb.New(o.CreatedAt),
	}
	if o.PaidAt != nil {
		result.PaidAt = timestamppb.New(*o.PaidAt)
	}
	return result
}
