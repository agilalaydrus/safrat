package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	operatorRepository   *repository.OperatorRepository
	pilgrimRepository    *repository.PilgrimRepository
	productRepository    *repository.ProductRepository
	orderRepository      *repository.OrderRepository
	auditRepository      *repository.AuditRepository
	xenditClient         *payment.Client
	ledgerRepository     *repository.LedgerRepository
	agentRepository      *repository.AgentRepository
	seasonRepository     *repository.SeasonRepository
	fulfilmentService    *FulfilmentService
	fulfilmentRepository *repository.FulfilmentRepository
	refundRepository     *repository.RefundRepository
	db                   *pgxpool.Pool
	// appBaseURL is where Xendit redirects the pilgrim's browser back to
	// after payment — CORS_ALLOWED_ORIGIN doubles as this app's canonical
	// web origin, so no separate env var.
	appBaseURL string
}

func NewOrderService(operators *repository.OperatorRepository, pilgrims *repository.PilgrimRepository, products *repository.ProductRepository, orders *repository.OrderRepository, audit *repository.AuditRepository, ledger *repository.LedgerRepository, refunds *repository.RefundRepository, agents *repository.AgentRepository, seasons *repository.SeasonRepository, db *pgxpool.Pool, xendit *payment.Client, appBaseURL string) *OrderService {
	return &OrderService{operatorRepository: operators, pilgrimRepository: pilgrims, productRepository: products, orderRepository: orders, auditRepository: audit, ledgerRepository: ledger, refundRepository: refunds, agentRepository: agents, seasonRepository: seasons, db: db, xenditClient: xendit, appBaseURL: appBaseURL}
}

// ensurePriceCoversSupplierCost refuses a sale whose platform base is below
// what the product costs TawafiqHub to supply.
//
// Checked at the moment of sale, not only when the price was set: an observed
// supplier cost moves on its own — the supplier raises their rate and the next
// fulfilment records it — so a price that was fine last week can be underwater
// today with nobody having touched it.
//
// A product with no known cost is sold without this check. That is the honest
// position: refusing would block every product nobody has costed, which today
// is all of them, and pretending a floor exists where none does would be
// worse than admitting there is none.
func ensurePriceCoversSupplierCost(product *domain.Product, price Price) error {
	if product == nil || product.SupplierCostIDR == nil {
		return nil
	}
	cost := *product.SupplierCostIDR * int64(price.Quantity)
	if price.BasePriceIDR < cost {
		return connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("harga dasar TawafiqHub %s di bawah harga modal supplier %s; perbarui harga dasar sebelum menjual", rupiah(price.BasePriceIDR), rupiah(cost)))
	}
	return nil
}

