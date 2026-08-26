package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
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
	agentRepository    *repository.AgentRepository
	refundRepository   *repository.RefundRepository
	db                 *pgxpool.Pool
	// appBaseURL is where Xendit redirects the pilgrim's browser back to
	// after payment — CORS_ALLOWED_ORIGIN doubles as this app's canonical
	// web origin, so no separate env var.
	appBaseURL string
}

func NewOrderService(operators *repository.OperatorRepository, pilgrims *repository.PilgrimRepository, products *repository.ProductRepository, orders *repository.OrderRepository, audit *repository.AuditRepository, ledger *repository.LedgerRepository, refunds *repository.RefundRepository, agents *repository.AgentRepository, db *pgxpool.Pool, xendit *payment.Client, appBaseURL string) *OrderService {
	return &OrderService{operatorRepository: operators, pilgrimRepository: pilgrims, productRepository: products, orderRepository: orders, auditRepository: audit, ledgerRepository: ledger, refundRepository: refunds, agentRepository: agents, db: db, xenditClient: xendit, appBaseURL: appBaseURL}
}

// orderSplit is how one transaction's money divides.
type orderSplit struct {
	TotalPrice      int64
	PlatformAmount  int64
	OperatorAmount  int64
	AgentCommission int64
}

// computeSplit derives the platform/operator/agent split for a quantity of a
// product — shared by every lane that creates an order, so they can never
// diverge on how money gets divided.
//
// agentID is the *referrer*, and commission is zero without one (CODEX_SPEC
// §7). It is deliberately not "whoever placed the order": an agent selling to
// a jamaah somebody else referred must not collect that referrer's commission.
func computeSplit(product *domain.Product, quantity int32, agentID string) orderSplit {
	split := orderSplit{TotalPrice: product.PriceIDR * int64(quantity)}
	// Rounds down — a fraction of a rupiah has nowhere to go, and
	// under-crediting by a fraction is the safe direction for a split
	// that must sum to <= total, never over.
	split.PlatformAmount = int64(float64(split.TotalPrice) * product.PlatformMarginPct)
	split.OperatorAmount = int64(float64(split.TotalPrice) * product.OperatorMarginPct)
	if strings.TrimSpace(agentID) != "" {
		split.AgentCommission = int64(float64(split.TotalPrice) * product.AgentMarginPct)
	}
	return split
}

