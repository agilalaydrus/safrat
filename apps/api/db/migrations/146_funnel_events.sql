-- +goose Up
-- Who arrives, how far they get, and where they stop.
--
-- Nothing measured any of this before. Registrations and CRM leads only record
-- somebody after they have typed a name and a phone number, so everyone who
-- arrived and left was invisible — which is exactly the part that can be fixed.
--
-- One table serves two funnels, split by operator_id: a travel agency's
-- storefront, and TawafiqHub's own site. NULL means the platform.

CREATE TABLE funnel_events (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  -- NULL = the platform's own site. Also the ownership boundary: an agency may
  -- read its own rows and nobody else's, enforced in the repository the way
  -- branch isolation is.
  operator_id   UUID REFERENCES operators(id) ON DELETE CASCADE,
  -- SHA256(salt ‖ date ‖ IP ‖ user agent). Not reversible to an IP, and it
  -- changes every day, so nobody can be followed across days — not even by us.
  -- The raw IP is never written to any column here; that is what keeps this
  -- table aggregate rather than personal data.
  visitor_hash  TEXT NOT NULL CHECK (length(visitor_hash) = 64),
  step          TEXT NOT NULL CHECK (step IN (
                  'LANDING', 'KATALOG', 'ARTIKEL', 'MULAI_ISI', 'KIRIM', 'SELESAI')),
  path          TEXT NOT NULL DEFAULT '',
  -- Set on ARTIKEL. Without it every article read collapses into one useless
  -- number, and content cannot be measured at all.
  article_slug  TEXT NOT NULL DEFAULT '',
  referrer_host TEXT NOT NULL DEFAULT '',
  utm_source    TEXT NOT NULL DEFAULT '',
  utm_campaign  TEXT NOT NULL DEFAULT '',
  -- Geolocation result, stored as place names. City level and deliberately no
  -- finer.
  city          TEXT NOT NULL DEFAULT '',
  province      TEXT NOT NULL DEFAULT '',
  -- Filled on SELESAI so the funnel can be joined to what it produced, and
  -- through that to money.
  entity_id     UUID,
  occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX funnel_events_rollup_idx ON funnel_events (occurred_at);
CREATE INDEX funnel_events_operator_idx ON funnel_events (operator_id, occurred_at DESC);

-- Daily rollup. Screens read this and never recompute from raw rows — the same
-- reasoning as usage_counters, and what keeps the page fast a year from now.
CREATE TABLE funnel_daily (
  operator_id  UUID REFERENCES operators(id) ON DELETE CASCADE,
  day          DATE NOT NULL,
  step         TEXT NOT NULL,
  utm_source   TEXT NOT NULL DEFAULT '',
  visitors     INTEGER NOT NULL DEFAULT 0 CHECK (visitors >= 0),
  events       INTEGER NOT NULL DEFAULT 0 CHECK (events >= 0),
  computed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A unique index rather than a primary key, and that distinction is the whole
-- point.
--
-- A primary key cannot contain a nullable column, so using one would force the
-- platform's own rows to carry a fake operator id — a sentinel that every
-- later query would have to remember to exclude, and that somebody would
-- eventually forget. Postgres treats NULLs as distinct in a unique index by
-- default, which would silently let the platform accumulate a duplicate
-- summary row per rollup, so NULLS NOT DISTINCT is what makes the constraint
-- mean what it reads as.
CREATE UNIQUE INDEX funnel_daily_key_idx
  ON funnel_daily (operator_id, day, step, utm_source) NULLS NOT DISTINCT;
CREATE INDEX funnel_daily_day_idx ON funnel_daily (day DESC);

-- +goose Down
DROP TABLE IF EXISTS funnel_daily;
DROP TABLE IF EXISTS funnel_events;