// pricePilgrimOrder is the one pricing gate shared by every lane that sells
// to a jamaah. Expected configuration gaps are failed preconditions a person
// can fix, not internal errors.
func pricePilgrimOrder(product *domain.Product, levels domain.PriceLevels, route domain.RouteReadiness, quantity int32, referrerAgentID string) (Price, error) {
	price, err := ComputePrice(levels, quantity, BuyerPilgrim, referrerAgentID)
	if err != nil {
		return Price{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if err := ensurePriceCoversSupplierCost(product, price); err != nil {
		return Price{}, err
	}
	if err := ensureRouteReady(route); err != nil {
		return Price{}, err
	}
	return price, nil
}

// ensureRouteReady refuses a digital product that cannot reach a supplier.
//
// Checked before the payment is created, not after. The old order was: take
// the money, then discover at dispatch that nothing could deliver it — which
// leaves a jamaah holding a paid order and someone having to unwind it by
// hand. A refusal costs a person one message; a refund costs everyone an hour
// and a customer's confidence.
func ensureRouteReady(route domain.RouteReadiness) error {
	if reason := route.Refusal(); reason != "" {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(reason))
	}
	return nil
}

func priceAgentOrder(product *domain.Product, levels domain.PriceLevels, route domain.RouteReadiness, quantity int32) (Price, error) {
	price, err := ComputePrice(levels, quantity, BuyerAgent, "")
	if err != nil {
		return Price{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if err := ensurePriceCoversSupplierCost(product, price); err != nil {
		return Price{}, err
	}
	if err := ensureRouteReady(route); err != nil {
		return Price{}, err
	}
	return price, nil
}

// describeLimitRefusal turns the bare limit error into one a person can act
// on: the cap, and what is left today.
//
// "Transaksi ditolak" alone sends a jamaah to customer service to ask a
// question the system already knows the answer to. If the lookup itself fails
// the original refusal still stands — a limit that cannot be described is
// still a limit, and swallowing it would let the purchase through.
func (s *OrderService) describeLimitRefusal(ctx context.Context, err error, buyerKind, buyerID string) error {
	if errors.Is(err, apperror.ErrCheckoutAttemptLimit) {
		return fmt.Errorf("%w: maksimal 5 checkout pembayaran baru per akun dalam 1 jam; gunakan link yang sudah dibuat atau coba lagi setelah jeda 1 jam",
			apperror.ErrCheckoutAttemptLimit)
	}
	if errors.Is(err, apperror.ErrCheckoutHeldBlocked) {
		return fmt.Errorf("%w: akun ini masih memiliki pembayaran yang sedang diperiksa; selesaikan transaksi tersebut sebelum membuat checkout baru",
			apperror.ErrCheckoutHeldBlocked)
	}
	if !errors.Is(err, apperror.ErrDailyLimitExceeded) {
		return err
	}
	spend, lookupErr := s.orderRepository.DailyDigitalSpend(ctx, buyerKind, buyerID, jakartaToday())
	if lookupErr != nil {
		return err
	}
	remaining := spend.LimitIDR - spend.TotalIDR
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Errorf("%w: batas transaksi produk digital %s per hari; terpakai %s, sisa %s hari ini",
		apperror.ErrDailyLimitExceeded, rupiah(spend.LimitIDR), rupiah(spend.TotalIDR), rupiah(remaining))
}

// checkoutCreateError makes a fraud refusal visible even though PostgreSQL
// rolls the rejected INSERT back. The audit write uses a separate transaction;
// otherwise the evidence would disappear with the order it refused.
func (s *OrderService) checkoutCreateError(ctx context.Context, method, operatorID, userID, buyerKind, buyerID string, err error) error {
	described := s.describeLimitRefusal(ctx, err, buyerKind, buyerID)
	if errors.Is(err, apperror.ErrCheckoutAttemptLimit) || errors.Is(err, apperror.ErrCheckoutHeldBlocked) {
		_ = s.auditRepository.Write(ctx, operatorID, userID, "checkout_attempt_rejected", strings.ToLower(buyerKind), buyerID, described.Error())
	}
	return serviceError(method, described)
}

// jakartaLocation is resolved once. A daily limit that rolled over on UTC
// would reset at 07:00 local time — an evening purchase and the next
// morning's counting against different days, a single afternoon spanning two,
// and nobody able to predict when their limit comes back.
var jakartaLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Fixed offset rather than falling back to UTC. WIB has never observed
		// daylight saving, so +07:00 is exactly right; UTC would silently move
		// every buyer's reset time by seven hours.
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}()

// jakartaToday is the calendar day the daily limit counts against.
func jakartaToday() time.Time {
	now := time.Now().In(jakartaLocation)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakartaLocation)
}

// digitalDestination freezes the provider target on the order. A mutable
// profile phone is only a default; dispatch must never look it up later.
func digitalDestination(product *domain.Product, supplied, fallback string) (string, error) {
	if product == nil || (product.Category != "ROAMING_DATA" && product.Category != "PPOB_CREDIT") {
		return "", nil
	}
	destination := strings.TrimSpace(supplied)
	if destination == "" {
		destination = strings.TrimSpace(fallback)
	}
	if len(destination) < 3 || len(destination) > 100 {
		return "", connect.NewError(connect.CodeFailedPrecondition,
			errors.New("nomor tujuan produk digital belum lengkap"))
	}
	return destination, nil
}

// CreateOrder runs through the public (app_access_code) lane, same as the
// rest of PilgrimAppService — a pilgrim checks out from their own device,
// no Better Auth session. Builds the price from the platform base and this
// operator's configured markups, then freezes every component onto the order
// before ever calling Xendit.
func (s *OrderService) CreateOrder(ctx context.Context, req *hajjv1.CreateOrderRequest) (*hajjv1.CreateOrderResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" || strings.TrimSpace(req.ProductId) == "" ||
		req.Quantity < 1 || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("OrderService.CreateOrder", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrder", apperror.ErrNotFound)
	}
	product, levels, route, err := s.productRepository.Pricing(ctx, info.OperatorID, req.ProductId)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrder", apperror.ErrNotFound)
	}
	if !product.IsActive {
		return nil, serviceError("OrderService.CreateOrder", apperror.ErrFailedPrecondition)
	}
	price, err := pricePilgrimOrder(product, levels, route, req.Quantity, info.AgentID)
	if err != nil {
		return nil, err
	}
	destination, err := digitalDestination(product, "", info.Phone)
	if err != nil {
		return nil, err
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
	order, created, err := s.orderRepository.Create(ctx, repository.CreateOrderParams{
		OperatorID: info.OperatorID, SeasonID: info.SeasonID, PilgrimID: info.ID,
		ProductID: req.ProductId, AgentID: info.AgentID, Quantity: req.Quantity,
		BuyerKind: string(BuyerPilgrim), UnitPriceIDR: price.UnitPriceIDR,
		BasePriceIDR: price.BasePriceIDR, OperatorMarkupIDR: price.OperatorMarkupIDR, AgentMarkupIDR: price.AgentMarkupIDR,
		TotalPriceIDR: price.TotalPriceIDR, PlatformAmountIDR: price.PlatformAmountIDR,
		OperatorAmountIDR: price.OperatorAmountIDR, AgentCommissionIDR: price.AgentCommissionIDR,
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey), Destination: destination, CheckoutChannel: "XENDIT",
		CountsTowardDailyLimit: domain.RoutingRequired(product.Category), SpendDate: jakartaToday(),
	})
	if err != nil {
		return nil, s.checkoutCreateError(ctx, "OrderService.CreateOrder", info.OperatorID, "", string(BuyerPilgrim), info.ID, err)
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
		Amount:             price.TotalPriceIDR,
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
	product, levels, route, err := s.productRepository.Pricing(ctx, op.ID, req.ProductId)
	if err != nil {
		return nil, serviceError("OrderService.CreateManualOrder", apperror.ErrNotFound)
	}
	if !product.IsActive {
		return nil, serviceError("OrderService.CreateManualOrder", apperror.ErrFailedPrecondition)
	}
	price, err := pricePilgrimOrder(product, levels, route, req.Quantity, pilgrim.AgentID)
	if err != nil {
		return nil, err
	}
	destination, err := digitalDestination(product, "", pilgrim.Phone)
	if err != nil {
		return nil, err
	}
	if method == "XENDIT_LINK" && !s.xenditClient.Configured() {
		return nil, serviceError("OrderService.CreateManualOrder", fmt.Errorf("%w: %w", apperror.ErrFailedPrecondition, payment.ErrNotConfigured))
	}
	checkoutChannel := "MANUAL"
	if method == "XENDIT_LINK" {
		checkoutChannel = "XENDIT"
	}

	// Staff selling on a jamaah's behalf does not change who referred them, so
	// the commission still follows the referral.
	order, created, err := s.orderRepository.Create(ctx, repository.CreateOrderParams{
		OperatorID: op.ID, SeasonID: pilgrim.SeasonID, PilgrimID: pilgrim.ID,
		ProductID: req.ProductId, AgentID: pilgrim.AgentID, Quantity: req.Quantity,
		BuyerKind: string(BuyerPilgrim), UnitPriceIDR: price.UnitPriceIDR,
		BasePriceIDR: price.BasePriceIDR, OperatorMarkupIDR: price.OperatorMarkupIDR, AgentMarkupIDR: price.AgentMarkupIDR,
		TotalPriceIDR: price.TotalPriceIDR, PlatformAmountIDR: price.PlatformAmountIDR,
		OperatorAmountIDR: price.OperatorAmountIDR, AgentCommissionIDR: price.AgentCommissionIDR,
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey), Destination: destination, CheckoutChannel: checkoutChannel,
		CountsTowardDailyLimit: domain.RoutingRequired(product.Category), SpendDate: jakartaToday(),
	})
	if err != nil {
		return nil, s.checkoutCreateError(ctx, "OrderService.CreateManualOrder", op.ID, middleware.UserIDFromCtx(ctx), string(BuyerPilgrim), pilgrim.ID, err)
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
			Amount:             price.TotalPriceIDR,
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
		fmt.Sprintf("%s x%d — %s via %s%s", product.Name, req.Quantity, rupiah(price.TotalPriceIDR), method, noteSuffix(req.Note)))
	return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: checkoutURL}, nil
}

