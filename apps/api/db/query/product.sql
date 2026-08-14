-- name: CreateProduct :one
INSERT INTO products (operator_id, season_id, name, type, price_idr, duration_days, description, inclusions, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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
SET name = $3, type = $4, price_idr = $5, duration_days = $6,
    description = $7, inclusions = $8, is_active = $9, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products
WHERE id = $1 AND operator_id = $2;
