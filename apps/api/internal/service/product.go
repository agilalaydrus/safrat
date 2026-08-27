package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var productCategories = map[string]bool{"TRAVEL_PACKAGE": true, "EQUIPMENT": true, "ROAMING_DATA": true, "PPOB_CREDIT": true}

// Default margin split for products created before this feature existed,
// or via a request that omits margins entirely (all three zero) — the same
// 15/70/15 split the products.platform_margin_bps etc. column DEFAULTs use,
// kept in sync deliberately.
//
// Basis points: 1500 = 15.00%.
const (
	defaultPlatformMarginBps = 1500
	defaultOperatorMarginBps = 7000
	defaultAgentMarginBps    = 1500
	// marginDenominator is what basis points are out of. 10000 bps = 100%.
	marginDenominator = 10000
)

// resolveMargins applies the defaults above only when the caller sent all
// three as zero (an omitted proto double field decodes to 0, and no real
// product should ever intentionally configure every margin to zero — that
// would mean nobody, including the platform, earns anything). Explicit
// zero on just one or two fields (e.g. no agent commission on this
// product) is preserved as-is.
//
// An omitted proto int32 field decodes to 0, same as the double it replaced.
func resolveMargins(platform, operatorBps, agent int32) (int32, int32, int32, error) {
	if platform == 0 && operatorBps == 0 && agent == 0 {
		return defaultPlatformMarginBps, defaultOperatorMarginBps, defaultAgentMarginBps, nil
	}
	if platform < 0 || operatorBps < 0 || agent < 0 {
		return 0, 0, 0, apperror.ErrValidation
	}
	// An exact comparison, where the float version needed an epsilon to avoid
	// rejecting a split that summed to exactly 100%.
	if platform+operatorBps+agent > marginDenominator {
		return 0, 0, 0, apperror.ErrValidation
	}
	return platform, operatorBps, agent, nil
}

// validateProductTypeAndCategory enforces that `type` (HAJJ/UMRAH) only
// applies to TRAVEL_PACKAGE — other categories (equipment, roaming data,
// pulsa/PPOB) aren't Hajj- or Umrah-specific and must leave it blank.
func validateProductTypeAndCategory(category, productType string) bool {
	if !productCategories[category] {
		return false
	}
	if category == "TRAVEL_PACKAGE" {
		return productType == "HAJJ" || productType == "UMRAH"
	}
	return productType == ""
}

type ProductService struct {
	operatorRepository *repository.OperatorRepository
	productRepository  *repository.ProductRepository
}

