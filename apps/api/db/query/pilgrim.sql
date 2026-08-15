-- name: CreatePilgrim :one
INSERT INTO pilgrims (
  season_id, operator_id, group_id, full_name, passport_number, nationality,
  date_of_birth, gender, photo_url, phone, emergency_contact, preferred_lang,
  medical_notes, requires_wheelchair, mahram_id
) SELECT
  $1, $2, NULLIF($3::text, '')::uuid, $4, $5, $6,
  $7, $8, NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), $12,
  NULLIF($13, ''), $14, NULLIF($15::text, '')::uuid
WHERE EXISTS (
  SELECT 1 FROM seasons WHERE id = $1 AND operator_id = $2
)
RETURNING *;

-- name: GetPilgrim :one
SELECT * FROM pilgrims
WHERE id = $1 AND operator_id = $2;

-- name: GetPilgrimByAppAccessCode :one
SELECT * FROM pilgrims
WHERE app_access_code = $1 AND is_substituted = false;

-- name: UpdatePilgrimLocation :exec
UPDATE pilgrims
SET last_lat = $2, last_lng = $3, last_location_at = NOW()
WHERE app_access_code = $1 AND is_substituted = false;

-- name: GetPilgrimByPassport :one
SELECT * FROM pilgrims
WHERE operator_id = $1 AND season_id = $2 AND passport_number = $3;

-- name: ListPilgrims :many
SELECT * FROM pilgrims
WHERE operator_id = $1 AND season_id = $2
ORDER BY full_name ASC
LIMIT $3 OFFSET $4;

-- name: UpdatePilgrim :one
UPDATE pilgrims
SET group_id = NULLIF($3::text, '')::uuid,
    full_name = $4,
    passport_number = $5,
    nationality = $6,
    date_of_birth = $7,
    gender = $8,
    photo_url = NULLIF($9, ''),
    phone = NULLIF($10, ''),
    emergency_contact = NULLIF($11, ''),
    preferred_lang = $12,
    medical_notes = NULLIF($13, ''),
    requires_wheelchair = $14,
    mahram_id = NULLIF($15::text, '')::uuid,
    updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: MarkSubstituted :exec
UPDATE pilgrims
SET is_substituted = TRUE,
    substituted_by_id = NULL,
    updated_at = NOW()
WHERE id = $1 AND operator_id = $2;

-- name: SubstitutePilgrim :exec
UPDATE pilgrims
SET is_substituted = TRUE,
    substituted_by_id = $2,
    updated_at = NOW()
WHERE id = $1 AND operator_id = $3;

-- name: TransferPilgrimGroup :exec
UPDATE pilgrims
SET group_id = $2,
    updated_at = NOW()
WHERE id = $1 AND operator_id = $3;

-- name: CreateAuditLog :exec
INSERT INTO audit_logs (operator_id, user_id, action, entity_type, entity_id, metadata)
VALUES ($1, $2, $3, 'pilgrim', $4, jsonb_build_object('message', $5));

-- name: CountPilgrimsByOperator :one
SELECT COUNT(*) FROM pilgrims WHERE operator_id = $1;

-- name: GetPilgrimStats :one
SELECT
  COUNT(*)::int AS total,
  COUNT(*) FILTER (WHERE is_substituted)::int AS substituted,
  COUNT(*) FILTER (WHERE requires_wheelchair)::int AS requires_wheelchair,
  COUNT(*) FILTER (WHERE group_id IS NULL)::int AS unassigned_group
FROM pilgrims
WHERE operator_id = $1 AND season_id = $2;