// applyPaidSideEffects runs the TRAVEL_PACKAGE auto-kloter-assign cascade
// (see product.default_kloter_id) once an order actually reaches PAID —
// best-effort: a failure here never undoes the payment, it's just logged.
func (s *OrderService) applyPaidSideEffects(ctx context.Context, product *domain.Product, order *domain.Order) {
	// A paid order for anything the platform supplies now owes a delivery, and
	// that debt is recorded rather than assumed. Before this, payment was the
	// end of the story: money taken, nothing ever sent, and no state anywhere
	// saying so.
	//
	// Travel packages are excluded because nobody is buying them from a
	// supplier — the operator fulfils those themselves.
	if s.fulfilmentService != nil && product != nil && product.Category != "TRAVEL_PACKAGE" {
		s.fulfilmentService.Open(ctx, order.ID, order.OperatorID, product.Category)
	}
	if product == nil || product.Category != "TRAVEL_PACKAGE" || product.DefaultKloterID == "" || order.PilgrimID == "" {
		return
	}
	if err := s.pilgrimRepository.AssignKloterIfUnset(ctx, order.OperatorID, order.PilgrimID, product.DefaultKloterID); err != nil {
		sentry.CaptureException(fmt.Errorf("OrderService.applyPaidSideEffects: assign kloter: %w", err))
	}
}

