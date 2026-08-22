-- name: UpdatePilgrimInsurance :one
UPDATE pilgrims
SET insurance_provider = $2, insurance_policy_no = $3, insurance_class = $4,
    blood_type = $5, chronic_conditions = $6, current_medications = $7,
    insurance_start_date = $8, insurance_end_date = $9,
    insurance_beneficiary_name = $10, insurance_beneficiary_relation = $11,
    updated_at = NOW()
WHERE id = $1 AND operator_id = $12
RETURNING *;

-- name: CreateInsuranceClaim :one
INSERT INTO insurance_claims (pilgrim_id, operator_id, claim_type, incident_date, description, claim_amount_idr, filed_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListInsuranceClaims :many
SELECT ic.*, p.full_name, p.passport_number, p.insurance_provider, p.insurance_policy_no
FROM insurance_claims ic
JOIN pilgrims p ON p.id = ic.pilgrim_id
WHERE ic.operator_id = $1
ORDER BY ic.created_at DESC;

-- name: UpdateInsuranceClaimStatus :one
UPDATE insurance_claims
SET status = $3, settled_amount_idr = $4
WHERE id = $1 AND operator_id = $2
RETURNING *;

-- name: GetInsuranceClaimExportData :one
-- All fields needed for a complete insurance claim document.
SELECT
  p.full_name, p.passport_number, p.date_of_birth, p.gender, p.nationality,
  p.phone, p.emergency_contact_name, p.emergency_contact_phone,
  p.blood_type, p.chronic_conditions, p.current_medications,
  p.insurance_provider, p.insurance_policy_no, p.insurance_class,
  p.insurance_start_date, p.insurance_end_date, p.insurance_beneficiary_name, p.insurance_beneficiary_relation,
  p.medical_notes,
  s.name AS season_name, s.start_date, s.end_date,
  o.name AS operator_name, o.license_number, o.phone AS operator_phone,
  ic.id, ic.claim_type, ic.incident_date, ic.description, ic.status,
  ic.claim_amount_idr, ic.settled_amount_idr, ic.filed_by, ic.created_at
FROM insurance_claims ic
JOIN pilgrims p  ON p.id  = ic.pilgrim_id
JOIN seasons  s  ON s.id  = p.season_id
JOIN operators o ON o.id  = ic.operator_id
WHERE ic.id = $1 AND ic.operator_id = $2;
