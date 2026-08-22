-- +goose Up
-- room_allocations had UNIQUE(pilgrim_id) — structurally impossible for a
-- pilgrim to hold a room in both Makkah and Madinah at once, even though
-- ListPilgrimRoomAssignments' own comment already documented "one room
-- per pilgrim per hotel" as the intended rule (see CODEX_SPEC.md §7) and
-- picks the most-recent allocation via DISTINCT ON as if a second one
-- could exist. In practice AllocatePilgrimTx just failed outright on the
-- second hotel. hotel_id is denormalized onto the row (not just derivable
-- via room_id -> rooms.hotel_id) so the per-hotel uniqueness constraint
-- doesn't need a subquery.
ALTER TABLE room_allocations ADD COLUMN IF NOT EXISTS hotel_id UUID REFERENCES hotels(id) ON DELETE CASCADE;
UPDATE room_allocations ra SET hotel_id = r.hotel_id FROM rooms r WHERE r.id = ra.room_id AND ra.hotel_id IS NULL;
ALTER TABLE room_allocations ALTER COLUMN hotel_id SET NOT NULL;

ALTER TABLE room_allocations DROP CONSTRAINT IF EXISTS room_allocations_pilgrim_id_key;
ALTER TABLE room_allocations ADD CONSTRAINT room_allocations_pilgrim_hotel_key UNIQUE (pilgrim_id, hotel_id);
CREATE INDEX IF NOT EXISTS room_allocations_hotel_idx ON room_allocations(hotel_id);

-- +goose Down
DROP INDEX IF EXISTS room_allocations_hotel_idx;
ALTER TABLE room_allocations DROP CONSTRAINT IF EXISTS room_allocations_pilgrim_hotel_key;
ALTER TABLE room_allocations ADD CONSTRAINT room_allocations_pilgrim_id_key UNIQUE (pilgrim_id);
ALTER TABLE room_allocations DROP COLUMN IF EXISTS hotel_id;
