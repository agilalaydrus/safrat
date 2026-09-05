-- name: CreateStorefrontInquiry :one
INSERT INTO storefront_inquiries (
  operator_id, full_name, phone, email, message, utm_source, utm_campaign
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListStorefrontInquiries :many
SELECT * FROM storefront_inquiries
WHERE operator_id = sqlc.arg(operator_id)
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
ORDER BY created_at DESC;

-- name: GetStorefrontInquiry :one
SELECT * FROM storefront_inquiries WHERE id = $1 AND operator_id = $2;

-- name: LockStorefrontInquiry :one
SELECT * FROM storefront_inquiries WHERE id = $1 AND operator_id = $2 FOR UPDATE;

-- name: MarkStorefrontInquiryConverted :exec
UPDATE storefront_inquiries SET status = 'CONVERTED', converted_lead_id = $3
WHERE id = $1 AND operator_id = $2;

-- name: MarkStorefrontInquiryDismissed :exec
UPDATE storefront_inquiries SET status = 'DISMISSED'
WHERE id = $1 AND operator_id = $2;
