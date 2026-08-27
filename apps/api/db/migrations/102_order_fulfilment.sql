-- +goose Up

-- A paid order for a digital product currently ends there: the money is taken
-- and nothing is ever sent. This is the state that has been missing.
--
-- Separate from orders.status on purpose. Those answer different questions —
-- "did the jamaah pay" and "did the supplier deliver" — and a single column
-- forced to carry both would make "paid but undelivered" inexpressible, which
-- is precisely the state that needs to be visible.
CREATE TABLE order_fulfilments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  -- One fulfilment per order, enforced here rather than by the worker checking
  -- first: two workers picking up the same order both pass a check-then-act,
  -- and the result is a jamaah's pulsa sent twice at our cost.
  order_id UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  supplier_id UUID REFERENCES suppliers(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'PENDING'
    CHECK (status IN ('PENDING', 'SENT', 'DELIVERED', 'FAILED', 'NEEDS_REVIEW')),
  -- The supplier's own transaction id, once they give one. This is what a
  -- dispute is argued with.
  supplier_reference TEXT NOT NULL DEFAULT '',
  attempts INT NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  -- Set when a human resolved a NEEDS_REVIEW rather than the supplier
  -- answering, so the two are never confused after the fact.
  resolved_by_user_id TEXT,
  resolution_note TEXT NOT NULL DEFAULT '',
  sent_at TIMESTAMPTZ,
  delivered_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX order_fulfilments_status_idx ON order_fulfilments (status, created_at);
-- The two queues that need a human. NEEDS_REVIEW is a supplier saying something
-- unreadable; a long-SENT fulfilment is one that never answered at all.
CREATE INDEX order_fulfilments_attention_idx
  ON order_fulfilments (created_at) WHERE status IN ('NEEDS_REVIEW', 'SENT');

-- Suppliers accept requests in shapes as varied as their responses, so the
-- request body is a template rather than code, for the same reason the reading
-- rules are patterns: a supplier changing a field name should be a row edit.
--
-- Placeholders: {{sku}}, {{reference}}, {{amount}}, {{destination}}.
ALTER TABLE suppliers
  ADD COLUMN request_template TEXT NOT NULL DEFAULT '',
  -- Where a supplier posts their asynchronous result. Stored so the panel can
  -- show what to give them; the endpoint itself is one shared route.
  ADD COLUMN callback_token TEXT NOT NULL DEFAULT '';

-- Callbacks arrive authenticated by this token, so it has to be unique across
-- suppliers or one supplier's token would settle another's transactions.
CREATE UNIQUE INDEX suppliers_callback_token_idx
  ON suppliers (callback_token) WHERE callback_token <> '';

-- +goose Down
DROP INDEX suppliers_callback_token_idx;
ALTER TABLE suppliers DROP COLUMN callback_token, DROP COLUMN request_template;
DROP TABLE order_fulfilments;
