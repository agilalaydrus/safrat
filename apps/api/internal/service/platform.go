package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PlatformService is TawafiqHub's own administrative surface, sitting above
// every operator rather than inside one.
//
// Nothing here is scoped by tenant, which is the point and also the danger.
// Every method that reads or writes across tenants calls requirePlatformAdmin
// first — there is no interceptor doing it, so it must be visible at the top
// of each one.
type PlatformService struct {
	platformRepository     *repository.PlatformRepository
	supplierCostRepository *repository.SupplierCostRepository
	supplierRepository     *repository.SupplierRepository
	productRepository      *repository.ProductRepository
	subscriptionRepository *repository.SubscriptionRepository
	kycRepository          *repository.KYCRepository
	auditRepository        *repository.AuditRepository

	// Composed rather than reimplemented. Resolving a supplier failure has to
	// refund, and a second copy of the refund path for this caller is the last
	// thing that should ever exist twice.
	orderService      *OrderService
	fulfilmentService *FulfilmentService
}

// AttachFulfilment wires the two services this one needs to finish a review.
// Set after construction because the order service is built later in main and
// takes the fulfilment service itself — a constructor argument would make the
// cycle impossible to wire.
func (s *PlatformService) AttachFulfilment(orders *OrderService, fulfilments *FulfilmentService) {
	s.orderService = orders
	s.fulfilmentService = fulfilments
}

func NewPlatformService(platform *repository.PlatformRepository, supplierCosts *repository.SupplierCostRepository, suppliers *repository.SupplierRepository, products *repository.ProductRepository, subscriptions *repository.SubscriptionRepository, kyc *repository.KYCRepository, audit *repository.AuditRepository) *PlatformService {
	return &PlatformService{platformRepository: platform, supplierCostRepository: supplierCosts, supplierRepository: suppliers, productRepository: products, subscriptionRepository: subscriptions, kycRepository: kyc, auditRepository: audit}
}

// requirePlatformAdmin is the only thing standing between a signed-in operator
// user and every other tenant's data. It resolves the caller from their own
// session and answers from the database each time — platform access is the
// widest privilege here, and a revocation should bite on the next request.
func (s *PlatformService) requirePlatformAdmin(ctx context.Context) (string, error) {
	userID := middleware.UserIDFromCtx(ctx)
	if userID == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	access, err := s.platformRepository.PlatformAccessFor(ctx, userID)
	if err != nil {
		return "", serviceError("PlatformService", err)
	}
	if !access.Granted {
		// The same answer for "not an admin" as for "no such thing", so this
		// cannot be used to probe who holds platform access.
		return "", connect.NewError(connect.CodePermissionDenied, errors.New("akses admin platform diperlukan"))
	}
	// Being granted is not enough. This identity can read every tenant's data,
	// so it must not rest on a password alone — and without this check the
	// second factor would be optional for precisely the account where it
	// matters most.
	//
	// Distinguishable from a plain refusal on purpose: an admin who has not
	// enrolled needs to be told to enrol, not told they lack access.
	if !access.TwoFactorEnabled {
		return "", connect.NewError(connect.CodeFailedPrecondition,
			errors.New("aktifkan verifikasi dua langkah sebelum membuka panel admin"))
	}
	return userID, nil
}

// AmIPlatformAdmin answers about the caller and nobody else, so it is
// deliberately callable by any signed-in user: the web app needs to know
// whether to show the panel without first calling something it may not be
// allowed to call.
func (s *PlatformService) AmIPlatformAdmin(ctx context.Context) (*hajjv1.AmIPlatformAdminResponse, error) {
	userID := middleware.UserIDFromCtx(ctx)
	if userID == "" {
		return &hajjv1.AmIPlatformAdminResponse{IsPlatformAdmin: false}, nil
	}
	access, err := s.platformRepository.PlatformAccessFor(ctx, userID)
	if err != nil {
		return nil, serviceError("PlatformService.AmIPlatformAdmin", err)
	}
	// Reported separately so the panel can tell an admin who has not enrolled
	// to enrol, instead of showing them the same wall as somebody with no
	// access at all.
	return &hajjv1.AmIPlatformAdminResponse{
		IsPlatformAdmin: access.Granted, TwoFactorEnabled: access.TwoFactorEnabled,
	}, nil
}

