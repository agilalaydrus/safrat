package service

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
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
	// Digital products are supplied by TawafiqHub — only the platform holds the
	// supplier routes and credentials. A travel creating its own PPOB row would
	// be selling something nothing can deliver, and the checkout gate would
	// refuse it only once a customer tried to buy. Refused here with an
	// explanation instead of letting the database raise a constraint error.
	if domain.RoutingRequired(req.Category) {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("produk digital dikelola oleh TawafiqHub dan tidak dibuat sendiri oleh travel; yang Anda atur adalah markup-nya"))
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
// refuseIfPlatformOwned turns an attempted tenant write on a platform product
// into an explanation.
//
// The strict operator predicate on the UPDATE already makes the write
// impossible — this exists so the caller is told why rather than receiving a
// bare not-found, which reads as "this product does not exist" when in fact it
// exists and belongs to somebody else.
func (s *ProductService) refuseIfPlatformOwned(ctx context.Context, operatorID, productID, method string) error {
	product, err := s.productRepository.GetByID(ctx, operatorID, productID)
	if err != nil {
		return serviceError(method, err)
	}
	if product.IsPlatformOwned() {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("produk ini dikelola TawafiqHub; travel hanya dapat mengatur markup-nya"))
	}
	return nil
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
	if err := s.refuseIfPlatformOwned(ctx, op.ID, req.ProductId, "ProductService.Update"); err != nil {
		return nil, err
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
	if err := s.refuseIfPlatformOwned(ctx, op.ID, req.ProductId, "ProductService.Delete"); err != nil {
		return nil, err
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

// ListPricing shows what each kind of buyer pays for every product in a
// season, and why.
//
// Prices are computed here on every read rather than stored. A stored price is
// a copy of a derivation, and it goes stale the moment any level beneath it
// moves — the platform raising a base, or the operator saving a markup. Then
// two numbers disagree and nothing can say which one a customer is owed.
func (s *ProductService) ListPricing(ctx context.Context, orgID string, req *hajjv1.ListProductPricingRequest) (*hajjv1.ListProductPricingResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("ProductService.ListPricing", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProductService.ListPricing", err)
	}

	products, levels, routes, err := s.productRepository.ListPricing(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("ProductService.ListPricing", err)
	}

	out := make([]*hajjv1.ProductPricing, 0, len(products))
	for i, product := range products {
		out = append(out, pricingMessage(product, levels[i], routes[i]))
	}
	return &hajjv1.ListProductPricingResponse{Pricing: out}, nil
}

// SetMarkup saves this travel's markups for one product.
//
// Scoped through the operator resolved from the session, never from the
// request, so an operator cannot price another tenant's product by sending its
// id. Repository writes are an upsert, so two staff saving at once cannot
// leave the product carrying two markups.
func (s *ProductService) SetMarkup(ctx context.Context, orgID string, req *hajjv1.SetProductMarkupRequest) (*hajjv1.SetProductMarkupResponse, error) {
	if req == nil || strings.TrimSpace(req.ProductId) == "" ||
		req.OperatorMarkupIdr < 0 || req.AgentMarkupIdr < 0 {
		return nil, serviceError("ProductService.SetMarkup", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProductService.SetMarkup", err)
	}

	// Read first so a product belonging to another operator is a not-found
	// rather than a silent no-op write. The upsert is scoped by operator id
	// too, so nothing could be written across tenants either way — but a
	// caller must be told, not left believing a save happened.
	if _, _, _, err := s.productRepository.Pricing(ctx, op.ID, req.ProductId); err != nil {
		return nil, serviceError("ProductService.SetMarkup", err)
	}

	if err := s.productRepository.SetMarkup(ctx, op.ID, req.ProductId, req.OperatorMarkupIdr, req.AgentMarkupIdr); err != nil {
		return nil, serviceError("ProductService.SetMarkup", err)
	}

	// Re-read rather than echoing the request back. The response carries
	// computed prices, and computing them from what was just sent would show
	// the caller their own input dressed as a result — including when the base
	// is unset and the product still cannot be sold.
	product, levels, route, err := s.productRepository.Pricing(ctx, op.ID, req.ProductId)
	if err != nil {
		return nil, serviceError("ProductService.SetMarkup", err)
	}
	return &hajjv1.SetProductMarkupResponse{Pricing: pricingMessage(product, levels, route)}, nil
}

// pricingMessage builds one row of the pricing screen.
//
// Sellability is decided by running the real pricing gate, not by re-listing
// its conditions here. If the screen judged sellability on its own terms it
// would drift from checkout, and the failure mode is the worst kind: a product
// the screen calls ready that refuses at the moment a customer tries to pay.
func pricingMessage(product *domain.Product, levels domain.PriceLevels, route domain.RouteReadiness) *hajjv1.ProductPricing {
	msg := &hajjv1.ProductPricing{
		ProductId:         product.ID,
		ProductName:       product.Name,
		Code:              product.Code,
		Category:          product.Category,
		OperatorMarkupIdr: levels.OperatorMarkupIDR,
		AgentMarkupIdr:    levels.AgentMarkupIDR,
		MarkupConfigured:  levels.Configured,
		BasePriceSet:      levels.BasePriceIDR != nil,
	}
	if levels.BasePriceIDR != nil {
		msg.BasePriceIdr = *levels.BasePriceIDR
	}

	// Quantity one: this is a unit price list. Every level scales linearly, so
	// a unit price is the whole truth here.
	pilgrimPrice, pilgrimErr := pricePilgrimOrder(product, levels, route, 1, "")
	agentPrice, agentErr := priceAgentOrder(product, levels, route, 1)

	if pilgrimErr != nil {
		msg.UnsellableReason = refusalText(pilgrimErr)
		return msg
	}
	if agentErr != nil {
		msg.UnsellableReason = refusalText(agentErr)
		return msg
	}

	msg.PilgrimPriceIdr = pilgrimPrice.TotalPriceIDR
	msg.AgentPriceIdr = agentPrice.TotalPriceIDR
	msg.Sellable = true
	return msg
}

// refusalText unwraps a Connect error down to the message a person should
// read. The pricing gate returns failed preconditions carrying text written
// for exactly this purpose; anything else would be an internal fault and must
// not be shown verbatim.
func refusalText(err error) string {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeFailedPrecondition {
		return connectErr.Message()
	}
	return "produk belum siap dijual"
}
