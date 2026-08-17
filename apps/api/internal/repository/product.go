package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

type ProductRepository struct{ queries *db.Queries }

func NewProductRepository(queries *db.Queries) *ProductRepository {
	return &ProductRepository{queries: queries}
}

func (r *ProductRepository) Create(ctx context.Context, operatorID, seasonID, name, category, productType, description string, priceIDR int64, durationDays int32, inclusions []string, platformMarginPct, operatorMarginPct, agentMarginPct float64) (*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	product, err := r.queries.CreateProduct(ctx, db.CreateProductParams{OperatorID: opUUID, SeasonID: seasonUUID, Name: name, Category: category, Type: productType, PriceIdr: priceIDR, DurationDays: durationDays, Description: description, Inclusions: nonNilStrings(inclusions), IsActive: true, PlatformMarginPct: platformMarginPct, OperatorMarginPct: operatorMarginPct, AgentMarginPct: agentMarginPct})
	if err != nil {
		return nil, databaseError(err)
	}
	return toProduct(product), nil
}

func (r *ProductRepository) GetByID(ctx context.Context, operatorID, productID string) (*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	productUUID, err := pgUUID(productID)
	if err != nil {
		return nil, err
	}
	product, err := r.queries.GetProduct(ctx, db.GetProductParams{ID: productUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	return toProduct(product), nil
}

func (r *ProductRepository) ListBySeasonID(ctx context.Context, operatorID, seasonID string) ([]*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	products, err := r.queries.ListProducts(ctx, db.ListProductsParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Product, 0, len(products))
	for _, product := range products {
		result = append(result, toProduct(product))
	}
	return result, nil
}

func (r *ProductRepository) Update(ctx context.Context, operatorID, productID, name, category, productType, description string, priceIDR int64, durationDays int32, inclusions []string, isActive bool, platformMarginPct, operatorMarginPct, agentMarginPct float64) (*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	productUUID, err := pgUUID(productID)
	if err != nil {
		return nil, err
	}
	product, err := r.queries.UpdateProduct(ctx, db.UpdateProductParams{ID: productUUID, OperatorID: opUUID, Name: name, Category: category, Type: productType, PriceIdr: priceIDR, DurationDays: durationDays, Description: description, Inclusions: nonNilStrings(inclusions), IsActive: isActive, PlatformMarginPct: platformMarginPct, OperatorMarginPct: operatorMarginPct, AgentMarginPct: agentMarginPct})
	if err != nil {
		return nil, databaseError(err)
	}
	return toProduct(product), nil
}

func (r *ProductRepository) Delete(ctx context.Context, operatorID, productID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	productUUID, err := pgUUID(productID)
	if err != nil {
		return err
	}
	return r.queries.DeleteProduct(ctx, db.DeleteProductParams{ID: productUUID, OperatorID: opUUID})
}

func toProduct(product db.Product) *domain.Product {
	return &domain.Product{ID: uuid.UUID(product.ID.Bytes).String(), OperatorID: uuid.UUID(product.OperatorID.Bytes).String(), SeasonID: uuid.UUID(product.SeasonID.Bytes).String(), Name: product.Name, Category: product.Category, Type: product.Type, PriceIDR: product.PriceIdr, DurationDays: product.DurationDays, Description: product.Description, Inclusions: product.Inclusions, IsActive: product.IsActive, CreatedAt: product.CreatedAt.Time, UpdatedAt: product.UpdatedAt.Time, PlatformMarginPct: product.PlatformMarginPct, OperatorMarginPct: product.OperatorMarginPct, AgentMarginPct: product.AgentMarginPct}
}