func (s *PlatformService) ListOperators(ctx context.Context) (*hajjv1.ListOperatorsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	operators, err := s.platformRepository.ListOperators(ctx, 100)
	if err != nil {
		return nil, serviceError("PlatformService.ListOperators", err)
	}
	result := &hajjv1.ListOperatorsResponse{Operators: make([]*hajjv1.PlatformOperator, 0, len(operators))}
	for _, operator := range operators {
		message := &hajjv1.PlatformOperator{
			Id: operator.ID, Name: operator.Name, Slug: operator.Slug,
			Plan: operator.Plan, SubscriptionStatus: operator.SubscriptionStatus,
			PilgrimCount: operator.PilgrimCount, ProductCount: operator.ProductCount,
			HeldOrderCount: operator.HeldOrderCount, CreatedAt: timestamppb.New(operator.CreatedAt),
		}
		if operator.AccessUntil != nil {
			message.AccessUntil = timestamppb.New(*operator.AccessUntil)
		}
		result.Operators = append(result.Operators, message)
	}
	return result, nil
}

func (s *PlatformService) ListProductsNeedingCost(ctx context.Context, req *hajjv1.ListProductsNeedingCostRequest) (*hajjv1.ListProductsNeedingCostResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	includeCosted := req != nil && req.IncludeCosted
	products, err := s.platformRepository.ListProducts(ctx, includeCosted)
	if err != nil {
		return nil, serviceError("PlatformService.ListProductsNeedingCost", err)
	}
	result := &hajjv1.ListProductsNeedingCostResponse{Products: make([]*hajjv1.PlatformProduct, 0, len(products))}
	for _, product := range products {
		result.Products = append(result.Products, platformProductMessage(product))
	}
	return result, nil
}

func (s *PlatformService) SetProductSupplierCost(ctx context.Context, req *hajjv1.SetProductSupplierCostRequest) (*hajjv1.SetProductSupplierCostResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.ProductId) || req.SupplierCostIdr < 0 {
		return nil, serviceError("PlatformService.SetProductSupplierCost", apperror.ErrValidation)
	}
	product, err := s.platformRepository.GetProduct(ctx, req.ProductId)
	if err != nil {
		return nil, serviceError("PlatformService.SetProductSupplierCost", err)
	}
	if err := s.supplierCostRepository.SetManualCost(ctx, product.OperatorID, product.ID, req.SupplierCostIdr); err != nil {
		if errors.Is(err, apperror.ErrFailedPrecondition) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("harga modal produk ini sudah terbaca langsung dari supplier dan tidak dapat ditimpa manual"))
		}
		return nil, serviceError("PlatformService.SetProductSupplierCost", err)
	}
	updated, err := s.platformRepository.GetProduct(ctx, req.ProductId)
	if err != nil {
		return nil, serviceError("PlatformService.SetProductSupplierCost", err)
	}
	// Audited against the operator whose product it is, so the change appears
	// in that tenant's own trail rather than only in a platform-side log.
	_ = s.auditRepository.Write(ctx, product.OperatorID, userID, "supplier_cost_set", "product", product.ID,
		formatSupplierCostNote(product.SupplierCostIDR, req.SupplierCostIdr))
	return &hajjv1.SetProductSupplierCostResponse{Product: platformProductMessage(updated)}, nil
}

