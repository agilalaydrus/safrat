-- name: CreateManasikCurriculum :one
INSERT INTO manasik_curricula (operator_id, season_id, title, description, sort_order)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateManasikCurriculum :one
UPDATE manasik_curricula
SET title = $3, description = $4, sort_order = $5
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteManasikCurriculum :exec
DELETE FROM manasik_curricula WHERE id = $1 AND operator_id = $2;

-- name: ListManasikCurricula :many
SELECT * FROM manasik_curricula WHERE operator_id = $1 AND season_id = $2 ORDER BY sort_order, title;

-- name: CreateManasikSession :one
INSERT INTO manasik_sessions (
  operator_id, season_id, curriculum_id, kloter_id, title, location,
  instructor_name, scheduled_at, duration_minutes, capacity, notes
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING *;

-- name: UpdateManasikSession :one
UPDATE manasik_sessions
SET curriculum_id = $3, kloter_id = $4, title = $5, location = $6,
    instructor_name = $7, scheduled_at = $8, duration_minutes = $9,
    capacity = $10, notes = $11
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: UpdateManasikSessionStatus :one
UPDATE manasik_sessions SET status = $3 WHERE id = $1 AND operator_id = $2 RETURNING *;

-- name: DeleteManasikSession :exec
DELETE FROM manasik_sessions WHERE id = $1 AND operator_id = $2;

-- name: GetManasikSession :one
SELECT * FROM manasik_sessions WHERE id = $1 AND operator_id = $2;

-- name: ListManasikSessions :many
SELECT s.*, COALESCE(c.title, '') AS curriculum_title, COALESCE(k.code, '') AS kloter_code
FROM manasik_sessions s
LEFT JOIN manasik_curricula c ON c.id = s.curriculum_id
LEFT JOIN kloters k ON k.id = s.kloter_id
WHERE s.operator_id = $1 AND s.season_id = $2
ORDER BY s.scheduled_at;

-- name: UpsertManasikAttendance :one
INSERT INTO manasik_attendance (operator_id, session_id, pilgrim_id, status, notes)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (session_id, pilgrim_id) DO UPDATE
  SET status = EXCLUDED.status, notes = EXCLUDED.notes, recorded_at = NOW()
RETURNING *;

-- name: ListManasikAttendance :many
SELECT a.*, p.full_name AS pilgrim_name, p.passport_number
FROM manasik_attendance a
JOIN pilgrims p ON p.id = a.pilgrim_id
WHERE a.operator_id = $1 AND a.session_id = $2
ORDER BY p.full_name;

-- name: ManasikAttendanceSummary :one
SELECT
  COUNT(*) FILTER (WHERE status = 'PRESENT')::int AS present_count,
  COUNT(*) FILTER (WHERE status = 'ABSENT')::int AS absent_count,
  COUNT(*) FILTER (WHERE status = 'EXCUSED')::int AS excused_count
FROM manasik_attendance
WHERE operator_id = $1 AND session_id = $2;