// SettlePayment applies a gateway payment notification to an order.
//
// The amount is checked before anything settles. Commission counts only when
// what was paid matches what was owed (owner's rule), and until now the
// webhook took the gateway's word that *something* was paid — revenue,
// commission and the jamaah's payment history all followed from that.
//
// A mismatch is held rather than rejected. Money did arrive: rejecting would
// strand a real payment, and settling would accept an amount nobody agreed to.
// A held transaction still counts as pending — it neither failed nor was
// refunded — but it cannot settle, so nothing can be paid out on it until a
// human resolves it.
//
// Only PENDING orders move, so a redelivered notification is a harmless no-op.
// SettleFromGateway settles an order by asking Xendit what actually happened,
// rather than believing what arrived at the webhook endpoint.
//
// A delivery is a claim made by whoever could reach that URL. The callback
// token is a static shared secret riding on every request, so it proves rather
// less than it appears to; the outbound API call is authenticated with a key
// that never leaves this server. So the delivery is demoted to a hint — "look
// at this invoice" — and everything that follows comes from Xendit's answer.
//
// The same path serves a poller, which is what makes a dropped delivery
// survivable instead of leaving an order PENDING forever.
func (s *OrderService) SettleFromGateway(ctx context.Context, invoiceID string) error {
	invoice, err := s.xenditClient.FetchInvoice(ctx, invoiceID)
	if err != nil {
		return err
	}
	switch invoice.Status {
	case "PAID", "SETTLED":
		paid := invoice.PaidAmount
		if paid <= 0 {
			paid = invoice.Amount
		}
		return s.SettlePayment(ctx, invoiceID, paid)
	case "EXPIRED":
		return s.MarkStatusByInvoiceID(ctx, invoiceID, "EXPIRED")
	default:
		// Still pending as far as Xendit is concerned, whatever the delivery
		// said. Nothing to do, and doing something would mean acting on the
		// claim we just decided not to trust.
		return nil
	}
}

func (s *OrderService) SettlePayment(ctx context.Context, invoiceID string, paidAmountIDR int64) error {
	order, err := s.orderRepository.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if order.Status != "PENDING" {
		// Already settled, held, or closed. Nothing to do, and re-applying
		// would be exactly the double-count the guard exists to prevent.
		return nil
	}

	holdReasons := make([]string, 0, 2)
	amountMismatch := paidAmountIDR != order.TotalPriceIDR
	if amountMismatch {
		reason := fmt.Sprintf("Nominal dibayar %s tidak sama dengan tagihan %s", rupiah(paidAmountIDR), rupiah(order.TotalPriceIDR))
		if paidAmountIDR <= 0 {
			// The gateway told us nothing usable. That is a configuration
			// fault rather than a suspicious payer, and it must not be read as
			// a matching amount.
			reason = fmt.Sprintf("Nominal pembayaran tidak dilaporkan gateway; tagihan %s belum dapat diverifikasi", rupiah(order.TotalPriceIDR))
		}
		holdReasons = append(holdReasons, reason)
	}
	if order.RiskLevel == "REVIEW" {
		holdReasons = append(holdReasons, "Pemeriksaan keamanan: "+order.RiskReason)
	}
	if len(holdReasons) > 0 {
		reason := strings.Join(holdReasons, ". ")
		held, holdErr := s.orderRepository.HoldByInvoiceID(ctx, invoiceID, paidAmountIDR, reason)
		if holdErr != nil {
			return holdErr
		}
		_ = s.auditRepository.Write(ctx, order.OperatorID, "", "order_payment_held", "order", held.ID, reason)
		if amountMismatch {
			sentry.CaptureException(fmt.Errorf("OrderService.SettlePayment: %s (order %s)", reason, held.ID))
		}
		return nil
	}

	paid, err := s.orderRepository.MarkPaidByInvoiceID(ctx, invoiceID, paidAmountIDR)
	if err != nil {
		return err
	}
	product, err := s.productRepository.GetByID(ctx, paid.OperatorID, paid.ProductID)
	if err == nil {
		s.applyPaidSideEffects(ctx, product, paid)
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
		HeldReason: o.HeldReason, ReceiptNumber: o.ReceiptNumber,
		BuyerAgentId: o.BuyerAgentID, BuyerKind: o.BuyerKind, BuyerName: o.BuyerName,
		BasePriceIdr: o.BasePriceIDR, OperatorMarkupIdr: o.OperatorMarkupIDR,
		AgentMarkupIdr: o.AgentMarkupIDR, Destination: o.Destination,
		RiskLevel: o.RiskLevel, RiskReason: o.RiskReason,
	}
	if o.PaidAmountIDR != nil {
		result.PaidAmountIdr = *o.PaidAmountIDR
	}
	if o.PaidAt != nil {
		result.PaidAt = timestamppb.New(*o.PaidAt)
	}
	return result
}

