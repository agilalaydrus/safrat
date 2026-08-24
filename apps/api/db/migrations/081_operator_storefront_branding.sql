-- +goose Up
-- Storefront fields are deliberately kept on operators: every tenant owns one
-- public landing page and these values change together with its public profile.
ALTER TABLE operators
  ADD COLUMN IF NOT EXISTS brand_color     TEXT NOT NULL DEFAULT '#059669',
  ADD COLUMN IF NOT EXISTS hero_eyebrow    TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS hero_title      TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS hero_subtitle   TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS hero_image_url  TEXT NOT NULL DEFAULT '';

ALTER TABLE operators
  ADD CONSTRAINT operators_brand_color_hex
  CHECK (brand_color ~ '^#[0-9A-Fa-f]{6}$'),
  ADD CONSTRAINT operators_hero_eyebrow_length
  CHECK (char_length(hero_eyebrow) <= 80),
  ADD CONSTRAINT operators_hero_title_length
  CHECK (char_length(hero_title) <= 120),
  ADD CONSTRAINT operators_hero_subtitle_length
  CHECK (char_length(hero_subtitle) <= 240),
  ADD CONSTRAINT operators_hero_image_url_length
  CHECK (char_length(hero_image_url) <= 2048);

-- +goose Down
ALTER TABLE operators
  DROP CONSTRAINT IF EXISTS operators_hero_image_url_length,
  DROP CONSTRAINT IF EXISTS operators_hero_subtitle_length,
  DROP CONSTRAINT IF EXISTS operators_hero_title_length,
  DROP CONSTRAINT IF EXISTS operators_hero_eyebrow_length,
  DROP CONSTRAINT IF EXISTS operators_brand_color_hex;

ALTER TABLE operators
  DROP COLUMN IF EXISTS hero_image_url,
  DROP COLUMN IF EXISTS hero_subtitle,
  DROP COLUMN IF EXISTS hero_title,
  DROP COLUMN IF EXISTS hero_eyebrow,
  DROP COLUMN IF EXISTS brand_color;
