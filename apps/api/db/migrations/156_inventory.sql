-- +goose Up
-- Physical warehouse stock — koper, ihram, seragam, dan atribut rombongan.
-- Operator-scoped rather than season-scoped: the same stack of koper gets
-- restocked and reused across seasons, unlike a kloter or product which
-- belongs to exactly one.
CREATE TABLE inventory_items (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  sku             TEXT        NOT NULL,
  name            TEXT        NOT NULL,
  unit            TEXT        NOT NULL DEFAULT 'pcs',
  stock           INT         NOT NULL DEFAULT 0 CHECK (stock >= 0),
  min_stock       INT         NOT NULL DEFAULT 0 CHECK (min_stock >= 0),
  max_stock       INT         NOT NULL DEFAULT 0 CHECK (max_stock >= 0),
  unit_cost_idr   BIGINT      NOT NULL DEFAULT 0 CHECK (unit_cost_idr >= 0),
  -- How many of this item one pilgrim needs, so a kloter's roster size can be
  -- checked against stock on hand. NULL means the item is not issued per
  -- pilgrim (e.g. shared signage), not "needs zero".
  per_pilgrim_qty INT,
  per_pilgrim_notes TEXT      NOT NULL DEFAULT '',
  moq             INT         NOT NULL DEFAULT 0 CHECK (moq >= 0),
  lead_time_days  INT         NOT NULL DEFAULT 0 CHECK (lead_time_days >= 0),
  vendor_name     TEXT        NOT NULL DEFAULT '',
  rak             TEXT        NOT NULL DEFAULT '',
  last_restock_at TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (operator_id, sku)
);
CREATE INDEX inventory_items_operator_idx ON inventory_items(operator_id);
CREATE TRIGGER inventory_items_set_updated_at BEFORE UPDATE ON inventory_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Ledger of every stock change. inventory_items.stock is a running total kept
-- in sync with this in the same transaction — the ledger exists so "why is
-- stock 40" has an answer, not just a number.
CREATE TABLE inventory_stock_movements (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  item_id       UUID        NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
  movement_type TEXT        NOT NULL CHECK (movement_type IN ('IN','OUT','ADJUSTMENT')),
  quantity      INT         NOT NULL CHECK (quantity <> 0),
  reason        TEXT        NOT NULL DEFAULT '',
  reference     TEXT        NOT NULL DEFAULT '',
  created_by    TEXT        NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX inventory_stock_movements_item_idx ON inventory_stock_movements(item_id, created_at DESC);

CREATE TABLE purchase_orders (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id  UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  po_number    TEXT        NOT NULL,
  vendor_name  TEXT        NOT NULL DEFAULT '',
  status       TEXT        NOT NULL DEFAULT 'DRAFT'
               CHECK (status IN ('DRAFT','ORDERED','PARTIAL','RECEIVED','CANCELLED')),
  eta_date     DATE,
  notes        TEXT        NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (operator_id, po_number)
);
CREATE INDEX purchase_orders_operator_idx ON purchase_orders(operator_id, status);
CREATE TRIGGER purchase_orders_set_updated_at BEFORE UPDATE ON purchase_orders FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE purchase_order_items (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id       UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  po_id             UUID        NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
  item_id           UUID        NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
  quantity_ordered  INT         NOT NULL CHECK (quantity_ordered > 0),
  quantity_received INT         NOT NULL DEFAULT 0 CHECK (quantity_received >= 0),
  unit_cost_idr     BIGINT      NOT NULL DEFAULT 0 CHECK (unit_cost_idr >= 0),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (quantity_received <= quantity_ordered)
);
CREATE INDEX purchase_order_items_po_idx ON purchase_order_items(po_id);

-- +goose Down
DROP TABLE purchase_order_items;
DROP TABLE purchase_orders;
DROP TABLE inventory_stock_movements;
DROP TABLE inventory_items;
