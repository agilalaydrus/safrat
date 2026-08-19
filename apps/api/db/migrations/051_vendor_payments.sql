-- +goose Up

-- Tracks committed vendor payment obligations per season. Each row = one
-- scheduled payment to a vendor (hotel deposit, bus block, catering, etc).
-- amount is BIGINT rupiah, matching orders.total_price_idr — this
-- codebase never represents IDR with fractional subunits.
CREATE TABLE vendor_payments (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id     UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  vendor_name   TEXT        NOT NULL,
  category      TEXT        NOT NULL DEFAULT 'HOTEL'
                            CHECK (category IN ('HOTEL','TRANSPORT','CATERING','VISA','INSURANCE','OTHER')),
  description   TEXT        NOT NULL DEFAULT '',
  amount_idr    BIGINT      NOT NULL CHECK (amount_idr > 0),
  due_date      DATE        NOT NULL,
  status        TEXT        NOT NULL DEFAULT 'PENDING'
                            CHECK (status IN ('PENDING','PAID','OVERDUE','CANCELLED')),
  paid_at       TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX vendor_payments_operator_season_idx ON vendor_payments(operator_id, season_id);
CREATE INDEX vendor_payments_due_date_idx        ON vendor_payments(due_date, status);

CREATE TRIGGER vendor_payments_set_updated_at
  BEFORE UPDATE ON vendor_payments
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS vendor_payments;