// SetProductBasePrice sets what TawafiqHub charges a travel for a product.
//
// Platform-only, and scoped by product id alone rather than by operator: this
// is the price the operator pays, and an operator able to edit it could price
// below what the platform is charging them. That is why it lives on this
// service and not on ProductService.
func (s *PlatformService) SetProductBasePrice(ctx context.Context, req *hajjv1.SetProductBasePriceRequest) (*hajjv1.SetProductBasePriceResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.ProductId) || req.BasePriceIdr < 0 {
		return nil, serviceError("PlatformService.SetProductBasePrice", apperror.ErrValidation)
	}
	product, err := s.platformRepository.GetProduct(ctx, req.ProductId)
	if err != nil {
		return nil, serviceError("PlatformService.SetProductBasePrice", err)
	}

	// The base must cover what the supplier charges, or every sale of this
	// product loses money at the platform level regardless of what any travel
	// adds on top. Refused here as well as at checkout, because a base saved
	// below cost would otherwise sit looking valid until the first sale.
	if product.SupplierCostIDR != nil && req.BasePriceIdr < *product.SupplierCostIDR {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("harga dasar %s di bawah harga modal supplier %s", rupiah(req.BasePriceIdr), rupiah(*product.SupplierCostIDR)))
	}

	if err := s.productRepository.SetBasePrice(ctx, req.ProductId, req.BasePriceIdr); err != nil {
		return nil, serviceError("PlatformService.SetProductBasePrice", err)
	}
	updated, err := s.platformRepository.GetProduct(ctx, req.ProductId)
	if err != nil {
		return nil, serviceError("PlatformService.SetProductBasePrice", err)
	}

	// Audited against the operator who pays it, so the travel can see their own
	// cost move rather than finding out from an invoice.
	_ = s.auditRepository.Write(ctx, product.OperatorID, userID, "base_price_set", "product", product.ID,
		formatBasePriceNote(product.BasePriceIDR, req.BasePriceIdr))
	return &hajjv1.SetProductBasePriceResponse{Product: platformProductMessage(updated)}, nil
}

// SavePlatformProduct creates or edits a product TawafiqHub supplies to every
// travel.
//
// Restricted to the digital categories on purpose. Travel packages and
// equipment are the operator's own business — the platform has no supplier
// relationship behind them and no business owning rows a travel should be
// editing itself.
func (s *PlatformService) SavePlatformProduct(ctx context.Context, req *hajjv1.SavePlatformProductRequest) (*hajjv1.SavePlatformProductResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("PlatformService.SavePlatformProduct", apperror.ErrValidation)
	}
	if !domain.RoutingRequired(req.Category) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("hanya produk digital yang dikelola platform; paket dan perlengkapan dibuat oleh travel sendiri"))
	}
	if strings.TrimSpace(req.ProductId) != "" && !isUUID(req.ProductId) {
		return nil, serviceError("PlatformService.SavePlatformProduct", apperror.ErrValidation)
	}

	nominal := req.NominalIdr
	base := req.BasePriceIdr
	saved, err := s.productRepository.SavePlatformProduct(ctx, req.ProductId, domain.Product{
		Name:        strings.TrimSpace(req.Name),
		Code:        strings.ToUpper(strings.TrimSpace(req.Code)),
		Category:    req.Category,
		Description: strings.TrimSpace(req.Description),
		NominalIDR:  &nominal,
		// Always set, including zero. A platform product with no base cannot be
		// sold at all, and leaving it unset here would create exactly the gap
		// the pricing screen then complains about.
		BasePriceIDR: &base,
		IsActive:     req.IsActive,
	})
	if err != nil {
		if errors.Is(err, apperror.ErrAlreadyExists) {
			return nil, connect.NewError(connect.CodeAlreadyExists,
				errors.New("kode produk sudah dipakai di katalog platform"))
		}
		return nil, serviceError("PlatformService.SavePlatformProduct", err)
	}

	// Audited with no operator, because this belongs to no tenant. That is the
	// case migration 108 opened audit_logs up for.
	_ = s.auditRepository.Write(ctx, "", userID, "platform_product_saved", "product", saved.ID,
		fmt.Sprintf("%s (%s) harga dasar %s", saved.Name, saved.Code, rupiah(base)))

	product, err := s.platformRepository.GetProduct(ctx, saved.ID)
	if err != nil {
		return nil, serviceError("PlatformService.SavePlatformProduct", err)
	}
	return &hajjv1.SavePlatformProductResponse{Product: platformProductMessage(product)}, nil
}

