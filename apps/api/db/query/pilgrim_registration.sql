-- name: CreatePilgrimRegistration :one
INSERT INTO pilgrim_registrations
  (operator_id, season_id, product_id, full_name, passport_number,
   date_of_birth, gender, phone, email, nationality, address, agent_id)
VALUES ($1, $2, NULLIF($3::text, '')::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: ListPilgrimRegistrations :many
SELECT r.*, COALESCE(a.name, '') AS agent_name FROM pilgrim_registrations r
LEFT JOIN agents a ON a.id = r.agent_id
WHERE r.operator_id = $1 AND r.season_id = $2
ORDER BY r.created_at DESC;

-- name: GetPilgrimRegistration :one
SELECT * FROM pilgrim_registrations WHERE id = $1 AND operator_id = $2;

-- name: UpdateRegistrationStatus :one
UPDATE pilgrim_registrations
SET status = $3, notes = $4, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: GetOperatorSeasonForRegistration :one
-- Public storefronts list every operator-owned season that has not ended yet.
-- Keep form validation on that same availability rule: is_active selects the
-- operator's current operational season and must not close registration for
-- other future packages that are still publicly advertised.
SELECT o.id AS operator_id, o.name AS operator_name,
       s.id AS season_id, s.name AS season_name, s.is_active
FROM operators o
JOIN seasons s ON s.operator_id = o.id
WHERE o.id = $1 AND s.id = $2 AND s.end_date >= NOW();

-- name: ListActiveProductsForRegistration :many
SELECT id, name FROM products
WHERE operator_id = $1 AND season_id = $2 AND is_active = true
ORDER BY name ASC;
