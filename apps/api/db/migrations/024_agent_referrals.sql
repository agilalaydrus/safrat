-- +goose Up
ALTER TABLE agents
  ADD COLUMN referral_code TEXT UNIQUE NOT NULL DEFAULT upper(substr(replace(gen_random_uuid()::text, '-', ''), 1, 8)),
  ADD COLUMN tier TEXT NOT NULL DEFAULT 'BRONZE' CHECK (tier IN ('BRONZE','SILVER','GOLD')),
  ADD COLUMN referred_by_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL;
CREATE INDEX agents_referred_by_idx ON agents(referred_by_agent_id);

-- +goose Down
ALTER TABLE agents
  DROP COLUMN IF EXISTS referred_by_agent_id,
  DROP COLUMN IF EXISTS tier,
  DROP COLUMN IF EXISTS referral_code;
