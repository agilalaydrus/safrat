-- +goose Up
-- Grace is access policy, not a status label. NULL means follow the platform
-- default; a concrete value is a deliberate tenant exception, including zero.
INSERT INTO platform_settings (key, value)
VALUES ('grace_period_days', '0')
ON CONFLICT (key) DO NOTHING;

ALTER TABLE subscriptions
  ADD COLUMN grace_period_days INTEGER,
  ADD CONSTRAINT subscriptions_grace_period_days_check
    CHECK (grace_period_days BETWEEN 0 AND 90);

-- One definition shared by authentication, dunning, and the platform panel.
-- Duplicating this expression would eventually make one screen call a tenant
-- active while the interceptor has already locked it out.
CREATE FUNCTION platform_grace_period_days() RETURNS INTEGER
LANGUAGE sql STABLE AS $$
  SELECT COALESCE((
    SELECT CASE
      WHEN value ~ '^(0|[1-8]?[0-9]|90)$' THEN value::integer
      ELSE 0
    END
    FROM platform_settings WHERE key = 'grace_period_days'
  ), 0)
$$;

CREATE FUNCTION subscription_effective_access_until(
  paid_until TIMESTAMPTZ,
  grace_override INTEGER
) RETURNS TIMESTAMPTZ
LANGUAGE sql STABLE AS $$
  SELECT paid_until + make_interval(days => COALESCE(grace_override, platform_grace_period_days()))
$$;

-- +goose Down
DROP FUNCTION IF EXISTS subscription_effective_access_until(TIMESTAMPTZ, INTEGER);
DROP FUNCTION IF EXISTS platform_grace_period_days();
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_grace_period_days_check;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS grace_period_days;
DELETE FROM platform_settings WHERE key = 'grace_period_days';
