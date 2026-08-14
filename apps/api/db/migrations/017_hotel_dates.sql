-- +goose Up
ALTER TABLE hotels
  ADD COLUMN check_in_date DATE,
  ADD COLUMN check_out_date DATE;

-- +goose Down
ALTER TABLE hotels DROP COLUMN check_in_date;
ALTER TABLE hotels DROP COLUMN check_out_date;