// ListPlatformCatalogue is the admin's view of what TawafiqHub supplies.
func (s *PlatformService) ListPlatformCatalogue(ctx context.Context, req *hajjv1.ListPlatformCatalogueRequest) (*hajjv1.ListPlatformCatalogueResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	products, err := s.productRepository.PlatformCatalogue(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.ListPlatformCatalogue", err)
	}
	out := make([]*hajjv1.PlatformProduct, 0, len(products))
	for _, product := range products {
		message := &hajjv1.PlatformProduct{
			Id: product.ID, Name: product.Name, Category: product.Category,
			// Named rather than left blank: this catalogue belongs to the
			// platform, and a blank operator column reads as missing data.
			OperatorName:       "TawafiqHub",
			PriceIdr:           product.PriceIDR,
			SupplierCostSource: product.SupplierCostSource,
			Code:               product.Code,
			Description:        product.Description,
			IsActive:           product.IsActive,
		}
		if product.NominalIDR != nil {
			message.NominalIdr = *product.NominalIDR
		}
		if product.SupplierCostIDR != nil {
			message.SupplierCostIdr = *product.SupplierCostIDR
		}
		if product.BasePriceIDR != nil {
			message.BasePriceIdr = *product.BasePriceIDR
			message.BasePriceSet = true
		}
		out = append(out, message)
	}
	return &hajjv1.ListPlatformCatalogueResponse{Products: out}, nil
}

// ResolveFulfilment closes out a delivery nothing could determine automatically.
//
// Resolving is deliberately repeatable: a wrong call has to be correctable, or
// an operator who marked something delivered by mistake would be trapped
// holding a jamaah's money with no lawful way to return it.
//
// So the thing that stops two clicks becoming two refunds is not this — it is
// the refund itself, which the database allows only once per order, reached
// here with a key derived from the order id rather than a random one. The
// fulfilment moves first anyway, so a failure between the two leaves the record
// saying "failed, unrefunded" rather than money gone with the matter still
// showing open.
func (s *PlatformService) ResolveFulfilment(ctx context.Context, req *hajjv1.ResolveFulfilmentRequest) (*hajjv1.ResolveFulfilmentResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OrderId) || strings.TrimSpace(req.Note) == "" {
		return nil, serviceError("PlatformService.ResolveFulfilment", apperror.ErrValidation)
	}
	if s.orderService == nil || s.fulfilmentService == nil {
		return nil, serviceError("PlatformService.ResolveFulfilment", errors.New("layanan fulfilment belum terpasang"))
	}

	// Read off the order, not taken from the caller. A platform admin acts
	// across tenants, so the tenant has to come from the thing being acted on.
	operatorID, err := s.platformRepository.OperatorForOrder(ctx, req.OrderId)
	if err != nil {
		return nil, serviceError("PlatformService.ResolveFulfilment", err)
	}

	if err := s.fulfilmentService.ResolveManually(ctx, req.OrderId, req.Status, userID, strings.TrimSpace(req.Note)); err != nil {
		return nil, err
	}

	_ = s.auditRepository.Write(ctx, operatorID, userID, "fulfilment_resolved", "order", req.OrderId,
		req.Status+": "+strings.TrimSpace(req.Note))

	response := &hajjv1.ResolveFulfilmentResponse{Status: req.Status}
	if req.Status != "FAILED" {
		return response, nil
	}

	// Failed means the jamaah paid and received nothing, so the money goes
	// back. The key is derived from the order rather than random: a retry after
	// a network error must settle the same refund, not open a second one.
	refund, err := s.orderService.RefundOrderForOperator(ctx, operatorID, userID, &hajjv1.RefundOrderRequest{
		OrderId:        req.OrderId,
		Reason:         "Pengiriman gagal setelah ditinjau: " + strings.TrimSpace(req.Note),
		IdempotencyKey: "fulfilment-failed-" + req.OrderId,
	})
	if err != nil {
		// The fulfilment is already FAILED and the refund is not done. Said
		// plainly rather than swallowed: somebody has to finish it by hand, and
		// a generic error would hide which half succeeded.
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("status sudah ditandai gagal tetapi refund belum berhasil; selesaikan refund order %s secara manual: %w", req.OrderId, err))
	}

	response.Refunded = true
	if refund.Refund != nil {
		response.RefundedIdr = refund.Refund.AmountIdr
	}
	return response, nil
}

