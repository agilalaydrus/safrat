-- name: CreatePilgrimDocument :one
INSERT INTO pilgrim_documents (pilgrim_id, operator_id, doc_type, file_url, file_name, uploaded_by)
SELECT $1, $2, $3, $4, $5, $6
WHERE EXISTS (
  SELECT 1 FROM pilgrims p
  WHERE p.id = $1 AND p.operator_id = $2
    AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
)
RETURNING *;

-- name: ListPilgrimDocuments :many
SELECT pd.* FROM pilgrim_documents pd
JOIN pilgrims p ON p.id = pd.pilgrim_id
WHERE pd.pilgrim_id = $1
  AND pd.operator_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY pd.created_at DESC;

-- name: DeletePilgrimDocument :exec
DELETE FROM pilgrim_documents pd
WHERE pd.id = $1 AND pd.operator_id = $2
  AND EXISTS (
    SELECT 1 FROM pilgrims p
    WHERE p.id = pd.pilgrim_id
      AND p.operator_id = pd.operator_id
      AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
  );

-- name: ListSeasonDocuments :many
SELECT pd.*, p.full_name AS pilgrim_name, p.passport_number AS passport_number
FROM pilgrim_documents pd
JOIN pilgrims p ON p.id = pd.pilgrim_id
WHERE pd.operator_id = $1 AND p.season_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY pd.created_at DESC;
