package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepository struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewProductRepository(queries *db.Queries, pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{queries: queries, pool: pool}
}

func (r *ProductRepository) Create(ctx context.Context, operatorID, seasonID, name, category, productType, description string, priceIDR int64, durationDays int32, inclusions []string, platformMarginPct, operatorMarginPct, agentMarginPct float64, itinerary []domain.ItineraryDay, hotelIDs []string, defaultKloterID string) (*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := optionalUUID(defaultKloterID)
	if err != nil {
		return nil, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := r.queries.WithTx(tx)
	product, err := qtx.CreateProduct(ctx, db.CreateProductParams{OperatorID: opUUID, SeasonID: seasonUUID, Name: name, Category: category, Type: productType, PriceIdr: priceIDR, DurationDays: durationDays, Description: description, Inclusions: nonNilStrings(inclusions), IsActive: true, PlatformMarginPct: platformMarginPct, OperatorMarginPct: operatorMarginPct, AgentMarginPct: agentMarginPct, DefaultKloterID: kloterUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	if err := r.writeExtras(ctx, qtx, opUUID, product.ID, itinerary, hotelIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	result := toProduct(product)
	return r.loadExtras(ctx, result)
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
	return r.loadExtras(ctx, toProduct(product))
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
		p, err := r.loadExtras(ctx, toProduct(product))
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

func (r *ProductRepository) Update(ctx context.Context, operatorID, productID, name, category, productType, description string, priceIDR int64, durationDays int32, inclusions []string, isActive bool, platformMarginPct, operatorMarginPct, agentMarginPct float64, itinerary []domain.ItineraryDay, hotelIDs []string, defaultKloterID string) (*domain.Product, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	productUUID, err := pgUUID(productID)
	if err != nil {
		return nil, err
	}
	kloterUUID, err := optionalUUID(defaultKloterID)
	if err != nil {
		return nil, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := r.queries.WithTx(tx)
	product, err := qtx.UpdateProduct(ctx, db.UpdateProductParams{ID: productUUID, OperatorID: opUUID, Name: name, Category: category, Type: productType, PriceIdr: priceIDR, DurationDays: durationDays, Description: description, Inclusions: nonNilStrings(inclusions), IsActive: isActive, PlatformMarginPct: platformMarginPct, OperatorMarginPct: operatorMarginPct, AgentMarginPct: agentMarginPct, DefaultKloterID: kloterUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	if err := qtx.ReplaceProductItineraryDays(ctx, productUUID); err != nil {
		return nil, err
	}
	if err := qtx.ReplaceProductHotels(ctx, productUUID); err != nil {
		return nil, err
	}
	if err := r.writeExtras(ctx, qtx, opUUID, productUUID, itinerary, hotelIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.loadExtras(ctx, toProduct(product))
}

// writeExtras inserts itinerary days and hotel links for a product that
// already has no rows for either (a fresh Create, or right after
// Update's Replace* deletes) — never called against a product that might
// already have rows, to avoid unique-constraint churn.
func (r *ProductRepository) writeExtras(ctx context.Context, qtx *db.Queries, operatorID, productID pgtype.UUID, itinerary []domain.ItineraryDay, hotelIDs []string) error {
	for _, day := range itinerary {
		if err := qtx.InsertProductItineraryDay(ctx, db.InsertProductItineraryDayParams{
			OperatorID: operatorID, ProductID: productID, DayNumber: day.DayNumber, Title: day.Title, City: day.City,
			Activities: day.Activities, MealBreakfast: day.MealBreakfast, MealLunch: day.MealLunch, MealDinner: day.MealDinner,
		}); err != nil {
			return databaseError(err)
		}
	}
	for _, hotelID := range hotelIDs {
		hotelUUID, err := pgUUID(hotelID)
		if err != nil {
			continue
		}
		if err := qtx.InsertProductHotel(ctx, db.InsertProductHotelParams{OperatorID: operatorID, ProductID: productID, HotelID: hotelUUID}); err != nil {
			return databaseError(err)
		}
	}
	return nil
}

func (r *ProductRepository) loadExtras(ctx context.Context, product *domain.Product) (*domain.Product, error) {
	productUUID, err := pgUUID(product.ID)
	if err != nil {
		return nil, err
	}
	days, err := r.queries.ListProductItineraryDays(ctx, productUUID)
	if err != nil {
		return nil, err
	}
	for _, d := range days {
		product.ItineraryDays = append(product.ItineraryDays, domain.ItineraryDay{
			DayNumber: d.DayNumber, Title: d.Title, City: d.City, Activities: d.Activities,
			MealBreakfast: d.MealBreakfast, MealLunch: d.MealLunch, MealDinner: d.MealDinner,
		})
	}
	hotels, err := r.queries.ListProductHotels(ctx, productUUID)
	if err != nil {
		return nil, err
	}
	for _, h := range hotels {
		product.HotelIDs = append(product.HotelIDs, uuidString(h.HotelID))
		product.HotelNames = append(product.HotelNames, h.HotelName+" ("+h.HotelCity+")")
	}
	return product, nil
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

// optionalUUID treats an empty string as NULL — used for default_kloter_id,
// which is genuinely optional (most TRAVEL_PACKAGE products won't set one).
func optionalUUID(value string) (pgtype.UUID, error) {
	if value == "" {
		return pgtype.UUID{}, nil
	}
	return pgUUID(value)
}

func toProduct(product db.Product) *domain.Product {
	return &domain.Product{ID: uuid.UUID(product.ID.Bytes).String(), OperatorID: uuid.UUID(product.OperatorID.Bytes).String(), SeasonID: uuid.UUID(product.SeasonID.Bytes).String(), Name: product.Name, Category: product.Category, Type: product.Type, PriceIDR: product.PriceIdr, DurationDays: product.DurationDays, Description: product.Description, Inclusions: product.Inclusions, IsActive: product.IsActive, CreatedAt: product.CreatedAt.Time, UpdatedAt: product.UpdatedAt.Time, PlatformMarginPct: product.PlatformMarginPct, OperatorMarginPct: product.OperatorMarginPct, AgentMarginPct: product.AgentMarginPct, DefaultKloterID: nullableUUIDString(product.DefaultKloterID)}
}
