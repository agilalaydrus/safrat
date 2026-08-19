-- name: AssignStaffToKloter :one
INSERT INTO kloter_staff (operator_id, kloter_id, staff_id, staff_name, staff_email, role, duties)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (kloter_id, staff_id) DO UPDATE
  SET role = EXCLUDED.role, duties = EXCLUDED.duties
RETURNING *;

-- name: ListKloterStaff :many
SELECT ks.*, k.code AS kloter_name, k.departure_date
FROM kloter_staff ks
JOIN kloters k ON k.id = ks.kloter_id
WHERE ks.operator_id = $1 AND ks.kloter_id = $2;

-- name: ListMyAssignments :many
-- Staff-facing: show all kloters this staff member is assigned to.
SELECT ks.*, k.code AS kloter_name, k.departure_date, s.name AS season_name
FROM kloter_staff ks
JOIN kloters k ON k.id = ks.kloter_id
JOIN seasons s ON s.id = k.season_id
WHERE ks.staff_id = $1 AND ks.operator_id = $2
ORDER BY k.departure_date ASC;

-- name: ListAllStaffSchedule :many
-- Operator overview: all kloters with their assigned staff.
SELECT
  k.id AS kloter_id, k.code AS kloter_name, k.departure_date,
  s.name AS season_name,
  COUNT(ks.id) AS staff_count,
  COALESCE(STRING_AGG(ks.staff_name, ', ' ORDER BY ks.role), '') AS staff_names
FROM kloters k
JOIN seasons s ON s.id = k.season_id
LEFT JOIN kloter_staff ks ON ks.kloter_id = k.id
WHERE k.operator_id = $1 AND s.id = $2
GROUP BY k.id, k.code, k.departure_date, s.name
ORDER BY k.departure_date ASC;

-- name: RemoveStaffFromKloter :exec
DELETE FROM kloter_staff
WHERE kloter_id = $1 AND staff_id = $2 AND operator_id = $3;