// RefundOrder records that money is owed back to the buyer, credits the
// buyer's dedicated refund ledger, and reverses referral commission.
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

	// Refunding something the supplier already delivered is a straight loss:
	// we paid them and gave the money back. Refused rather than warned, and
	// deliberately not bypassable with a flag — if it was not really delivered,
	// the honest fix is to correct the fulfilment first, which leaves a record
	// of somebody deciding that. A flag would leave none.
	if s.fulfilmentRepository != nil {
		delivery, err := s.fulfilmentRepository.StatusFor(ctx, order.ID)
		if err != nil {
			return nil, serviceError("OrderService.RefundOrder", err)
		}
		if delivery == "DELIVERED" {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("produk sudah dikirim supplier dan tidak dapat direfund; jika sebenarnya tidak terkirim, perbaiki status pengirimannya lebih dulu"))
		}
		if delivery == "SENT" {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("pengiriman masih berjalan di supplier; tunggu hasilnya sebelum melakukan refund"))
		}
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

	if order.BuyerKind == string(BuyerAgent) {
		if order.BuyerAgentID == "" {
			return nil, serviceError("OrderService.RefundOrder", errors.New("pesanan agen tidak memiliki pemilik refund"))
		}
		if err := s.ledgerRepository.AppendAgentRefundBalanceTx(ctx, tx, repository.AgentRefundBalanceEntry{
			OperatorID: op.ID, AgentID: order.BuyerAgentID, AmountIDR: refundAmount, Kind: "REFUND",
			OrderID: order.ID, Note: refundNote(refund.Reason), CreatedByUserID: userID,
			IdempotencyKey: "refund-" + refund.ID,
		}); err != nil {
			return nil, serviceError("OrderService.RefundOrder", err)
		}
	} else {
		if err := s.ledgerRepository.AppendBalanceTx(ctx, tx, repository.BalanceEntry{
			OperatorID: op.ID, PilgrimID: order.PilgrimID, AmountIDR: refundAmount, Kind: "REFUND",
			OrderID: order.ID, Note: refundNote(refund.Reason), CreatedByUserID: userID,
			IdempotencyKey: "refund-" + refund.ID,
		}); err != nil {
			return nil, serviceError("OrderService.RefundOrder", err)
		}
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

	// A refunded order no longer holds value, so its share of the buyer's daily
	// limit goes back. After the commit for the same reason as the audit: the
	// money is already returned, and a counter must not be able to undo that.
	s.releaseDailyLimit(ctx, order.ID)

	// Audited after the commit: the refund is the fact, and failing to write
	// the log must not roll back money that has already been returned.
	_ = s.auditRepository.Write(ctx, op.ID, userID, "order_refunded", "order", order.ID,
		fmt.Sprintf("Refund penuh %s, komisi ditarik %s%s",
			rupiah(refundAmount), rupiah(commissionReversal), noteSuffix(refund.Reason)))

	return s.refundResponse(ctx, op.ID, order.ID, refund, true)
}

