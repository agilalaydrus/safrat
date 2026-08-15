-- name: CreateAuditLog :exec
INSERT INTO audit_logs (operator_id, user_id, action, entity_type, entity_id, metadata)
VALUES ($1, $2, $3, $4, $5, jsonb_build_object('message', $6::text));