// CreateOrder runs through the public (app_access_code) lane, same as the
// rest of PilgrimAppService — a pilgrim checks out from their own device,
// no Better Auth session. Computes the platform/operator/agent commission
// split from the product's configured margins and freezes it onto the
// order row before ever calling Xendit, so the split is correct even if
// the invoice creation call fails.
func (s *OrderService) CreateOrder(ctx context.Context, req *hajjv1.CreateOrderRequest) (*hajjv1.CreateOrderResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" || strings.TrimSpace(req.ProductId) == "" ||
		req.Quantity < 1 || strings.TrimSpace(req.IdempotencyKey) == "" {
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

	// The referral is what earns the commission, and it holds however the
	// jamaah reaches checkout — buying from their own phone included. This
	// used to be hard-coded to zero, so an agent's referral produced nothing
	// the moment the jamaah bought for themselves.
	split := computeSplit(product, req.Quantity, info.AgentID)

	order, created, err := s.orderRepository.Create(ctx, repository.CreateOrderParams{
		OperatorID: info.OperatorID, SeasonID: info.SeasonID, PilgrimID: info.ID,
		ProductID: req.ProductId, AgentID: info.AgentID, Quantity: req.Quantity,
		UnitPriceIDR: product.PriceIDR, TotalPriceIDR: split.TotalPrice,
		PlatformAmountIDR: split.PlatformAmount, OperatorAmountIDR: split.OperatorAmount,
		AgentCommissionIDR: split.AgentCommission, IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		return nil, serviceError("OrderService.CreateOrder", err)
	}
	// A replay of the same key: the order already exists and already has its
	// invoice. Creating a second invoice here is exactly the double charge the
	// key exists to prevent.
	if !created {
		return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: order.XenditInvoiceURL}, nil
	}
	s.recordCommission(ctx, order)

	invoice, err := s.xenditClient.CreateInvoice(ctx, payment.CreateInvoiceRequest{
		ExternalID:         order.ID,
		Amount:             split.TotalPrice,
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
	if req == nil || !isUUID(req.PilgrimId) || !isUUID(req.ProductId) || req.Quantity < 1 || req.Quantity > 20 ||
		strings.TrimSpace(req.IdempotencyKey) == "" {
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

	// Staff selling on a jamaah's behalf does not change who referred them, so
	// the commission still follows the referral.
	split := computeSplit(product, req.Quantity, pilgrim.AgentID)
	order, created, err := s.orderRepository.Create(ctx, repository.CreateOrderParams{
		OperatorID: op.ID, SeasonID: pilgrim.SeasonID, PilgrimID: pilgrim.ID,
		ProductID: req.ProductId, AgentID: pilgrim.AgentID, Quantity: req.Quantity,
		UnitPriceIDR: product.PriceIDR, TotalPriceIDR: split.TotalPrice,
		PlatformAmountIDR: split.PlatformAmount, OperatorAmountIDR: split.OperatorAmount,
		AgentCommissionIDR: split.AgentCommission, IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		return nil, serviceError("OrderService.CreateManualOrder", err)
	}
	order.ProductName = product.Name
	order.PilgrimName = pilgrim.FullName
	if !created {
		return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: order.XenditInvoiceURL}, nil
	}
	s.recordCommission(ctx, order)

	userID := middleware.UserIDFromCtx(ctx)
	checkoutURL := ""
	if method == "XENDIT_LINK" {
		invoice, err := s.xenditClient.CreateInvoice(ctx, payment.CreateInvoiceRequest{
			ExternalID:         order.ID,
			Amount:             split.TotalPrice,
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
		fmt.Sprintf("%s x%d — Rp%d via %s%s", product.Name, req.Quantity, split.TotalPrice, method, noteSuffix(req.Note)))
	return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: checkoutURL}, nil
}

// applyPaidSideEffects runs the TRAVEL_PACKAGE auto-kloter-assign cascade
// (see product.default_kloter_id) once an order actually reaches PAID —
// best-effort: a failure here never undoes the payment, it's just logged.
func (s *OrderService) applyPaidSideEffects(ctx context.Context, product *domain.Product, order *domain.Order) {
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

// RefundOrder records that money was returned to a pilgrim, credits their
// balance ledger, and reverses the agent's commission.
//
// A refund is always the whole transaction. The request carries no amount, so
// a partial refund is not something a caller can ask for, and the database
// refuses one anyway (migration 093) — the rule holds for paths that never
// reach this function.
//
// It records a refund; it does not itself move money at the gateway. Operators
// refund by transfer or at the counter, and what has to be true afterwards is
// that the books agree with reality. Automating the gateway call on top of an
// honest record is a smaller job than the other way around.
//
// Everything happens in one transaction under a row lock on the order: the
// refund record, the pilgrim's credit, the commission reversal and the order's
// status either all hold or none do. A half-applied refund would credit a
// pilgrim while leaving the agent paid for a sale that no longer exists.
func (s *OrderService) RefundOrder(ctx context.Context, orgID, userID string, req *hajjv1.RefundOrderRequest) (*hajjv1.RefundOrderResponse, error) {
	if req == nil || strings.TrimSpace(req.OrderId) == "" || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("OrderService.RefundOrder", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := s.refundRepository.LockOrderForRefund(ctx, tx, op.ID, req.OrderId)
	if err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}
	// The idempotency check comes first, before any precondition. A refund
	// leaves the order REFUNDED, so a retry after a lost response would
	// otherwise be rejected by the state its own original request created —
	// and the caller, having never seen the first answer, would conclude the
	// refund failed. The same key is always an advice about the same refund.
	if existing, err := s.refundRepository.FindRefundByKeyTx(ctx, tx, order.ID, req.IdempotencyKey); err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	} else if existing != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, serviceError("OrderService.RefundOrder", err)
		}
		return s.refundResponse(ctx, op.ID, order.ID, existing, false)
	}

	// Only money that actually arrived can go back. PENDING, EXPIRED, FAILED
	// and CANCELLED orders were never paid; a REFUNDED one has already been
	// returned, and cannot be returned again.
	if order.Status != "PAID" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("hanya pesanan berstatus LUNAS yang dapat direfund"))
	}

	// The whole transaction, and the whole commission with it. Commission is
	// earned only on a sale that stands, and no part of this sale does.
	refundAmount := order.TotalPriceIDR
	commissionReversal := int64(0)
	if order.AgentID != "" && order.AgentCommissionIDR > 0 {
		commissionReversal = order.AgentCommissionIDR
	}

	refund, created, err := s.refundRepository.CreateRefundTx(ctx, tx, repository.RefundParams{
		OperatorID: op.ID, OrderID: order.ID, AmountIDR: refundAmount,
		CommissionReversedIDR: commissionReversal, Reason: strings.TrimSpace(req.Reason),
		CreatedByUserID: userID, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}
	// A replay: the refund it asked for already exists, and its effects were
	// applied by the request that created it. Report that outcome instead of
	// applying anything a second time.
	if !created {
		if err := tx.Commit(ctx); err != nil {
			return nil, serviceError("OrderService.RefundOrder", err)
		}
		return s.refundResponse(ctx, op.ID, order.ID, refund, false)
	}

	if err := s.ledgerRepository.AppendBalanceTx(ctx, tx, repository.BalanceEntry{
		OperatorID: op.ID, PilgrimID: order.PilgrimID, AmountIDR: refundAmount, Kind: "REFUND",
		OrderID: order.ID, Note: refundNote(refund.Reason), CreatedByUserID: userID,
		IdempotencyKey: "refund-" + refund.ID,
	}); err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}

	if commissionReversal > 0 {
		if err := s.ledgerRepository.AppendCommissionTx(ctx, tx, repository.CommissionEntry{
			OperatorID: op.ID, AgentID: order.AgentID, AmountIDR: -commissionReversal, Kind: "REVERSED",
			OrderID: order.ID, Note: refundNote(refund.Reason), CreatedByUserID: userID,
			IdempotencyKey: "reversal-" + refund.ID,
		}); err != nil {
			return nil, serviceError("OrderService.RefundOrder", err)
		}
	}

	if err := s.refundRepository.MarkOrderRefundedTx(ctx, tx, order.ID); err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}

	// Audited after the commit: the refund is the fact, and failing to write
	// the log must not roll back money that has already been returned.
	_ = s.auditRepository.Write(ctx, op.ID, userID, "order_refunded", "order", order.ID,
		fmt.Sprintf("Refund penuh Rp%d, komisi ditarik Rp%d%s",
			refundAmount, commissionReversal, noteSuffix(refund.Reason)))

	return s.refundResponse(ctx, op.ID, order.ID, refund, true)
}

