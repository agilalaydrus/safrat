-- name: CreateProduct :one
INSERT INTO products (operator_id, season_id, name, code, nominal_idr, category, type, price_idr, duration_days, description, inclusions, is_active, platform_margin_bps, operator_margin_bps, agent_margin_bps, default_kloter_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: GetProduct :one
-- Strict: this product belongs to this operator. Used wherever the caller is
-- about to change something, so a platform product can never be reached by a
-- tenant write.
SELECT * FROM products
WHERE id = $1 AND operator_id = $2;

-- name: GetSellableProduct :one
-- Widened: a travel sells its own catalogue plus the platform's. Separate from
-- GetProduct rather than a flag on it, because "may I read this" and "may I
-- change this" are different questions and a single query answering both is
-- how the wrong one gets used.
SELECT * FROM products
WHERE id = $1 AND (operator_id = $2 OR operator_id IS NULL);

-- name: ListProducts :many
-- The operator's catalogue: their own season's products, plus everything the
-- platform supplies. Platform products carry no season — pulsa does not belong
-- to Umrah 2026 — so they are matched on ownership alone.
SELECT * FROM products
WHERE (operator_id = $1 AND season_id = $2) OR operator_id IS NULL
ORDER BY operator_id IS NULL, created_at DESC;

-- name: UpdateProduct :one
UPDATE products
SET name = $3, category = $4, type = $5, price_idr = $6, duration_days = $7,
    description = $8, inclusions = $9, is_active = $10,
    platform_margin_bps = $11, operator_margin_bps = $12, agent_margin_bps = $13,
    default_kloter_id = $14, code = $15, nominal_idr = $16, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products
WHERE id = $1 AND operator_id = $2;

-- name: ReplaceProductItineraryDays :exec
DELETE FROM product_itinerary_days WHERE product_id = $1;

-- name: InsertProductItineraryDay :exec
INSERT INTO product_itinerary_days (operator_id, product_id, day_number, title, city, activities, meal_breakfast, meal_lunch, meal_dinner)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListProductItineraryDays :many
SELECT * FROM product_itinerary_days WHERE product_id = $1 ORDER BY day_number ASC;

-- name: ReplaceProductHotels :exec
DELETE FROM product_hotels WHERE product_id = $1;

-- name: InsertProductHotel :exec
INSERT INTO product_hotels (operator_id, product_id, hotel_id)
VALUES ($1, $2, $3)
ON CONFLICT (product_id, hotel_id) DO NOTHING;

-- name: ListProductHotels :many
SELECT ph.hotel_id, h.name AS hotel_name, h.city AS hotel_city
FROM product_hotels ph
JOIN hotels h ON h.id = ph.hotel_id
WHERE ph.product_id = $1
ORDER BY h.city ASC, h.name ASC;

-- name: GetProductPricing :one
-- Product and its markup in one read, because a sale prices from both and
-- fetching them separately leaves a window where the operator saves new
-- markups between the two queries — the order would then be priced from one
-- version and itemised from another.
--
-- LEFT JOIN so an unconfigured product still returns a row: the caller must be
-- able to tell "no markup configured" from "product does not exist", and those
-- carry different errors for the person on the screen.
SELECT
  sqlc.embed(p),
  m.operator_markup_idr,
  m.agent_markup_idr,
  (m.id IS NOT NULL)::boolean AS markup_configured,
  -- Routing state travels with the price because a digital product with no
  -- working route cannot be sold at any price. Read here rather than in a
  -- second query so the pricing screen and checkout judge the same row.
  (r.id IS NOT NULL)::boolean AS route_exists,
  COALESCE(r.is_active, false)::boolean AS route_active,
  COALESCE(sup.status, '')::text AS supplier_status
FROM products p
LEFT JOIN product_markups m
  ON m.product_id = p.id AND m.operator_id = $2
LEFT JOIN product_routes r ON r.product_id = p.id
LEFT JOIN suppliers sup ON sup.id = r.supplier_id
WHERE p.id = $1 AND (p.operator_id = $2 OR p.operator_id IS NULL);

-- name: UpsertProductMarkup :one
INSERT INTO product_markups (product_id, operator_id, operator_markup_idr, agent_markup_idr)
VALUES ($1, $2, $3, $4)
ON CONFLICT (product_id, operator_id) DO UPDATE
SET operator_markup_idr = EXCLUDED.operator_markup_idr,
    agent_markup_idr    = EXCLUDED.agent_markup_idr,
    updated_at          = NOW()
RETURNING *;

-- name: ListProductMarkups :many
SELECT sqlc.embed(p), m.operator_markup_idr, m.agent_markup_idr,
  (m.id IS NOT NULL)::boolean AS markup_configured,
  (r.id IS NOT NULL)::boolean AS route_exists,
  COALESCE(r.is_active, false)::boolean AS route_active,
  COALESCE(sup.status, '')::text AS supplier_status
FROM products p
LEFT JOIN product_markups m ON m.product_id = p.id AND m.operator_id = $1
LEFT JOIN product_routes r ON r.product_id = p.id
LEFT JOIN suppliers sup ON sup.id = r.supplier_id
WHERE (p.operator_id = $1 AND p.season_id = $2) OR p.operator_id IS NULL
ORDER BY p.operator_id IS NULL, p.created_at DESC;

-- name: SetProductBasePrice :one
-- The base is TawafiqHub's number, not the travel's, so this is deliberately
-- not part of UpdateProduct — an operator editing their catalogue must not be
-- able to move the price they pay.
UPDATE products
SET base_price_idr = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreatePlatformProduct :one
-- operator_id and season_id are left NULL: this belongs to TawafiqHub, not to
-- any travel or season. The ownership CHECK requires both to be NULL together,
-- so a half-owned row cannot be created here by accident.
INSERT INTO products (
  operator_id, season_id, name, code, category, type, description,
  nominal_idr, base_price_idr, price_idr, is_active
) VALUES (
  NULL, NULL, sqlc.arg(name), sqlc.arg(code), sqlc.arg(category), '',
  sqlc.arg(description), sqlc.narg(nominal_idr), sqlc.arg(base_price_idr),
  -- price_idr is the legacy sell price, kept in step with the base so nothing
  -- reading the old column sees zero. Nothing prices from it any more.
  sqlc.arg(base_price_idr), sqlc.arg(is_active)
)
RETURNING *;

-- name: UpdatePlatformProduct :one
-- The operator_id IS NULL predicate is the guard: this statement can only ever
-- touch the platform catalogue, so a product id belonging to a travel cannot
-- be edited through the admin panel by mistake.
UPDATE products
SET name = sqlc.arg(name), code = sqlc.arg(code), category = sqlc.arg(category),
    description = sqlc.arg(description), nominal_idr = sqlc.narg(nominal_idr),
    base_price_idr = sqlc.arg(base_price_idr), price_idr = sqlc.arg(base_price_idr),
    is_active = sqlc.arg(is_active), updated_at = NOW()
WHERE id = sqlc.arg(id) AND operator_id IS NULL
RETURNING *;

-- name: ListPlatformCatalogue :many
-- The platform's own catalogue: no operator, no season. Ordered so the rows
-- that cannot be sold yet surface first — a base price or supplier cost that
-- is still missing is the thing somebody has to act on.
SELECT * FROM products
WHERE operator_id IS NULL
ORDER BY (base_price_idr IS NOT NULL AND supplier_cost_idr IS NOT NULL), category, code;
