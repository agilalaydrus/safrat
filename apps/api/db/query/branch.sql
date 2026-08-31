-- name: GetStaffBranchScope :one
SELECT branch_id
FROM branch_members
WHERE better_auth_user_id = $1
  AND operator_id = $2;

-- name: ListBranches :many
SELECT b.*, COALESCE(m.better_auth_user_id, '') AS head_user_id
FROM branches b
LEFT JOIN branch_members m ON m.branch_id = b.id
WHERE b.operator_id = $1
  AND ($2::boolean OR b.is_active)
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR b.id = sqlc.narg(branch_scope)::uuid)
ORDER BY b.name;

-- name: CreateBranch :one
INSERT INTO branches (operator_id, name, city, target_pilgrims, target_revenue_idr, phone, bank_name, account_number, account_holder)
SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9
WHERE sqlc.narg(branch_scope)::uuid IS NULL
RETURNING *;

-- name: UpdateBranch :one
UPDATE branches
SET name=$3, city=$4, target_pilgrims=$5, target_revenue_idr=$6, phone=$7,
    bank_name=$8, account_number=$9, account_holder=$10, is_active=$11, updated_at=NOW()
WHERE id=$1 AND operator_id=$2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR id = sqlc.narg(branch_scope)::uuid)
RETURNING *;

-- name: AssignBranchHead :one
WITH target AS (
  SELECT b.id, b.operator_id FROM branches b
  WHERE b.id=$1 AND b.operator_id=$2 AND sqlc.narg(branch_scope)::uuid IS NULL
)
INSERT INTO branch_members (better_auth_user_id, branch_id, operator_id)
SELECT $3, id, operator_id FROM target
ON CONFLICT (branch_id) DO UPDATE SET better_auth_user_id=EXCLUDED.better_auth_user_id, operator_id=EXCLUDED.operator_id
RETURNING branch_id;

-- name: ClearBranchHead :exec
DELETE FROM branch_members
WHERE branch_id=$1 AND operator_id=$2
  AND sqlc.narg(branch_scope)::uuid IS NULL;
