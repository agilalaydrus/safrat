-- name: CreateProduct :one
INSERT INTO products (operator_id, season_id, name, code, nominal_idr, category, type, price_idr, duration_days, description, inclusions, is_active, platform_margin_bps, operator_margin_bps, agent_margin_bps, default_kloter_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: GetProduct :one
SELECT * FROM products
WHERE id = $1 AND operator_id = $2;

-- name: ListProducts :many
SELECT * FROM products
WHERE operator_id = $1 AND season_id = $2
ORDER BY created_at DESC;

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
