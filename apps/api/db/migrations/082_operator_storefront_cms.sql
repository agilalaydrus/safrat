-- +goose Up
CREATE TABLE operator_storefronts (
  operator_id        UUID PRIMARY KEY REFERENCES operators(id) ON DELETE CASCADE,
  draft              JSONB NOT NULL DEFAULT '{}'::jsonb,
  published          JSONB,
  draft_revision     BIGINT NOT NULL DEFAULT 1 CHECK (draft_revision > 0),
  published_revision BIGINT NOT NULL DEFAULT 0 CHECK (published_revision >= 0),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  published_at       TIMESTAMPTZ
);

INSERT INTO operator_storefronts (operator_id, draft, published, draft_revision, published_revision, published_at)
SELECT
  id,
  jsonb_build_object(
    'displayName', name,
    'logoUrl', logo_url,
    'description', description,
    'whatsappNumber', whatsapp_number,
    'website', website,
    'address', address,
    'city', city,
    'brandColor', brand_color,
    'heroEyebrow', hero_eyebrow,
    'heroTitle', hero_title,
    'heroSubtitle', hero_subtitle,
    'heroImageUrl', hero_image_url,
    'packages', jsonb_build_array(),
    'gallery', jsonb_build_array(),
    'testimonials', jsonb_build_array(),
    'faqs', jsonb_build_array()
  ),
  jsonb_build_object(
    'displayName', name,
    'logoUrl', logo_url,
    'description', description,
    'whatsappNumber', whatsapp_number,
    'website', website,
    'address', address,
    'city', city,
    'brandColor', brand_color,
    'heroEyebrow', hero_eyebrow,
    'heroTitle', hero_title,
    'heroSubtitle', hero_subtitle,
    'heroImageUrl', hero_image_url,
    'packages', jsonb_build_array(),
    'gallery', jsonb_build_array(),
    'testimonials', jsonb_build_array(),
    'faqs', jsonb_build_array()
  ),
  1,
  1,
  NOW()
FROM operators;

CREATE INDEX operator_storefronts_published_at_idx
  ON operator_storefronts (published_at DESC)
  WHERE published IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS operator_storefronts;
