-- name: GetSeasonAnalytics :one
SELECT
  COUNT(DISTINCT p.id)                                                        AS total_pilgrims,
  COUNT(DISTINCT p.id) FILTER (WHERE p.payment_status = 'PAID')              AS paid_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.payment_status = 'DP')                AS dp_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.payment_status = 'UNPAID' OR p.payment_status IS NULL) AS unpaid_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.documents_passport AND p.documents_photo AND p.documents_vaccine) AS docs_complete,
  COUNT(DISTINCT p.id) FILTER (WHERE p.hotel_checked_in)                     AS checked_in_count,
  COUNT(DISTINCT ra.id)                                                       AS rooms_allocated,
  COUNT(DISTINCT sa.id)                                                       AS seats_assigned,
  COUNT(DISTINCT p.id) FILTER (WHERE p.requires_wheelchair)                   AS wheelchair_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.group_id IS NULL)                      AS unassigned_group_count,
  COUNT(DISTINCT p.id) FILTER (WHERE p.kloter_id IS NULL)                     AS unassigned_kloter_count
FROM pilgrims p
LEFT JOIN room_allocations ra ON ra.pilgrim_id = p.id
LEFT JOIN seat_assignments sa ON sa.pilgrim_id = p.id
WHERE p.operator_id = $1
  AND p.season_id   = $2
  AND NOT p.is_substituted;

-- name: GetSeasonOrderStats :one
-- Season-scoped revenue (unlike ListAgentPayouts/GetAgentPayoutSummary,
-- which are deliberately lifetime — see their doc comments in order.sql).
-- Revenue is net of refunds: money that went back to the pilgrim was never
-- revenue. order_payments reports zero for unpaid orders, so the count still
-- covers every order while the revenue covers only what is actually held.
SELECT
  COUNT(*)::int AS order_count,
  COALESCE(SUM(net_paid_idr), 0)::bigint AS total_revenue_idr
FROM order_payments
WHERE operator_id = $1 AND season_id = $2;

-- name: ListKloterFillForSeason :many
SELECT k.code AS kloter_code, k.pilgrim_count::int, k.capacity::int
FROM (
  SELECT kl.id, kl.code, kl.capacity, COUNT(p.id)::int AS pilgrim_count
  FROM kloters kl
  LEFT JOIN pilgrims p ON p.kloter_id = kl.id AND p.is_substituted = false
  WHERE kl.operator_id = $1 AND kl.season_id = $2
  GROUP BY kl.id
) k
ORDER BY k.code ASC;

-- name: ListHotelOccupancyForSeason :many
SELECT h.name AS hotel_name, h.city,
  COALESCE(SUM(r.capacity), 0)::int AS capacity,
  COUNT(ra.id)::int AS allocated
FROM hotels h
LEFT JOIN rooms r ON r.hotel_id = h.id
LEFT JOIN room_allocations ra ON ra.room_id = r.id
WHERE h.operator_id = $1 AND h.season_id = $2
GROUP BY h.id, h.name, h.city
ORDER BY h.city ASC, h.name ASC;

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