func (s *OrderService) refundResponse(ctx context.Context, operatorID, orderID string, refund *domain.OrderRefund, created bool) (*hajjv1.RefundOrderResponse, error) {
	order, err := s.orderRepository.Get(ctx, operatorID, orderID)
	if err != nil {
		return nil, serviceError("OrderService.RefundOrder", err)
	}
	var balance int64
	if order.BuyerKind == string(BuyerAgent) {
		balance, err = s.ledgerRepository.AgentRefundBalance(ctx, order.BuyerAgentID)
	} else {
		balance, err = s.ledgerRepository.PilgrimBalance(ctx, order.PilgrimID)
	}
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
	product, levels, route, err := s.productRepository.Pricing(ctx, op.ID, req.ProductId)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", apperror.ErrNotFound)
	}
	if !product.IsActive {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", apperror.ErrFailedPrecondition)
	}
	price, err := pricePilgrimOrder(product, levels, route, req.Quantity, pilgrim.AgentID)
	if err != nil {
		return nil, err
	}
	destination, err := digitalDestination(product, "", pilgrim.Phone)
	if err != nil {
		return nil, err
	}
	// Checked before the order exists, for the same reason as CreateOrder: an
	// order nobody can pay for should never have been created.
	if !s.xenditClient.Configured() {
		return nil, serviceError("OrderService.CreateOrderForPilgrim", fmt.Errorf("%w: %w", apperror.ErrFailedPrecondition, payment.ErrNotConfigured))
	}

	order, created, err := s.orderRepository.Create(ctx, repository.CreateOrderParams{
		OperatorID: op.ID, SeasonID: pilgrim.SeasonID, PilgrimID: pilgrim.ID,
		ProductID: req.ProductId, AgentID: pilgrim.AgentID, PlacedByAgentID: agent.ID,
		Quantity: req.Quantity, BuyerKind: string(BuyerPilgrim), UnitPriceIDR: price.UnitPriceIDR,
		BasePriceIDR: price.BasePriceIDR, OperatorMarkupIDR: price.OperatorMarkupIDR, AgentMarkupIDR: price.AgentMarkupIDR,
		TotalPriceIDR: price.TotalPriceIDR, PlatformAmountIDR: price.PlatformAmountIDR,
		OperatorAmountIDR: price.OperatorAmountIDR, AgentCommissionIDR: price.AgentCommissionIDR,
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey), Destination: destination, CheckoutChannel: "XENDIT",
		CountsTowardDailyLimit: domain.RoutingRequired(product.Category), SpendDate: jakartaToday(),
	})
	if err != nil {
		return nil, s.checkoutCreateError(ctx, "OrderService.CreateOrderForPilgrim", op.ID, userID, string(BuyerPilgrim), pilgrim.ID, err)
	}
	order.ProductName = product.Name
	order.PilgrimName = pilgrim.FullName
	if !created {
		return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: order.XenditInvoiceURL}, nil
	}
	s.recordCommission(ctx, order)

	invoice, err := s.xenditClient.CreateInvoice(ctx, payment.CreateInvoiceRequest{
		ExternalID:         order.ID,
		Amount:             price.TotalPriceIDR,
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
		fmt.Sprintf("%s x%d — %s untuk %s oleh agen %s",
			product.Name, req.Quantity, rupiah(price.TotalPriceIDR), pilgrim.FullName, agent.Name))
	return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: invoice.InvoiceURL}, nil
}

// ListMyPurchaseCatalogue quotes active digital products at the agent price.
// Travel packages and physical goods stay out until their buyer-specific
// fulfilment flows exist; letting an agent pay for an undeliverable item would
// turn a catalogue gap into real money owed.
func (s *OrderService) ListMyPurchaseCatalogue(ctx context.Context, orgID string, req *hajjv1.ListMyPurchaseCatalogueRequest) (*hajjv1.ListMyPurchaseCatalogueResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("OrderService.ListMyPurchaseCatalogue", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.ListMyPurchaseCatalogue", err)
	}
	products, levels, routes, err := s.productRepository.ListPricing(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("OrderService.ListMyPurchaseCatalogue", err)
	}
	result := &hajjv1.ListMyPurchaseCatalogueResponse{Products: make([]*hajjv1.PurchaseCatalogueProduct, 0, len(products))}
	for i, product := range products {
		if !product.IsActive || (product.Category != "ROAMING_DATA" && product.Category != "PPOB_CREDIT") {
			continue
		}
		// Anything that would refuse at checkout is left out of the catalogue
		// entirely — an unroutable product included here is one an agent can
		// select, pay attention to, and only then be told no.
		price, priceErr := priceAgentOrder(product, levels[i], routes[i], 1)
		if priceErr != nil {
			continue
		}
		item := &hajjv1.PurchaseCatalogueProduct{
			Id: product.ID, SeasonId: product.SeasonID, Name: product.Name,
			Category: product.Category, Description: product.Description,
			Code: product.Code, UnitPriceIdr: price.UnitPriceIDR,
		}
		if product.NominalIDR != nil {
			item.NominalIdr = *product.NominalIDR
		}
		result.Products = append(result.Products, item)
	}
	return result, nil
}

