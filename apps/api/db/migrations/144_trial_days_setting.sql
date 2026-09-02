-- +goose Up
-- Trial length becomes a setting, not a constant.
--
-- The owner decided ten days on 2 September 2026 and the row already said so,
-- but nothing read it: repository code still carried TrialDays = 3, so every
-- travel agency signing up was given three. A commercial figure that needs a
-- release to correct stays wrong for months, and this one was wrong the day it
-- was decided.
--
-- Ten because three is close to useless for an umrah agency: they need to
-- import a spreadsheet, train an admin, and put one real registration through,
-- and three working days can land across a weekend.

-- Same shape as platform_grace_period_days: validated in SQL with a safe
-- fallback, so a malformed row cannot stop signups from working. Anything
-- outside 1..90 is ignored rather than trusted — a typo that hands out a
-- ten-thousand-day trial is worse than one that hands out none.
-- +goose StatementBegin
CREATE FUNCTION platform_trial_days() RETURNS INTEGER
LANGUAGE sql STABLE AS $$
  SELECT COALESCE((
    SELECT CASE
      WHEN value ~ '^([1-9]|[1-8][0-9]|90)$' THEN value::integer
      ELSE 10
    END
    FROM platform_settings WHERE key = 'trial_days'
  ), 10)
$$;
-- +goose StatementEnd

-- The row already exists from migration 139; this only makes sure a database
-- that somehow lacks it still lands on the decided number rather than on
-- whatever a fallback happens to be.
INSERT INTO platform_settings (key, value) VALUES ('trial_days', '10')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DROP FUNCTION IF EXISTS platform_trial_days();
