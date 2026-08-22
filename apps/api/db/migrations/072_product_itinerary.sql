-- +goose Up
-- Turns a TRAVEL_PACKAGE Product from a priced SKU with free-text
-- "inclusions" into a real, structured itinerary an operator can build:
-- day-by-day schedule, which hotels are included, and (optionally) which
-- kloter a buyer gets slotted into automatically.
ALTER TABLE products ADD COLUMN default_kloter_id UUID REFERENCES kloters(id) ON DELETE SET NULL;

CREATE TABLE product_itinerary_days (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  day_number  INT NOT NULL CHECK (day_number > 0),
  title       TEXT NOT NULL,
  city        TEXT NOT NULL DEFAULT '',
  activities  TEXT NOT NULL DEFAULT '',
  meal_breakfast BOOLEAN NOT NULL DEFAULT FALSE,
  meal_lunch     BOOLEAN NOT NULL DEFAULT FALSE,
  meal_dinner    BOOLEAN NOT NULL DEFAULT FALSE,
  UNIQUE(product_id, day_number)
);
CREATE INDEX product_itinerary_days_product_idx ON product_itinerary_days(product_id, day_number);

-- Which hotels this package includes — many-to-many since a Hajj/Umrah
-- package almost always spans Makkah + Madinah.
CREATE TABLE product_hotels (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  hotel_id    UUID NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
  UNIQUE(product_id, hotel_id)
);
CREATE INDEX product_hotels_product_idx ON product_hotels(product_id);

-- +goose Down
DROP TABLE product_hotels;
DROP TABLE product_itinerary_days;
ALTER TABLE products DROP COLUMN default_kloter_id;
