-- +goose Up
-- Matriks Notifikasi + Jam Tenang (§4.9 DESAIN). Scoped honestly to what is
-- actually wired today: three routine push-only cascade events
-- (GroupCityUpdated, KloterStatusUpdated, RitualBulkCompleted, all in
-- internal/worker/outbox.go) get an on/off toggle and can be muted during a
-- quiet-hours window. This is not a channel-per-event grid — push is the
-- only channel any of these three events has ever had, and building a
-- multi-channel matrix over channels that don't exist would be exactly the
-- "looks like it enforces something" screen this feature exists to avoid.
--
-- HealthReportCreated (a BERAT health alert to staff) is deliberately never
-- toggleable and never muted by quiet hours — same reasoning as SOS never
-- being rate-limited: a health emergency at 2am is not the kind of
-- notification anyone wants silenced by a convenience setting.
--
-- Escalation rules are not part of this table. The only real escalation
-- logic in this codebase is the SOS 10-minute rule
-- (db/query/sos_alert.sql's EscalateStaleSOSAlerts) — a safety mechanism,
-- not a business setting, and not touched here. See the task file for why.
CREATE TABLE operator_notification_settings (
  operator_id                 UUID        PRIMARY KEY REFERENCES operators(id) ON DELETE CASCADE,
  quiet_hours_enabled         BOOLEAN     NOT NULL DEFAULT FALSE,
  -- Local wall-clock time, interpreted in the operator's own timezone
  -- concept elsewhere in this codebase (Asia/Jakarta) — not UTC, so
  -- "22:00–06:00" means what an operator typing it expects.
  quiet_hours_start           TIME        NOT NULL DEFAULT '22:00',
  quiet_hours_end             TIME        NOT NULL DEFAULT '06:00',
  notify_group_city_change    BOOLEAN     NOT NULL DEFAULT TRUE,
  notify_kloter_status_change BOOLEAN     NOT NULL DEFAULT TRUE,
  notify_ritual_bulk_complete BOOLEAN     NOT NULL DEFAULT TRUE,
  updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TRIGGER operator_notification_settings_set_updated_at BEFORE UPDATE ON operator_notification_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS operator_notification_settings;
