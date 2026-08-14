-- name: CreateHotel :one
INSERT INTO hotels (operator_id, season_id, name, city, star_rating, address, check_in_date, check_out_date)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7::date, $8::date) RETURNING *;
-- name: GetHotel :one
SELECT * FROM hotels WHERE id = $1 AND operator_id = $2;
-- name: ListHotels :many
SELECT * FROM hotels WHERE operator_id = $1 AND season_id = $2 ORDER BY name;
-- name: DeleteHotel :exec
DELETE FROM hotels WHERE id = $1 AND operator_id = $2;
