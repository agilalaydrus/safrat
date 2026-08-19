-- name: GetCertificateData :one
-- All fields needed to render a complete trip certificate. Authenticated
-- by app_access_code only — never exposes anything beyond what's already
-- shown elsewhere in the pilgrim app.
SELECT
  p.id, p.full_name, p.passport_number, p.nationality,
  s.name AS season_name, s.type AS season_type,
  s.start_date, s.end_date,
  o.name AS operator_name, o.license_number,
  COALESCE(g.name, '') AS group_name,
  COALESCE(u.name, '') AS leader_name,
  COALESCE(STRING_AGG(DISTINCT h.name, ', '), '') AS hotels_visited,
  COALESCE(STRING_AGG(DISTINCT h.name, ', ') FILTER (WHERE h.city ILIKE 'Makkah' OR h.city ILIKE 'Mecca'), '') AS makkah_hotels,
  COALESCE(STRING_AGG(DISTINCT h.name, ', ') FILTER (WHERE h.city ILIKE 'Madinah' OR h.city ILIKE 'Medina'), '') AS madinah_hotels
FROM pilgrims p
JOIN seasons   s  ON s.id  = p.season_id
JOIN operators o  ON o.id  = p.operator_id
LEFT JOIN groups g ON g.id = p.group_id
LEFT JOIN "user" u ON u.id = g.leader_id
LEFT JOIN room_allocations ra ON ra.pilgrim_id = p.id
LEFT JOIN rooms r  ON r.id  = ra.room_id
LEFT JOIN hotels h ON h.id  = r.hotel_id
WHERE p.app_access_code = $1 AND p.is_substituted = false
GROUP BY p.id, p.full_name, p.passport_number, p.nationality,
         s.name, s.type, s.start_date, s.end_date, o.name, o.license_number,
         g.name, u.name;
