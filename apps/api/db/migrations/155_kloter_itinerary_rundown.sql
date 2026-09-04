-- +goose Up
-- Rangkaian ("itinerary") is the ordered spine of a kloter's journey:
-- alternating Transport (movement) and Hotel (stay) segments, always
-- starting and ending with Transport — the trip cannot begin or end
-- anywhere but on a vehicle. Armada Bus depends on this: only a Bus
-- movement that has actually been added here is eligible for seat
-- assignment, so an operator cannot assign seats on a movement nobody has
-- scheduled into the plan yet.
CREATE TABLE kloter_itinerary_segments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  kloter_id UUID NOT NULL REFERENCES kloters(id) ON DELETE CASCADE,
  position INT NOT NULL,
  segment_type TEXT NOT NULL CHECK (segment_type IN ('TRANSPORT','HOTEL')),
  movement_id UUID REFERENCES movements(id) ON DELETE CASCADE,
  hotel_id UUID REFERENCES hotels(id) ON DELETE CASCADE,
  notes TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (kloter_id, position),
  CHECK (
    (segment_type = 'TRANSPORT' AND movement_id IS NOT NULL AND hotel_id IS NULL) OR
    (segment_type = 'HOTEL' AND hotel_id IS NOT NULL AND movement_id IS NULL)
  )
);
CREATE INDEX kloter_itinerary_segments_kloter_idx ON kloter_itinerary_segments(kloter_id);

-- Rundown is the day-by-day operational schedule for a kloter — what a
-- coordinator or muttawwif actually reads on the ground. Distinct from
-- Rangkaian above (the transport/hotel spine) and from a Product's
-- product_itinerary_days (a sales-facing template shown before purchase,
-- not the real day's plan for a departed batch).
CREATE TABLE kloter_rundown_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  kloter_id UUID NOT NULL REFERENCES kloters(id) ON DELETE CASCADE,
  day_number INT NOT NULL CHECK (day_number > 0),
  position INT NOT NULL DEFAULT 0,
  time_label TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  location TEXT NOT NULL DEFAULT '',
  pic TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX kloter_rundown_items_kloter_idx ON kloter_rundown_items(kloter_id, day_number, position);

-- +goose Down
DROP TABLE kloter_rundown_items;
DROP TABLE kloter_itinerary_segments;
