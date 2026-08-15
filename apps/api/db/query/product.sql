-- name: CreateProduct :one
INSERT INTO products (operator_id, season_id, name, category, type, price_idr, duration_days, description, inclusions, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
    description = $8, inclusions = $9, is_active = $10, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products
WHERE id = $1 AND operator_id = $2;