func NewProductService(operators *repository.OperatorRepository, products *repository.ProductRepository) *ProductService {
	return &ProductService{operatorRepository: operators, productRepository: products}
}
func (s *ProductService) Create(ctx context.Context, orgID string, req *hajjv1.CreateProductRequest) (*hajjv1.Product, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.SeasonId) == "" || !validateProductTypeAndCategory(req.Category, req.Type) {
		return nil, serviceError("ProductService.Create", apperror.ErrValidation)
	}
	platformMargin, operatorMargin, agentMargin, err := resolveMargins(req.PlatformMarginBps, req.OperatorMarginBps, req.AgentMarginBps)
	if err != nil {
		return nil, serviceError("ProductService.Create", err)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProductService.Create", err)
	}
	product, err := s.productRepository.Create(ctx, op.ID, req.SeasonId, req.Name, productCode(req.Code, req.Name), req.Category, req.Type, req.Description, req.PriceIdr, optionalAmount(req.NominalIdr), req.DurationDays, req.Inclusions, platformMargin, operatorMargin, agentMargin, itineraryFromProto(req.ItineraryDays), req.HotelIds, req.DefaultKloterId)
	if err != nil {
		return nil, serviceError("ProductService.Create", err)
	}
	return productMessage(product), nil
}
func (s *ProductService) Get(ctx context.Context, orgID string, req *hajjv1.GetProductRequest) (*hajjv1.Product, error) {
	if req == nil {
		return nil, serviceError("ProductService.Get", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProductService.Get", err)
	}
	product, err := s.productRepository.GetByID(ctx, op.ID, req.ProductId)
	if err != nil {
		return nil, serviceError("ProductService.Get", err)
	}
	return productMessage(product), nil
}
func (s *ProductService) List(ctx context.Context, orgID string, req *hajjv1.ListProductsRequest) (*hajjv1.ListProductsResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("ProductService.List", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProductService.List", err)
	}
	products, err := s.productRepository.ListBySeasonID(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("ProductService.List", err)
	}
	result := &hajjv1.ListProductsResponse{Products: make([]*hajjv1.Product, 0, len(products))}
	for _, product := range products {
		result.Products = append(result.Products, productMessage(product))
	}
	return result, nil
}
func (s *ProductService) Update(ctx context.Context, orgID string, req *hajjv1.UpdateProductRequest) (*hajjv1.Product, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" || !validateProductTypeAndCategory(req.Category, req.Type) {
		return nil, serviceError("ProductService.Update", apperror.ErrValidation)
	}
	if _, err := uuid.Parse(req.ProductId); err != nil {
		return nil, serviceError("ProductService.Update", apperror.ErrValidation)
	}
	platformMargin, operatorMargin, agentMargin, err := resolveMargins(req.PlatformMarginBps, req.OperatorMarginBps, req.AgentMarginBps)
	if err != nil {
		return nil, serviceError("ProductService.Update", err)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProductService.Update", err)
	}
	product, err := s.productRepository.Update(ctx, op.ID, req.ProductId, req.Name, productCode(req.Code, req.Name), req.Category, req.Type, req.Description, req.PriceIdr, optionalAmount(req.NominalIdr), req.DurationDays, req.Inclusions, req.IsActive, platformMargin, operatorMargin, agentMargin, itineraryFromProto(req.ItineraryDays), req.HotelIds, req.DefaultKloterId)
	if err != nil {
		return nil, serviceError("ProductService.Update", err)
	}
	return productMessage(product), nil
}
func (s *ProductService) Delete(ctx context.Context, orgID string, req *hajjv1.DeleteProductRequest) (*hajjv1.DeleteProductResponse, error) {
	if req == nil {
		return nil, serviceError("ProductService.Delete", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProductService.Delete", err)
	}
	if err := s.productRepository.Delete(ctx, op.ID, req.ProductId); err != nil {
		return nil, serviceError("ProductService.Delete", err)
	}
	return &hajjv1.DeleteProductResponse{}, nil
}
func productMessage(product *domain.Product) *hajjv1.Product {
	msg := &hajjv1.Product{
		Id: product.ID, OperatorId: product.OperatorID, SeasonId: product.SeasonID, Name: product.Name, Category: product.Category, Type: product.Type,
		PriceIdr: product.PriceIDR, DurationDays: product.DurationDays, Description: product.Description, Inclusions: product.Inclusions, IsActive: product.IsActive,
		CreatedAt: timestamppb.New(product.CreatedAt), UpdatedAt: timestamppb.New(product.UpdatedAt),
		PlatformMarginBps: product.PlatformMarginBps, OperatorMarginBps: product.OperatorMarginBps, AgentMarginBps: product.AgentMarginBps,
		HotelIds: product.HotelIDs, DefaultKloterId: product.DefaultKloterID,
		Code: product.Code,
	}
	// Zero on the wire means "no face value", which is what a travel package
	// has — as distinct from a face value of nothing.
	if product.NominalIDR != nil {
		msg.NominalIdr = *product.NominalIDR
	}
	for _, d := range product.ItineraryDays {
		msg.ItineraryDays = append(msg.ItineraryDays, &hajjv1.ItineraryDay{
			DayNumber: d.DayNumber, Title: d.Title, City: d.City, Activities: d.Activities,
			MealBreakfast: d.MealBreakfast, MealLunch: d.MealLunch, MealDinner: d.MealDinner,
		})
	}
	return msg
}

func itineraryFromProto(days []*hajjv1.ItineraryDay) []domain.ItineraryDay {
	result := make([]domain.ItineraryDay, 0, len(days))
	for _, d := range days {
		result = append(result, domain.ItineraryDay{
			DayNumber: d.DayNumber, Title: d.Title, City: d.City, Activities: d.Activities,
			MealBreakfast: d.MealBreakfast, MealLunch: d.MealLunch, MealDinner: d.MealDinner,
		})
	}
	return result
}

// productCode falls back to a slug of the name when none is given.
//
// A code is what a person quotes on the phone, so an operator who does not
// think to invent one should still get something usable rather than a blank —
// and a blank would also slip past the uniqueness index, which only covers
// non-empty codes.
func productCode(code, name string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(code))
	if trimmed != "" {
		return trimmed
	}
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r >= 'a' && r <= 'z':
			return r - 32
		case r == ' ' || r == '-' || r == '_':
			return '-'
		default:
			return -1
		}
	}, strings.TrimSpace(name))
	slug = strings.Trim(slug, "-")
	if len(slug) > 32 {
		slug = strings.Trim(slug[:32], "-")
	}
	return slug
}

// optionalAmount keeps "not applicable" distinct from zero. A travel package
// has no face value; it does not have one worth nothing.
func optionalAmount(amount int64) *int64 {
	if amount <= 0 {
		return nil
	}
	return &amount
}