func (s *OrderService) refundResponse(ctx context.Context, operatorID, orderID string, refund *domain.OrderRefund, created bool) (*hajjv1.RefundOrderResponse, error) {
	order, err := s.orderRepository.Get(ctx, operatorID, orderID)
	if err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}
	balance, err := s.ledgerRepository.PilgrimBalance(ctx, order.PilgrimID)
	if err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}
	return &hajjv1.RefundOrderResponse{
		Order: orderMessage(order), Refund: refundMessage(refund),
		PilgrimBalanceIdr: balance, Created: created,
	}, nil
}

// ListOrderRefunds returns an order's refund history for the dashboard.
func (s *OrderService) ListOrderRefunds(ctx context.Context, orgID string, req *hajjv1.ListOrderRefundsRequest) (*hajjv1.ListOrderRefundsResponse, error) {
	if req == nil || strings.TrimSpace(req.OrderId) == "" {
		return nil, serviceError("OrderService.ListOrderRefunds", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.ListOrderRefunds", err)
	}
	refunds, err := s.refundRepository.ListByOrder(ctx, op.ID, req.OrderId)
	if err != nil {
		return nil, serviceError("OrderService.ListOrderRefunds", err)
	}
	result := &hajjv1.ListOrderRefundsResponse{Refunds: make([]*hajjv1.OrderRefund, 0, len(refunds))}
	for _, refund := range refunds {
		result.TotalRefundedIdr += refund.AmountIDR
		result.Refunds = append(result.Refunds, refundMessage(refund))
	}
	return result, nil
}

func refundNote(reason string) string {
	if reason == "" {
		return "Refund pesanan"
	}
	return "Refund pesanan: " + reason
}

func refundMessage(r *domain.OrderRefund) *hajjv1.OrderRefund {
	return &hajjv1.OrderRefund{
		Id: r.ID, OrderId: r.OrderID, AmountIdr: r.AmountIDR,
		CommissionReversedIdr: r.CommissionReversedIDR, Reason: r.Reason,
		CreatedByUserId: r.CreatedByUserID, CreatedAt: timestamppb.New(r.CreatedAt),
	}
}

// CreateOrderForPilgrim lets an agent or Muttawwif sell to any jamaah of their
// operator, without going through operator staff.
//
// Selling is open; earning is not, and the two are kept apart deliberately:
//
//   - Who may place it. Any agent of this operator, for any of its jamaah.
//     The caller is still resolved from their own Better Auth identity, so
//     they can only ever act as themselves, and only inside their operator.
//   - Who earns from it. The commission follows the jamaah's referral, exactly
//     as it does when the jamaah buys for themselves or staff sells to them.
//     Selling to somebody else's referral credits that referrer, not the
//     seller, who is recorded separately as placed_by_agent_id.
//
// That separation is the whole point: transacting is free, but it cannot be
// used to take a commission that belongs to whoever brought the jamaah in.
//
// A jamaah with no referrer produces no commission at all. Making the seller
// the referrer would let an agent claim an unreferred jamaah simply by selling
// to them, quietly and permanently.
func (s *OrderService) CreateOrderForPilgrim(ctx context.Context, orgID, userID string, req *hajjv1.CreateOrderForPilgrimRequest) (*hajjv1.CreateOrderResponse, error) {
	if req == nil || !isUUID(req.PilgrimId) || !isUUID(req.ProductId) || req.Quantity < 1 ||
		strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", err)
	}
	agent, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", err)
	}
	pilgrim, err := s.pilgrimRepository.Get(ctx, op.ID, req.PilgrimId)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", err)
	}
	product, err := s.productRepository.GetByID(ctx, op.ID, req.ProductId)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", apperror.ErrNotFound)
	}
	if !product.IsActive {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", apperror.ErrFailedPrecondition)
	}
	// Checked before the order exists, for the same reason as CreateOrder: an
	// order nobody can pay for should never have been created.
	if !s.xenditClient.Configured() {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", fmt.Errorf("%w: %w", apperror.ErrFailedPrecondition, payment.ErrNotConfigured))
	}

	split := computeSplit(product, req.Quantity, pilgrim.AgentID)
	order, created, err := s.orderRepository.Create(ctx, repository.CreateOrderParams{
		OperatorID: op.ID, SeasonID: pilgrim.SeasonID, PilgrimID: pilgrim.ID,
		ProductID: req.ProductId, AgentID: pilgrim.AgentID, PlacedByAgentID: agent.ID,
		Quantity: req.Quantity, UnitPriceIDR: product.PriceIDR, TotalPriceIDR: split.TotalPrice,
		PlatformAmountIDR: split.PlatformAmount, OperatorAmountIDR: split.OperatorAmount,
		AgentCommissionIDR: split.AgentCommission, IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", err)
	}
	order.ProductName = product.Name
	order.PilgrimName = pilgrim.FullName
	if !created {
		return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: order.XenditInvoiceURL}, nil
	}
	s.recordCommission(ctx, order)

	invoice, err := s.xenditClient.CreateInvoice(ctx, payment.CreateInvoiceRequest{
		ExternalID:         order.ID,
		Amount:             split.TotalPrice,
		Description:        fmt.Sprintf("%s — %s", product.Name, pilgrim.FullName),
		SuccessRedirectURL: s.appBaseURL + "/agent?order=success",
		FailureRedirectURL: s.appBaseURL + "/agent?order=failed",
	})
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", fmt.Errorf("create xendit invoice: %w", err))
	}
	if err := s.orderRepository.SetXenditInvoice(ctx, order.ID, invoice.ID, invoice.InvoiceURL); err != nil {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", err)
	}
	order.XenditInvoiceID = invoice.ID
	order.XenditInvoiceURL = invoice.InvoiceURL

	_ = s.auditRepository.Write(ctx, op.ID, userID, "agent_order_created", "order", order.ID,
		fmt.Sprintf("%s x%d — Rp%d untuk %s oleh agen %s",
			product.Name, req.Quantity, split.TotalPrice, pilgrim.FullName, agent.Name))
	return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: invoice.InvoiceURL}, nil
}

