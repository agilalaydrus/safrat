-- +goose Up
-- Support (§2.17 RENCANA): a ticket to the platform, with a priority and a
-- response-time target — "kecil, tapi bukti ada yang bertanggung jawab"
-- (small, but proof someone is accountable). This migration is the
-- operator-facing half only: create a ticket, read it back, reply on the
-- thread. The platform-side inbox at /admin (view every tenant's tickets,
-- reply as staff, change status) is Panel SaaS territory — see
-- TUGAS-PANEL-SAAS.md.
CREATE TABLE support_tickets (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  subject       TEXT        NOT NULL CHECK (length(trim(subject)) > 0),
  -- LOW | MEDIUM | HIGH | URGENT. Fixed set, not a free label — the
  -- response-time target below is read off this value, and a made-up
  -- priority would have no target to answer to.
  priority      TEXT        NOT NULL DEFAULT 'MEDIUM' CHECK (priority IN ('LOW','MEDIUM','HIGH','URGENT')),
  status        TEXT        NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','IN_PROGRESS','RESOLVED','CLOSED')),
  created_by_user_id TEXT   NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at   TIMESTAMPTZ,
  CHECK ((status IN ('RESOLVED','CLOSED')) = (resolved_at IS NOT NULL))
);
CREATE INDEX support_tickets_operator_idx ON support_tickets (operator_id, created_at DESC);
CREATE INDEX support_tickets_open_idx ON support_tickets (created_at) WHERE status IN ('OPEN','IN_PROGRESS');
CREATE TRIGGER support_tickets_set_updated_at BEFORE UPDATE ON support_tickets FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One row per message on the thread — from the operator's own staff for
-- now; a platform-staff reply is the same table with author_is_platform
-- true, so the operator-facing thread view never needs a second query to
-- render both sides once the admin inbox writes to it.
CREATE TABLE support_ticket_messages (
  id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id           UUID        NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
  body                TEXT        NOT NULL CHECK (length(trim(body)) > 0),
  author_user_id      TEXT        NOT NULL DEFAULT '',
  author_name         TEXT        NOT NULL,
  author_is_platform  BOOLEAN     NOT NULL DEFAULT FALSE,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX support_ticket_messages_ticket_idx ON support_ticket_messages (ticket_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS support_ticket_messages;
DROP TABLE IF EXISTS support_tickets;
