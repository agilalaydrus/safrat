-- +goose Up
ALTER TABLE kloters
  ADD COLUMN status TEXT NOT NULL DEFAULT 'DRAFT',
  ADD COLUMN notes TEXT NOT NULL DEFAULT '';

ALTER TABLE kloters ADD CONSTRAINT kloters_status_check
  CHECK (status IN ('DRAFT','CONFIRMED','DEPARTED','IN_SAUDI','DEPARTED_SAUDI','COMPLETED'));

-- +goose Down
ALTER TABLE kloters DROP CONSTRAINT kloters_status_check;
ALTER TABLE kloters DROP COLUMN status;
ALTER TABLE kloters DROP COLUMN notes;
