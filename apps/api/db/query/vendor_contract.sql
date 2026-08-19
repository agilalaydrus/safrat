-- name: CreateVendorContract :one
INSERT INTO vendor_contracts (
  operator_id, season_id, vendor_name, vendor_type, contract_number,
  committed_units, confirmation_deadline,
  rate_per_unit_idr, deposit_amount_idr, notes, contact_name, contact_phone
) VALUES (
  $1, $2, $3, $4, $5,
  $6, $7,
  $8, $9, $10, $11, $12
) RETURNING *;

-- name: ListVendorContracts :many
SELECT * FROM vendor_contracts
WHERE operator_id = $1 AND season_id = $2
ORDER BY vendor_type, vendor_name;

-- name: UpdateVendorContract :one
UPDATE vendor_contracts
SET vendor_name = $3, confirmed_units = $4,
    confirmation_deadline = $5, status = $6,
    notes = $7, deposit_paid = $8, contact_name = $9,
    contact_phone = $10
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: DeleteVendorContract :exec
DELETE FROM vendor_contracts WHERE id = $1 AND operator_id = $2;

-- name: CreateContractEvent :one
INSERT INTO vendor_contract_events (contract_id, operator_id, event_type, description, recorded_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListContractEvents :many
SELECT * FROM vendor_contract_events
WHERE contract_id = $1 AND operator_id = $2
ORDER BY created_at DESC;

-- name: GetVendorSLAStatus :many
-- Summary of all contracts with SLA health check.
SELECT
  vc.*,
  CASE
    WHEN vc.confirmed_units >= vc.committed_units THEN 'ON_TRACK'
    WHEN vc.confirmation_deadline IS NOT NULL AND vc.confirmation_deadline < CURRENT_DATE AND vc.confirmed_units < vc.committed_units THEN 'OVERDUE'
    WHEN vc.confirmation_deadline IS NOT NULL AND vc.confirmation_deadline BETWEEN CURRENT_DATE AND CURRENT_DATE + 7 THEN 'AT_RISK'
    ELSE 'PENDING'
  END AS sla_health
FROM vendor_contracts vc
WHERE vc.operator_id = $1 AND vc.season_id = $2
ORDER BY sla_health DESC, vc.confirmation_deadline ASC;
