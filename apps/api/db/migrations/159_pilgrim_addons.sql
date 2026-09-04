-- +goose Up
-- Layanan tambahan (§2.11 RENCANA-DASHBOARD-TRAVEL): add-on services sold per
-- pilgrim on top of their package — an executive seat, VIP airport handling,
-- lounge access, a certified badal umrah, extra insurance, special catering,
-- a zamzam water shipment. Season-scoped catalog, like kloters and manasik,
-- so a price set for one season's intake does not silently apply to the next.
CREATE TABLE addon_items (
  id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id    UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id      UUID        NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  name           TEXT        NOT NULL CHECK (length(trim(name)) > 0),
  unit_price_idr BIGINT      NOT NULL DEFAULT 0 CHECK (unit_price_idr >= 0),
  is_active      BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX addon_items_season_name_key ON addon_items (season_id, lower(trim(name)));
CREATE INDEX addon_items_season_idx ON addon_items (operator_id, season_id) WHERE is_active;
CREATE TRIGGER addon_items_set_updated_at BEFORE UPDATE ON addon_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One row per pilgrim per add-on type; quantity covers "two zamzam boxes"
-- rather than two rows. unit_price_idr is copied from the catalog at
-- assignment time and never re-read from it afterward — a later catalog
-- price change must not rewrite what a pilgrim already agreed to pay.
--
-- RESTRICT on the catalog FK: deleting an add-on type that pilgrims already
-- hold would either cascade away a paid line item or silently orphan it.
-- Deactivating (is_active = false) is the way to retire a type; the
-- assignments already sold on it stay intact and readable.
CREATE TABLE pilgrim_addons (
  id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id    UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  pilgrim_id     UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  addon_item_id  UUID        NOT NULL REFERENCES addon_items(id) ON DELETE RESTRICT,
  quantity       INT         NOT NULL DEFAULT 1 CHECK (quantity > 0),
  unit_price_idr BIGINT      NOT NULL CHECK (unit_price_idr >= 0),
  paid           BOOLEAN     NOT NULL DEFAULT FALSE,
  notes          TEXT        NOT NULL DEFAULT '',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (pilgrim_id, addon_item_id)
);
CREATE INDEX pilgrim_addons_pilgrim_idx ON pilgrim_addons (pilgrim_id);
CREATE INDEX pilgrim_addons_operator_idx ON pilgrim_addons (operator_id);
CREATE TRIGGER pilgrim_addons_set_updated_at BEFORE UPDATE ON pilgrim_addons FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS pilgrim_addons;
DROP TABLE IF EXISTS addon_items;
