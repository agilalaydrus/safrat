-- name: CreatePilgrim :one
INSERT INTO pilgrims (
  season_id, operator_id, group_id, full_name, passport_number, nationality,
  date_of_birth, gender, photo_url, phone, emergency_contact, preferred_lang,
  medical_notes, requires_wheelchair, mahram_id, kloter_id, email
) SELECT
  $1, $2, NULLIF($3::text, '')::uuid, $4, $5, $6,
  $7, $8, NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), $12,
  NULLIF($13, ''), $14, NULLIF($15::text, '')::uuid, NULLIF($16::text, '')::uuid, NULLIF($17, '')
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
    kloter_id = NULLIF($16::text, '')::uuid,
    email = NULLIF($17, ''),
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

-- name: RegenerateAccessCode :exec
UPDATE pilgrims
SET app_access_code = gen_random_uuid()::text,
    updated_at = NOW()
WHERE id = $1 AND operator_id = $2;

-- name: TransferPilgrimGroup :exec
UPDATE pilgrims
SET group_id = $2,
    updated_at = NOW()
WHERE id = $1 AND operator_id = $3;

-- name: CountPilgrimsBySeason :one
SELECT COUNT(*) FROM pilgrims WHERE operator_id = $1 AND season_id = $2;

-- name: GetPilgrimStats :one
-- Substituted pilgrims are replaced/inactive — every other roster count in
-- the app (group rosters, group pilgrim_count, etc.) already excludes them,
-- so total/wheelchair/unassigned here must too, or the dashboard overcounts
-- against what every other screen shows.
SELECT
  COUNT(*) FILTER (WHERE NOT is_substituted)::int AS total,
  COUNT(*) FILTER (WHERE is_substituted)::int AS substituted,
  COUNT(*) FILTER (WHERE NOT is_substituted AND requires_wheelchair)::int AS requires_wheelchair,
  COUNT(*) FILTER (WHERE NOT is_substituted AND group_id IS NULL)::int AS unassigned_group,
  COUNT(*) FILTER (WHERE NOT is_substituted AND kloter_id IS NULL)::int AS unassigned_kloter
FROM pilgrims
WHERE operator_id = $1 AND season_id = $2;

-- name: GetPilgrimStatsByKloter :one
-- Same shape as GetPilgrimStats, scoped to one kloter — powers the Operator
-- Dashboard's kloter filter.
SELECT
  COUNT(*) FILTER (WHERE NOT is_substituted)::int AS total,
  COUNT(*) FILTER (WHERE is_substituted)::int AS substituted,
  COUNT(*) FILTER (WHERE NOT is_substituted AND requires_wheelchair)::int AS requires_wheelchair,
  COUNT(*) FILTER (WHERE NOT is_substituted AND group_id IS NULL)::int AS unassigned_group,
  0::int AS unassigned_kloter
FROM pilgrims
WHERE operator_id = $1 AND season_id = $2 AND kloter_id = $3;

-- name: UpdatePilgrimPayment :one
UPDATE pilgrims
SET payment_status = $3, payment_receipt_url = $4,
    payment_notes = $5, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: UpdatePilgrimDocuments :one
UPDATE pilgrims
SET documents_passport = $3, documents_photo = $4,
    documents_vaccine = $5, passport_expiry_date = $6,
    vaccine_meningitis_date = $7, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: UpdatePilgrimEmergencyContact :one
UPDATE pilgrims
SET emergency_contact_name = $3,
    emergency_contact_phone = $4, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: CheckInPilgrimHotel :one
UPDATE pilgrims
SET hotel_checked_in = $3, updated_at = NOW()
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: ListPilgrimsWithExpiringPassports :many
SELECT * FROM pilgrims
WHERE operator_id = $1
  AND season_id = $2
  AND passport_expiry_date IS NOT NULL
  AND passport_expiry_date < $3
  AND NOT is_substituted
ORDER BY passport_expiry_date ASC;
