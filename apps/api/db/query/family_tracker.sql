-- name: GetFamilyTrackerInfo :one
-- Returns minimal, curated info for the public family tracker page.
-- Authenticated by app_access_code only (no operator session). Never
-- selects passport_number, date_of_birth, phone, room number, or raw
-- GPS coordinates — only a "last update" timestamp, which tells a family
-- member the tracking is live without revealing where the pilgrim is.
SELECT
  p.id,
  SPLIT_PART(p.full_name, ' ', 1) AS first_name,
  p.payment_status,
  p.hotel_checked_in,
  p.status AS pilgrim_status,
  s.name AS season_name,
  s.start_date AS departure_date,
  COALESCE(g.name, '') AS group_name,
  COALESCE(u.name, '') AS leader_name,
  p.last_location_at,
  EXISTS (
    SELECT 1 FROM sos_alerts sa
    WHERE sa.pilgrim_id = p.id AND sa.status IN ('ACTIVE', 'ACKNOWLEDGED', 'ESCALATED')
  ) AS has_active_sos
FROM pilgrims p
JOIN seasons s ON s.id = p.season_id
LEFT JOIN groups g ON g.id = p.group_id
LEFT JOIN "user" u ON u.id = g.leader_id
WHERE p.app_access_code = $1
LIMIT 1;