// ListPendingTransfers shows what to look for in the bank statement.
func (s *PlatformService) ListPendingTransfers(ctx context.Context, _ *hajjv1.ListPendingTransfersRequest) (*hajjv1.ListPendingTransfersResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	transfers, err := s.subscriptionRepository.ListPendingTransfers(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.ListPendingTransfers", err)
	}
	out := make([]*hajjv1.PendingTransfer, 0, len(transfers))
	for _, t := range transfers {
		message := &hajjv1.PendingTransfer{
			InvoiceId: t.InvoiceID, OperatorName: t.OperatorName,
			Plan: t.Plan, AmountIdr: t.AmountIDR, IssuedAt: timestamppb.New(t.IssuedAt),
		}
		if t.ExpiresAt != nil {
			message.ExpiresAt = timestamppb.New(*t.ExpiresAt)
		}
		out = append(out, message)
	}
	return &hajjv1.ListPendingTransfersResponse{Transfers: out}, nil
}

// ConfirmBankTransfer settles the invoice whose amount arrived.
//
// Matched on the exact figure, which is the point of the unique suffix: an
// amount rounded off the statement matches nothing, and that is the correct
// answer rather than something to be lenient about. Crediting the wrong
// operator is far worse than asking somebody to re-read a number.
func (s *PlatformService) ConfirmBankTransfer(ctx context.Context, req *hajjv1.ConfirmBankTransferRequest) (*hajjv1.ConfirmBankTransferResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || req.AmountIdr <= 0 {
		return nil, serviceError("PlatformService.ConfirmBankTransfer", apperror.ErrValidation)
	}

	invoiceID, operatorID, operatorName, err := s.subscriptionRepository.FindPayableByAmount(ctx, req.AmountIdr)
	if errors.Is(err, apperror.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("tidak ada tagihan transfer yang menunggu senilai %s; periksa lagi nominalnya sampai rupiah terakhir", rupiah(req.AmountIdr)))
	}
	if err != nil {
		return nil, serviceError("PlatformService.ConfirmBankTransfer", err)
	}

	// MarkPaid settles the invoice and extends access in one transaction, so
	// money can never be recorded without the access it bought. Repeating it is
	// safe: the update is conditional on the invoice still being PENDING.
	if err := s.subscriptionRepository.MarkPaid(ctx, invoiceID); err != nil {
		return nil, serviceError("PlatformService.ConfirmBankTransfer", err)
	}

	_ = s.auditRepository.Write(ctx, operatorID, userID, "bank_transfer_confirmed", "subscription_invoice", invoiceID,
		"transfer masuk "+rupiah(req.AmountIdr))

	return &hajjv1.ConfirmBankTransferResponse{
		InvoiceId: invoiceID, OperatorName: operatorName, AmountIdr: req.AmountIdr,
	}, nil
}

func formatBasePriceNote(previous *int64, next int64) string {
	if previous == nil {
		return fmt.Sprintf("harga dasar ditetapkan %s", rupiah(next))
	}
	return fmt.Sprintf("harga dasar %s -> %s", rupiah(*previous), rupiah(next))
}

func formatSupplierCostNote(previous *int64, next int64) string {
	if previous == nil {
		return "Harga modal supplier ditetapkan " + rupiah(next)
	}
	return "Harga modal supplier diubah dari " + rupiah(*previous) + " ke " + rupiah(next)
}

func platformProductMessage(product *repository.PlatformProduct) *hajjv1.PlatformProduct {
	message := &hajjv1.PlatformProduct{
		Id: product.ID, OperatorId: product.OperatorID, OperatorName: product.OperatorName,
		SeasonName: product.SeasonName, Name: product.Name, Category: product.Category,
		PriceIdr: product.PriceIDR, SupplierCostSource: product.SupplierCostSource,
	}
	if product.SupplierCostIDR != nil {
		message.SupplierCostIdr = *product.SupplierCostIDR
	}
	if product.SupplierCostUpdatedAt != nil {
		message.SupplierCostUpdatedAt = timestamppb.New(*product.SupplierCostUpdatedAt)
	}
	if product.BasePriceIDR != nil {
		message.BasePriceIdr = *product.BasePriceIDR
		message.BasePriceSet = true
	}
	return message
}