// CreateOrderForSelf charges the signed-in agent/Muttawwif at the agent price.
// No referral is looked up and no commission is recorded: the agent markup is
// not collected on this price, so paying any commission would create money
// that the buyer never paid.
func (s *OrderService) CreateOrderForSelf(ctx context.Context, orgID, userID string, req *hajjv1.CreateOrderForSelfRequest) (*hajjv1.CreateOrderResponse, error) {
	if req == nil || !isUUID(req.ProductId) || req.Quantity < 1 || req.Quantity > 20 ||
		strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError("OrderService.CreateOrderForSelf", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForSelf", err)
	}
	agent, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForSelf", err)
	}
	product, levels, route, err := s.productRepository.Pricing(ctx, op.ID, req.ProductId)
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForSelf", apperror.ErrNotFound)
	}
	if !product.IsActive || (product.Category != "ROAMING_DATA" && product.Category != "PPOB_CREDIT") {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("produk ini belum tersedia untuk pembelian mandiri agen"))
	}
	price, err := priceAgentOrder(product, levels, route, req.Quantity)
	if err != nil {
		return nil, err
	}
	// A platform-owned product carries no season — pulsa does not belong to
	// Umrah 2026 — but an order does: the operator's reports are scoped by
	// season, and a seasonless order would simply stop appearing in them. The
	// buyer is an agent, who has no season of their own, so the operator's
	// active one is where this purchase belongs.
	seasonID := product.SeasonID
	if seasonID == "" {
		seasonID, err = s.seasonRepository.GetActiveSeasonID(ctx, op.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("travel ini belum punya musim aktif; buat musim sebelum agen dapat bertransaksi"))
		}
	}

	destination, err := digitalDestination(product, req.Destination, agent.Phone)
	if err != nil {
		return nil, err
	}
	if !s.xenditClient.Configured() {
		return nil, serviceError("OrderService.CreateOrderForSelf", fmt.Errorf("%w: %w", apperror.ErrFailedPrecondition, payment.ErrNotConfigured))
	}

	order, created, err := s.orderRepository.Create(ctx, repository.CreateOrderParams{
		OperatorID: op.ID, SeasonID: seasonID, BuyerAgentID: agent.ID,
		BuyerKind: string(BuyerAgent), ProductID: product.ID, PlacedByAgentID: agent.ID,
		Quantity: req.Quantity, UnitPriceIDR: price.UnitPriceIDR,
		BasePriceIDR: price.BasePriceIDR, OperatorMarkupIDR: price.OperatorMarkupIDR,
		AgentMarkupIDR: price.AgentMarkupIDR, TotalPriceIDR: price.TotalPriceIDR,
		PlatformAmountIDR: price.PlatformAmountIDR, OperatorAmountIDR: price.OperatorAmountIDR,
		AgentCommissionIDR: 0, IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		Destination: destination, CheckoutChannel: "XENDIT",
		CountsTowardDailyLimit: domain.RoutingRequired(product.Category), SpendDate: jakartaToday(),
	})
	if err != nil {
		return nil, s.checkoutCreateError(ctx, "OrderService.CreateOrderForSelf", op.ID, userID, string(BuyerAgent), agent.ID, err)
	}
	order.ProductName = product.Name
	order.BuyerName = agent.Name
	if !created {
		return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: order.XenditInvoiceURL}, nil
	}

	invoice, err := s.xenditClient.CreateInvoice(ctx, payment.CreateInvoiceRequest{
		ExternalID: order.ID, Amount: price.TotalPriceIDR,
		Description:        fmt.Sprintf("%s — %s", product.Name, agent.Name),
		SuccessRedirectURL: s.appBaseURL + "/agent?order=success",
		FailureRedirectURL: s.appBaseURL + "/agent?order=failed",
	})
	if err != nil {
		return nil, serviceError("OrderService.CreateOrderForSelf", fmt.Errorf("create xendit invoice: %w", err))
	}
	if err := s.orderRepository.SetXenditInvoice(ctx, order.ID, invoice.ID, invoice.InvoiceURL); err != nil {
		return nil, serviceError("OrderService.CreateOrderForSelf", err)
	}
	order.XenditInvoiceID = invoice.ID
	order.XenditInvoiceURL = invoice.InvoiceURL
	_ = s.auditRepository.Write(ctx, op.ID, userID, "agent_self_order_created", "order", order.ID,
		fmt.Sprintf("%s x%d — %s ke %s oleh %s", product.Name, req.Quantity, rupiah(price.TotalPriceIDR), destination, agent.Name))
	return &hajjv1.CreateOrderResponse{Order: orderMessage(order), CheckoutUrl: invoice.InvoiceURL}, nil
}

func (s *OrderService) ListMyOrders(ctx context.Context, orgID, userID string, req *hajjv1.ListMyOrdersRequest) (*hajjv1.ListOrdersResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("OrderService.ListMyOrders", apperror.ErrValidation)
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.ListMyOrders", err)
	}
	agent, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("OrderService.ListMyOrders", err)
	}
	orders, count, err := s.orderRepository.ListForBuyerAgent(ctx, op.ID, req.SeasonId, agent.ID, req.Limit, req.Offset)
	if err != nil {
		return nil, serviceError("OrderService.ListMyOrders", err)
	}
	result := &hajjv1.ListOrdersResponse{Orders: make([]*hajjv1.Order, 0, len(orders)), TotalCount: count}
	for _, order := range orders {
		result.Orders = append(result.Orders, orderMessage(order))
	}
	return result, nil
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
	s.releaseDailyLimit(ctx, order.ID)
	return nil
}

