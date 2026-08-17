package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrderService struct {
	operatorRepository *repository.OperatorRepository
	pilgrimRepository  *repository.PilgrimRepository
	productRepository  *repository.ProductRepository
	orderRepository    *repository.OrderRepository
	xenditClient       *payment.Client
	// appBaseURL is where Xendit redirects the pilgrim's browser back to
	// after payment — CORS_ALLOWED_ORIGIN doubles as this app's canonical
	// web origin, so no separate env var.
	appBaseURL string
}

func NewOrderService(operators *repository.OperatorRepository, pilgrims *repository.PilgrimRepository, products *repository.ProductRepository, orders *repository.OrderRepository, xendit *payment.Client, appBaseURL string) *OrderService {
	return &OrderService{operatorRepository: operators, pilgrimRepository: pilgrims, productRepository: products, orderRepository: orders, xenditClient: xendit, appBaseURL: appBaseURL}
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

	totalPrice := product.PriceIDR * int64(req.Quantity)
	// Rounds down — a fraction of a rupiah has nowhere to go, and
	// under-crediting by a fraction is the safe direction for a split
	// that must sum to <= total, never over.
	platformAmount := int64(float64(totalPrice) * product.PlatformMarginPct)
	operatorAmount := int64(float64(totalPrice) * product.OperatorMarginPct)
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

func (s *OrderService) ListOrders(ctx context.Context, orgID string, req *hajjv1.ListOrdersRequest) (*hajjv1.ListOrdersResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("OrderService.ListOrders", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("OrderService.ListOrders", err)
	}
	orders, err := s.orderRepository.ListBySeason(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("OrderService.ListOrders", err)
	}
	result := &hajjv1.ListOrdersResponse{Orders: make([]*hajjv1.Order, 0, len(orders))}
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
