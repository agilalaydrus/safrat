-- +goose Up
-- ListAgentPayouts/GetAgentPayoutSummary (Agents dashboard, leader wallet)
-- both filter orders by agent_id + status='PAID' on every load. orders is
-- the one table expected to grow continuously (every digital-product
-- sale) — without this it's a full sequential scan as that grows.
CREATE INDEX orders_agent_status_idx ON orders(agent_id, status);

-- +goose Down
DROP INDEX IF EXISTS orders_agent_status_idx;
