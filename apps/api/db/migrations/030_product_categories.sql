-- +goose Up
-- Products now span more than Hajj/Umrah travel packages (equipment, roaming
-- data, phone/PPOB credit) — `type` (HAJJ/UMRAH) only makes sense for the
-- travel-package category, so it's relaxed to allow '' for the others.
ALTER TABLE products DROP CONSTRAINT products_type_check;
ALTER TABLE products ALTER COLUMN type SET DEFAULT '';
ALTER TABLE products ADD CONSTRAINT products_type_check CHECK (type = '' OR type IN ('HAJJ','UMRAH'));
ALTER TABLE products ADD COLUMN category TEXT NOT NULL DEFAULT 'TRAVEL_PACKAGE'
  CHECK (category IN ('TRAVEL_PACKAGE','EQUIPMENT','ROAMING_DATA','PPOB_CREDIT'));
CREATE INDEX products_category_idx ON products(operator_id, category);

-- +goose Down
DROP INDEX IF EXISTS products_category_idx;
ALTER TABLE products DROP COLUMN category;
ALTER TABLE products DROP CONSTRAINT products_type_check;
UPDATE products SET type = 'HAJJ' WHERE type = '';
ALTER TABLE products ALTER COLUMN type SET DEFAULT 'HAJJ';
ALTER TABLE products ADD CONSTRAINT products_type_check CHECK (type IN ('HAJJ','UMRAH'));
