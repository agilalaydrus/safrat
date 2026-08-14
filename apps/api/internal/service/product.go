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

type ProductService struct {
	operatorRepository *repository.OperatorRepository
	productRepository  *repository.ProductRepository
}

func NewProductService(operators *repository.OperatorRepository, products *repository.ProductRepository) *ProductService {
	return &ProductService{operatorRepository: operators, productRepository: products}
}
func (s *ProductService) Create(ctx context.Context, orgID string, req *hajjv1.CreateProductRequest) (*hajjv1.Product, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.SeasonId) == "" || (req.Type != "HAJJ" && req.Type != "UMRAH") {
		return nil, serviceError("ProductService.Create", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProductService.Create", err)
	}
	product, err := s.productRepository.Create(ctx, op.ID, req.SeasonId, req.Name, req.Type, req.Description, req.PriceIdr, req.DurationDays, req.Inclusions)
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
	if req == nil || strings.TrimSpace(req.Name) == "" || (req.Type != "HAJJ" && req.Type != "UMRAH") {
		return nil, serviceError("ProductService.Update", apperror.ErrValidation)
	}
	if _, err := uuid.Parse(req.ProductId); err != nil {
		return nil, serviceError("ProductService.Update", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ProductService.Update", err)
	}
	product, err := s.productRepository.Update(ctx, op.ID, req.ProductId, req.Name, req.Type, req.Description, req.PriceIdr, req.DurationDays, req.Inclusions, req.IsActive)
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
	return &hajjv1.Product{Id: product.ID, OperatorId: product.OperatorID, SeasonId: product.SeasonID, Name: product.Name, Type: product.Type, PriceIdr: product.PriceIDR, DurationDays: product.DurationDays, Description: product.Description, Inclusions: product.Inclusions, IsActive: product.IsActive, CreatedAt: timestamppb.New(product.CreatedAt), UpdatedAt: timestamppb.New(product.UpdatedAt)}
}
