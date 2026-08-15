-- name: GetPilgrimAppInfo :one
-- A pilgrim can hold one room allocation per hotel in a season (e.g. Makkah + Madinah),
-- so DISTINCT ON picks the most recently allocated room as the one shown on Home —
-- a display simplification, not a claim there's only ever one.
SELECT DISTINCT ON (p.id)
  p.*,
  g.name AS group_name,
  h.name AS hotel_name,
  r.room_number AS room_number
FROM pilgrims p
LEFT JOIN groups g ON g.id = p.group_id
LEFT JOIN room_allocations ra ON ra.pilgrim_id = p.id
LEFT JOIN rooms r ON r.id = ra.room_id
LEFT JOIN hotels h ON h.id = r.hotel_id
WHERE p.app_access_code = $1 AND p.is_substituted = false
ORDER BY p.id, ra.allocated_at DESC NULLS LAST;

-- name: ListUpcomingMovementsForSeason :many
SELECT * FROM movements
WHERE season_id = $1 AND operator_id = $2 AND status = 'scheduled'
ORDER BY scheduled_at ASC
LIMIT 10;
