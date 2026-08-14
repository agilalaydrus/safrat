-- +goose Up
ALTER TABLE operators
ALTER COLUMN country TYPE TEXT USING BTRIM(country);

-- +goose Down
ALTER TABLE operators
ALTER COLUMN country TYPE CHAR(2) USING country;
