-- +goose Up

-- Operator-defined template items per season.
CREATE TABLE checklist_templates (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id   UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  title       TEXT        NOT NULL,
  description TEXT        NOT NULL DEFAULT '',
  category    TEXT        NOT NULL DEFAULT 'DOCUMENT'
              CHECK (category IN ('DOCUMENT','MEDICAL','PAYMENT','PREPARATION','OTHER')),
  is_required BOOLEAN     NOT NULL DEFAULT true,
  sort_order  INTEGER     NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX checklist_templates_season_idx ON checklist_templates(season_id, sort_order);

-- Per-pilgrim checklist state.
CREATE TABLE pilgrim_checklist_items (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id     UUID        NOT NULL REFERENCES checklist_templates(id) ON DELETE CASCADE,
  pilgrim_id      UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  is_completed    BOOLEAN     NOT NULL DEFAULT false,
  completed_at    TIMESTAMPTZ,
  completed_by    TEXT        NOT NULL DEFAULT '',
  notes           TEXT        NOT NULL DEFAULT '',
  UNIQUE (template_id, pilgrim_id)
);

CREATE INDEX pilgrim_checklist_items_pilgrim_idx ON pilgrim_checklist_items(pilgrim_id);
CREATE INDEX pilgrim_checklist_items_operator_idx ON pilgrim_checklist_items(operator_id, template_id);

-- +goose Down
DROP TABLE IF EXISTS pilgrim_checklist_items;
DROP TABLE IF EXISTS checklist_templates;
