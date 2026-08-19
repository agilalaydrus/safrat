-- name: GetSeasonAnalytics :one
SELECT
  COUNT(DISTINCT p.id)                                                        AS total_pilgrims,
  COUNT(DISTINCT p.id) FILTER (WHERE p.payment_status = 'PAID')              AS paid_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.payment_status = 'DP')                AS dp_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.payment_status = 'UNPAID' OR p.payment_status IS NULL) AS unpaid_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.documents_passport AND p.documents_photo AND p.documents_vaccine) AS docs_complete,
  COUNT(DISTINCT p.id) FILTER (WHERE p.hotel_checked_in)                     AS checked_in_count,
  COUNT(DISTINCT ra.id)                                                       AS rooms_allocated,
  COUNT(DISTINCT sa.id)                                                       AS seats_assigned
FROM pilgrims p
LEFT JOIN room_allocations ra ON ra.pilgrim_id = p.id
LEFT JOIN seat_assignments sa ON sa.pilgrim_id = p.id
WHERE p.operator_id = $1
  AND p.season_id   = $2
  AND NOT p.is_substituted;

-- name: GetAgentSeasonStats :many
SELECT
  a.name                                                            AS agent_name,
  COUNT(DISTINCT p.id)                                             AS pilgrim_count,
  a.commission_rate
FROM agents a
LEFT JOIN pilgrims p ON p.agent_id = a.id AND p.season_id = $2
WHERE a.operator_id = $1
GROUP BY a.id, a.name, a.commission_rate
ORDER BY pilgrim_count DESC;

-- name: GetPaymentTimelineByMonth :many
SELECT
  DATE_TRUNC('month', p.created_at)::DATE AS month,
  COUNT(*) FILTER (WHERE p.payment_status = 'PAID')   AS paid,
  COUNT(*) FILTER (WHERE p.payment_status = 'DP')     AS dp,
  COUNT(*) FILTER (WHERE p.payment_status = 'UNPAID' OR p.payment_status IS NULL) AS unpaid
FROM pilgrims p
WHERE p.operator_id = $1
  AND p.season_id   = $2
GROUP BY 1
ORDER BY 1;
