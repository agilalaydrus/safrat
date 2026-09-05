-- name: GetOperatorNotificationSettings :one
SELECT * FROM operator_notification_settings WHERE operator_id = $1;

-- name: UpsertOperatorNotificationSettings :one
INSERT INTO operator_notification_settings (
  operator_id, quiet_hours_enabled, quiet_hours_start, quiet_hours_end,
  notify_group_city_change, notify_kloter_status_change, notify_ritual_bulk_complete
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (operator_id) DO UPDATE
  SET quiet_hours_enabled = EXCLUDED.quiet_hours_enabled,
      quiet_hours_start = EXCLUDED.quiet_hours_start,
      quiet_hours_end = EXCLUDED.quiet_hours_end,
      notify_group_city_change = EXCLUDED.notify_group_city_change,
      notify_kloter_status_change = EXCLUDED.notify_kloter_status_change,
      notify_ritual_bulk_complete = EXCLUDED.notify_ritual_bulk_complete
RETURNING *;
