-- name: CreatePilgrimDocument :one
INSERT INTO pilgrim_documents (pilgrim_id, operator_id, doc_type, file_url, file_name, uploaded_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPilgrimDocuments :many
SELECT * FROM pilgrim_documents
WHERE pilgrim_id = $1
ORDER BY created_at DESC;

-- name: DeletePilgrimDocument :exec
DELETE FROM pilgrim_documents
WHERE id = $1 AND operator_id = $2;

-- name: ListSeasonDocuments :many
SELECT pd.*, p.full_name AS pilgrim_name, p.passport_number AS passport_number
FROM pilgrim_documents pd
JOIN pilgrims p ON p.id = pd.pilgrim_id
WHERE pd.operator_id = $1 AND p.season_id = $2
ORDER BY pd.created_at DESC;
