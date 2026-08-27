package repository

import (
	"context"
	"errors"

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

func (r *ProductRepository) Create(ctx context.Context, operatorID, seasonID, name, code, category, productType, description string, priceIDR int64, nominalIDR *int64, durationDays int32, inclusions []string, platformMarginBps, operatorMarginBps, agentMarginBps int32, itinerary []domain.ItineraryDay, hotelIDs []string, defaultKloterID string) (*domain.Product, error) {
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
	product, err := qtx.CreateProduct(ctx, db.CreateProductParams{OperatorID: opUUID, SeasonID: seasonUUID, Name: name, Code: code, NominalIdr: pgInt8Ptr(nominalIDR), Category: category, Type: productType, PriceIdr: priceIDR, DurationDays: durationDays, Description: description, Inclusions: nonNilStrings(inclusions), IsActive: true, PlatformMarginBps: platformMarginBps, OperatorMarginBps: operatorMarginBps, AgentMarginBps: agentMarginBps, DefaultKloterID: kloterUUID})
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

func (r *ProductRepository) Update(ctx context.Context, operatorID, productID, name, code, category, productType, description string, priceIDR int64, nominalIDR *int64, durationDays int32, inclusions []string, isActive bool, platformMarginBps, operatorMarginBps, agentMarginBps int32, itinerary []domain.ItineraryDay, hotelIDs []string, defaultKloterID string) (*domain.Product, error) {
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
	product, err := qtx.UpdateProduct(ctx, db.UpdateProductParams{ID: productUUID, OperatorID: opUUID, Name: name, Code: code, NominalIdr: pgInt8Ptr(nominalIDR), Category: category, Type: productType, PriceIdr: priceIDR, DurationDays: durationDays, Description: description, Inclusions: nonNilStrings(inclusions), IsActive: isActive, PlatformMarginBps: platformMarginBps, OperatorMarginBps: operatorMarginBps, AgentMarginBps: agentMarginBps, DefaultKloterID: kloterUUID})
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
	return &domain.Product{ID: uuid.UUID(product.ID.Bytes).String(), OperatorID: uuid.UUID(product.OperatorID.Bytes).String(), SeasonID: uuid.UUID(product.SeasonID.Bytes).String(), Name: product.Name, Category: product.Category, Type: product.Type, PriceIDR: product.PriceIdr, DurationDays: product.DurationDays, Description: product.Description, Inclusions: product.Inclusions, IsActive: product.IsActive, CreatedAt: product.CreatedAt.Time, UpdatedAt: product.UpdatedAt.Time, Code: product.Code, NominalIDR: int8Ptr(product.NominalIdr),
		SupplierCostIDR: int8Ptr(product.SupplierCostIdr), SupplierCostSource: product.SupplierCostSource,
		BasePriceIDR: int8Ptr(product.BasePriceIdr),
		PlatformMarginBps: product.PlatformMarginBps, OperatorMarginBps: product.OperatorMarginBps, AgentMarginBps: product.AgentMarginBps, DefaultKloterID: nullableUUIDString(product.DefaultKloterID)}
}

// Pricing reads a product together with this operator's markups, in one
// query. Sale pricing needs both, and two reads would let an operator's save
// land between them — pricing the order from one version and itemising it
// from another.
//
// A product that exists but has no markup row comes back with Configured
// false rather than as an error, so the caller can say "belum diatur" instead
// of "tidak ditemukan". Those send a person to two different screens.
func (r *ProductRepository) Pricing(ctx context.Context, operatorID, productID string) (*domain.Product, domain.PriceLevels, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, domain.PriceLevels{}, err
	}
	product, err := pgUUID(productID)
	if err != nil {
		return nil, domain.PriceLevels{}, err
	}

	row, err := r.queries.GetProductPricing(ctx, db.GetProductPricingParams{ID: product, OperatorID: operator})
	if err != nil {
		return nil, domain.PriceLevels{}, err
	}
	return toProduct(row.Product), toPriceLevels(row.Product, row.OperatorMarkupIdr, row.AgentMarkupIdr, row.MarkupConfigured), nil
}

// SetMarkup writes an operator's markups for one product.
//
// Upsert rather than read-then-write: two staff saving the pricing screen at
// once would otherwise both see no row and both insert, leaving the product
// with two markups and no way to say which applies.
func (r *ProductRepository) SetMarkup(ctx context.Context, operatorID, productID string, operatorMarkupIDR, agentMarkupIDR int64) error {
	if operatorMarkupIDR < 0 || agentMarkupIDR < 0 {
		return errors.New("markup tidak boleh negatif")
	}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	product, err := pgUUID(productID)
	if err != nil {
		return err
	}

	_, err = r.queries.UpsertProductMarkup(ctx, db.UpsertProductMarkupParams{
		ProductID:         product,
		OperatorID:        operator,
		OperatorMarkupIdr: operatorMarkupIDR,
		AgentMarkupIdr:    agentMarkupIDR,
	})
	return err
}

// SetBasePrice is the platform's own lever and is scoped by product id alone —
// deliberately not by operator, because the operator is not allowed to move
// the price they are charged. Callers must be platform admins; that check
// belongs in the service, not here.
func (r *ProductRepository) SetBasePrice(ctx context.Context, productID string, baseIDR int64) error {
	if baseIDR < 0 {
		return errors.New("harga dasar tidak boleh negatif")
	}
	product, err := pgUUID(productID)
	if err != nil {
		return err
	}
	_, err = r.queries.SetProductBasePrice(ctx, db.SetProductBasePriceParams{
		ID:           product,
		BasePriceIdr: pgtype.Int8{Int64: baseIDR, Valid: true},
	})
	return err
}

// ListPricing is the operator's pricing screen: every product in a season with
// whatever markup it carries, including the ones carrying none.
func (r *ProductRepository) ListPricing(ctx context.Context, operatorID, seasonID string) ([]*domain.Product, []domain.PriceLevels, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, nil, err
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, nil, err
	}

	rows, err := r.queries.ListProductMarkups(ctx, db.ListProductMarkupsParams{OperatorID: operator, SeasonID: season})
	if err != nil {
		return nil, nil, err
	}

	products := make([]*domain.Product, 0, len(rows))
	levels := make([]domain.PriceLevels, 0, len(rows))
	for _, row := range rows {
		products = append(products, toProduct(row.Product))
		levels = append(levels, toPriceLevels(row.Product, row.OperatorMarkupIdr, row.AgentMarkupIdr, row.MarkupConfigured))
	}
	return products, levels, nil
}

func toPriceLevels(product db.Product, operatorMarkup, agentMarkup pgtype.Int8, configured bool) domain.PriceLevels {
	return domain.PriceLevels{
		BasePriceIDR:      int8Ptr(product.BasePriceIdr),
		OperatorMarkupIDR: operatorMarkup.Int64,
		AgentMarkupIDR:    agentMarkup.Int64,
		Configured:        configured,
	}
}
