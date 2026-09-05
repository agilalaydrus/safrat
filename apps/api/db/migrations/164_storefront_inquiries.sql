-- +goose Up

-- K2.5 (TUGAS-CORONG.md): crm_leads.source/campaign is typed by hand today.
-- crm_leads itself can't be the public write target — CreateLead's entitlement
-- trigger requires the Growth/Pro "crm" feature flag and created_by_user_id
-- references a real Better Auth staff user, neither of which a visitor
-- filling out a storefront form has. So public interest lands here first,
-- untouched by plan or staff identity, and a staff member converts it into a
-- crm_lead with one click — at which point source/campaign carry what the
-- visitor's own link said instead of what staff guessed from memory.
CREATE TABLE storefront_inquiries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  full_name TEXT NOT NULL CHECK (length(trim(full_name)) BETWEEN 2 AND 150),
  phone TEXT NOT NULL DEFAULT '' CHECK (length(phone) <= 40),
  email TEXT NOT NULL DEFAULT '' CHECK (length(email) <= 254),
  message TEXT NOT NULL DEFAULT '' CHECK (length(message) <= 2000),
  utm_source TEXT NOT NULL DEFAULT '' CHECK (length(utm_source) <= 80),
  utm_campaign TEXT NOT NULL DEFAULT '' CHECK (length(utm_campaign) <= 120),
  status TEXT NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'CONVERTED', 'DISMISSED')),
  converted_lead_id UUID REFERENCES crm_leads(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (length(trim(phone)) > 0 OR length(trim(email)) > 0),
  CHECK ((status = 'CONVERTED') = (converted_lead_id IS NOT NULL))
);

CREATE INDEX storefront_inquiries_inbox_idx ON storefront_inquiries (operator_id, status, created_at DESC);

CREATE TRIGGER storefront_inquiries_touch_updated_at
  BEFORE UPDATE ON storefront_inquiries
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS storefront_inquiries;
