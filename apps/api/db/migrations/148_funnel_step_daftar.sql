-- +goose Up
-- The platform's own middle step.
--
-- TawafiqHub's funnel is "/" → "/sign-up" → an active tenant, and the middle
-- step had nowhere to go: the CHECK allowed only the six storefront steps.
--
-- Reusing KATALOG for it was the cheaper option and the wrong one. KATALOG
-- means "opened one journey's registration page"; on the platform the same row
-- would mean "opened the sign-up form". One column with two meanings depending
-- on whether operator_id is NULL is a trap for the next query somebody writes.
ALTER TABLE funnel_events DROP CONSTRAINT funnel_events_step_check;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_step_check
  CHECK (step IN ('LANDING', 'KATALOG', 'ARTIKEL', 'MULAI_ISI', 'KIRIM', 'SELESAI', 'DAFTAR'));

-- +goose Down
-- Rows using the new value must go first, or the constraint cannot be put back.
DELETE FROM funnel_events WHERE step = 'DAFTAR';
DELETE FROM funnel_daily WHERE step = 'DAFTAR';
ALTER TABLE funnel_events DROP CONSTRAINT funnel_events_step_check;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_step_check
  CHECK (step IN ('LANDING', 'KATALOG', 'ARTIKEL', 'MULAI_ISI', 'KIRIM', 'SELESAI'));
