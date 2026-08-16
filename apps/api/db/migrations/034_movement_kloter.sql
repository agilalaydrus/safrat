-- +goose Up
-- A movement is often a specific batch's transport leg — different kloter
-- depart on different flights/dates, so "Arrival CGK -> Madinah" for
-- Gelombang I's SOC-01 is a different, real, separately-scheduled event
-- from SOC-05's. Nullable: some movements (e.g. a shared ground shuttle)
-- genuinely aren't scoped to one kloter.
ALTER TABLE movements
  ADD COLUMN kloter_id UUID REFERENCES kloters(id) ON DELETE SET NULL;
CREATE INDEX movements_kloter_id_idx ON movements(kloter_id);

-- +goose Down
ALTER TABLE movements DROP COLUMN IF EXISTS kloter_id;
