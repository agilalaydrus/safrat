package service

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
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
	kycRepository          *repository.KYCRepository
	auditRepository        *repository.AuditRepository
}

func NewPlatformService(platform *repository.PlatformRepository, supplierCosts *repository.SupplierCostRepository, suppliers *repository.SupplierRepository, kyc *repository.KYCRepository, audit *repository.AuditRepository) *PlatformService {
	return &PlatformService{platformRepository: platform, supplierCostRepository: supplierCosts, supplierRepository: suppliers, kycRepository: kyc, auditRepository: audit}
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
	return message
}
