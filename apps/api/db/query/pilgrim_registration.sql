-- name: CreatePilgrimRegistration :one
INSERT INTO pilgrim_registrations
  (operator_id, season_id, product_id, full_name, passport_number,
   date_of_birth, gender, phone, email, nationality, address)
VALUES ($1, $2, NULLIF($3::text, '')::uuid, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListPilgrimRegistrations :many
SELECT * FROM pilgrim_registrations
WHERE operator_id = $1 AND season_id = $2
ORDER BY created_at DESC;

-- name: GetPilgrimRegistration :one
SELECT * FROM pilgrim_registrations WHERE id = $1 AND operator_id = $2;

-- name: UpdateRegistrationStatus :one
UPDATE pilgrim_registrations
SET status = $3, notes = $4, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: GetOperatorSeasonForRegistration :one
-- Dipakai untuk validasi public form: cek apakah operator+season aktif menerima pendaftaran
SELECT o.id AS operator_id, o.name AS operator_name,
       s.id AS season_id, s.name AS season_name, s.is_active
FROM operators o
JOIN seasons s ON s.operator_id = o.id
WHERE o.id = $1 AND s.id = $2 AND s.is_active = true;

-- name: ListActiveProductsForRegistration :many
SELECT id, name FROM products
WHERE operator_id = $1 AND season_id = $2 AND is_active = true
ORDER BY name ASC;
