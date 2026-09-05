-- +goose Up
-- E2 (TUGAS-PANEL-SAAS.md §10.1 DESAIN): a channel from the platform to
-- tenants that does not exist today — "saat ada pemeliharaan, insiden, atau
-- fitur baru, tidak ada cara memberi tahu selain menghubungi satu per satu."
--
-- recipient_filter is stored as the CRITERIA (mode + plan/operator_ids), not
-- as a frozen list — "jumlah penerima dihitung langsung dari data, bukan
-- diperkirakan" applies at send time too: a scheduled announcement recomputes
-- who currently matches when it actually fires, not who matched when it was
-- composed. announcement_deliveries is the frozen snapshot of who it was
-- actually sent to, taken at that moment — that snapshot is what read
-- tracking and the "recipient already announced to in the last 24h" overlap
-- check both read from.
CREATE TABLE announcements (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_user_id    TEXT NOT NULL,
  title            TEXT NOT NULL CHECK (length(trim(title)) > 0),
  body             TEXT NOT NULL CHECK (length(trim(body)) > 0),
  link             TEXT,
  channels         TEXT[] NOT NULL DEFAULT ARRAY['IN_APP'],
  recipient_filter JSONB NOT NULL,
  recipient_count  INT NOT NULL DEFAULT 0,
  scheduled_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  sent_at          TIMESTAMPTZ,
  idempotency_key  TEXT NOT NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (admin_user_id, idempotency_key)
);

-- The worker's due-for-dispatch sweep: unsent announcements whose scheduled
-- time has arrived. Tiny and self-limiting (nothing stays in this state for
-- long), same shape as cascade_events' own partial index.
CREATE INDEX announcements_due_idx ON announcements (scheduled_at) WHERE sent_at IS NULL;

CREATE TABLE announcement_deliveries (
  announcement_id UUID NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
  operator_id     UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  email_sent_at   TIMESTAMPTZ,
  read_at         TIMESTAMPTZ,
  read_by_user_id TEXT,
  PRIMARY KEY (announcement_id, operator_id)
);

-- The operator-facing inbox query: "everything sent to me, newest first."
CREATE INDEX announcement_deliveries_operator_idx ON announcement_deliveries (operator_id, announcement_id);

-- A sent announcement's title and body must stay exactly what recipients
-- actually read — "menyunting pesan yang sudah dibaca orang mengubah catatan
-- tentang apa yang mereka baca" (§10.1 DESAIN). No RPC exposes an update at
-- all, but the same leaked-credential threat migration 125 wrote about
-- applies here too, so the privilege is removed the same way.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    REVOKE UPDATE (title, body, link, recipient_filter, admin_user_id, idempotency_key) ON announcements FROM safrat_app;
    REVOKE DELETE, TRUNCATE ON announcements FROM safrat_app;
    REVOKE DELETE, TRUNCATE ON announcement_deliveries FROM safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'safrat_app') THEN
    GRANT UPDATE ON announcements TO safrat_app;
    GRANT DELETE ON announcements TO safrat_app;
    GRANT DELETE ON announcement_deliveries TO safrat_app;
  END IF;
END
$$;
-- +goose StatementEnd
DROP TABLE IF EXISTS announcement_deliveries;
DROP TABLE IF EXISTS announcements;
