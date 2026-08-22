-- +goose Up
-- current_city stays a closed set (needed for the journey-status cascade
-- map in service/group.go) but real itineraries have much richer detail
-- than "which city" — city tours, ziarah stops, free time, etc. This is a
-- free-text mirror of the latest group_location_log.location, persisted on
-- the row so lists (Kloter Detail, Group Detail, admin dashboard) don't
-- need to join the log table just to show "what's happening right now".
ALTER TABLE groups ADD COLUMN current_activity TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE groups DROP COLUMN current_activity;