// recordCommission recognises an order's commission the moment the order
// exists, not when it is paid.
//
// A pending transaction already counts towards everything related to it
// (owner's rule): the referrer sees what they have made as soon as their
// jamaah transacts, rather than only after settlement. What pending commission
// cannot do is be withdrawn — agent_commission_state separates recognised from
// settled, and a payout may only draw on settled. Failure or a refund takes it
// back with an explicit reversing entry.
//
// Keyed by the order, so a retried creation or a reconciliation sweep records
// the same recognition once. Failures are reported rather than returned: the
// order itself is already real, and refusing here would leave a transaction
// the jamaah can pay but nobody is credited for — the sweep repairs that.
func (s *OrderService) recordCommission(ctx context.Context, order *domain.Order) {
	if s.ledgerRepository == nil || order == nil || order.AgentID == "" || order.AgentCommissionIDR <= 0 {
		return
	}
	if err := s.ledgerRepository.AppendCommission(ctx, repository.CommissionEntry{
		OperatorID: order.OperatorID, AgentID: order.AgentID,
		AmountIDR: order.AgentCommissionIDR, Kind: "EARNED", OrderID: order.ID,
		Note: "Komisi referral dari transaksi", IdempotencyKey: "order-earned-" + order.ID,
	}); err != nil {
		sentry.CaptureException(fmt.Errorf("OrderService.recordCommission: %w", err))
	}
}

