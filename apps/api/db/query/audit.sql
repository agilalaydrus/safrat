-- name: CreateAuditLog :exec
INSERT INTO audit_logs (operator_id, branch_id, user_id, action, entity_type, entity_id, metadata)
VALUES (
  $1,
  (SELECT bm.branch_id FROM branch_members bm
   WHERE bm.better_auth_user_id = $2 AND bm.operator_id = $1),
  $2, $3, $4, $5, jsonb_build_object('message', $6::text)
);