// releaseDailyLimit gives back the headroom an order consumed once it stops
// holding value.
//
// Safe to call for any order and safe to call twice: the stamp on the order is
// the guard, and the release clears it in the same statement. That is why this
// can sit on every terminal path — three settlement routes and the
// reconciliation sweep — without any of them coordinating over who does it.
//
// Failure is logged, never returned. The order has already moved to a status
// it will not leave; refusing to finish that because a counter could not be
// decremented would leave the transaction in a worse state than the stale
// headroom does, and the headroom resets at midnight regardless.
func (s *OrderService) releaseDailyLimit(ctx context.Context, orderID string) {
	if err := s.orderRepository.ReleaseDigitalSpend(ctx, orderID); err != nil {
		sentry.CaptureException(fmt.Errorf("OrderService.releaseDailyLimit: %w", err))
	}
}

// ResolveHeldOrder closes out a transaction whose payment did not match the
// bill. Without it a hold is a dead end that needs database access to clear.
//
// Accepting is the operator attesting that the difference was settled some
// other way — cash at the counter, a top-up transfer, or a shortfall they
// chose to waive. Nothing outside this system confirms that, so it is always
// audit-logged with who decided it and the exact discrepancy they accepted.
//
// Rejecting abandons the transaction and reverses the commission that was
// recognised when the order was placed. The money itself goes back outside the
// system, the same as every other refund here.
func (s *OrderService) ResolveHeldOrder(ctx context.Context, orgID, userID string, req *hajjv1.ResolveHeldOrderRequest) (*hajjv1.Order, error) {
	if req == nil || !isUUID(req.OrderId) {
		return nil, serviceError("OrderService.ResolveHeldOrder", apperror.ErrValidation)
	}
	accept := req.Resolution == hajjv1.HeldOrderResolution_HELD_ORDER_RESOLUTION_ACCEPT
	if !accept && req.Resolution != hajjv1.HeldOrderResolution_HELD_ORDER_RESOLUTION_REJECT {
		return nil, serviceError("OrderService.ResolveHeldOrder", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.ResolveHeldOrder", err)
	}
	// Read before resolving, so the discrepancy can be named in the audit
	// entry — afterwards the held reason is history rather than current state.
	before, err := s.orderRepository.Get(ctx, op.ID, req.OrderId)
	if err != nil {
		return nil, serviceError("OrderService.ResolveHeldOrder", err)
	}
	if before.Status != "HELD" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("hanya transaksi berstatus perlu ditinjau yang dapat diselesaikan"))
	}

	resolved, err := s.orderRepository.ResolveHeld(ctx, op.ID, req.OrderId, accept)
	if err != nil {
		return nil, serviceError("OrderService.ResolveHeldOrder", err)
	}
	resolved.PilgrimName = before.PilgrimName
	resolved.ProductName = before.ProductName
	resolved.AgentName = before.AgentName

	if accept {
		product, productErr := s.productRepository.GetByID(ctx, op.ID, resolved.ProductID)
		if productErr == nil {
			s.applyPaidSideEffects(ctx, product, resolved)
		}
	} else {
		s.reverseCommission(ctx, resolved, "Komisi ditarik: transaksi ditolak setelah ditinjau")
		s.releaseDailyLimit(ctx, resolved.ID)
	}

	decision := "diterima"
	if !accept {
		decision = "ditolak"
	}
	paid := int64(0)
	if before.PaidAmountIDR != nil {
		paid = *before.PaidAmountIDR
	}
	_ = s.auditRepository.Write(ctx, op.ID, userID, "held_order_resolved", "order", resolved.ID,
		fmt.Sprintf("Transaksi tertahan %s — dibayar %s dari tagihan %s (%s)%s",
			decision, rupiah(paid), rupiah(before.TotalPriceIDR), before.HeldReason, noteSuffix(req.Note)))
	return orderMessage(resolved), nil
}

// AttachFulfilment wires the delivery side in after construction.
//
// A setter rather than another constructor argument: FulfilmentService needs
// repositories OrderService also uses, and threading it through the
// constructor would make the wiring in main.go order-dependent for no gain.
// Nil is a valid state — an operator selling only travel packages owes no
// supplier anything.
func (s *OrderService) AttachFulfilment(fulfilments *FulfilmentService, records *repository.FulfilmentRepository) {
	s.fulfilmentService = fulfilments
	s.fulfilmentRepository = records
}
