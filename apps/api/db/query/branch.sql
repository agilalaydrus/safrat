-- name: GetStaffBranchScope :one
SELECT branch_id
FROM branch_members
WHERE better_auth_user_id = $1
  AND operator_id = $2;