// reverseCommission takes back commission on a transaction that will never
// complete. Refunds reverse through RefundOrder instead, which has more to
// undo than this does.
func (s *OrderService) reverseCommission(ctx context.Context, order *domain.Order, reason string) {
	if s.ledgerRepository == nil || order == nil || order.AgentID == "" || order.AgentCommissionIDR <= 0 {
		return
	}
	if err := s.ledgerRepository.AppendCommission(ctx, repository.CommissionEntry{
		OperatorID: order.OperatorID, AgentID: order.AgentID,
		AmountIDR: -order.AgentCommissionIDR, Kind: "REVERSED", OrderID: order.ID,
		Note: reason, IdempotencyKey: "order-reversed-" + order.ID,
	}); err != nil {
		sentry.CaptureException(fmt.Errorf("OrderService.reverseCommission: %w", err))
	}
}

// MarkStatusByInvoiceID moves an order out of PENDING to a status it will
// never leave, and reverses the commission that was recognised when it was
// created. Routed through the service rather than the repository so the
// reversal cannot be skipped by a caller that only knows about the status.
func (s *OrderService) MarkStatusByInvoiceID(ctx context.Context, invoiceID, status string) error {
	order, err := s.orderRepository.MarkStatusByInvoiceID(ctx, invoiceID, status)
	if err != nil {
		return err
	}
	s.reverseCommission(ctx, order, "Komisi ditarik: transaksi "+strings.ToLower(status))
	return nil
}
