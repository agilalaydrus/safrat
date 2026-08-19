-- +goose Up
CREATE TABLE pilgrim_documents (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  pilgrim_id   UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id  UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  doc_type     TEXT        NOT NULL CHECK (doc_type IN ('PASSPORT','PHOTO','VACCINE','OTHER')),
  file_url     TEXT        NOT NULL,
  file_name    TEXT        NOT NULL,
  uploaded_by  TEXT        NOT NULL DEFAULT 'operator',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX pilgrim_documents_pilgrim_idx ON pilgrim_documents(pilgrim_id);

-- +goose Down
DROP TABLE IF EXISTS pilgrim_documents;
