-- name: GetPilgrimInstallmentSubject :one
SELECT p.id, p.operator_id, p.season_id, p.branch_id, p.full_name
FROM pilgrims p
WHERE p.id = $1 AND p.operator_id = $2
  AND p.status = 'ACTIVE' AND NOT p.is_substituted
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR p.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: InsertInstallmentPlan :one
INSERT INTO installment_plans (
  operator_id, season_id, pilgrim_id, branch_id, scheme,
  gross_amount_idr, cash_bonus_idr, first_due_date,
  created_by_user_id, idempotency_key
)
VALUES ($1,$2,$3,sqlc.narg(branch_id)::uuid,$4,$5,$6,$7,$8,$9)
ON CONFLICT (operator_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetInstallmentPlanByIdempotency :one
SELECT * FROM installment_plans
WHERE operator_id = $1 AND idempotency_key = $2;

-- name: InsertInstallment :one
INSERT INTO installments (
  plan_id, operator_id, branch_id, installment_number, label, due_date, amount_due_idr
)
VALUES ($1,$2,sqlc.narg(branch_id)::uuid,$3,$4,$5,$6)
RETURNING *;

-- name: GetActiveInstallmentPlanByPilgrim :one
SELECT ip.*, p.full_name AS pilgrim_name
FROM installment_plans ip
JOIN pilgrims p ON p.id = ip.pilgrim_id
WHERE ip.operator_id = $1 AND ip.pilgrim_id = $2 AND ip.status = 'ACTIVE'
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR ip.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: GetInstallmentPlanByID :one
SELECT ip.*, p.full_name AS pilgrim_name
FROM installment_plans ip
JOIN pilgrims p ON p.id = ip.pilgrim_id
WHERE ip.operator_id = $1 AND ip.id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR ip.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: ListInstallmentsByPlan :many
SELECT i.*,
       COALESCE(SUM(pe.amount_idr), 0)::bigint AS paid_amount_idr,
       (i.amount_due_idr - COALESCE(SUM(pe.amount_idr), 0))::bigint AS outstanding_amount_idr,
       CASE
         WHEN COALESCE(SUM(pe.amount_idr), 0) >= i.amount_due_idr THEN 'PAID'
         WHEN i.due_date < CURRENT_DATE THEN 'OVERDUE'
         WHEN COALESCE(SUM(pe.amount_idr), 0) > 0 THEN 'PARTIAL'
         WHEN i.due_date = CURRENT_DATE THEN 'DUE'
         ELSE 'UPCOMING'
       END AS computed_status,
       CASE WHEN i.due_date < CURRENT_DATE AND COALESCE(SUM(pe.amount_idr), 0) < i.amount_due_idr
            THEN (CURRENT_DATE - i.due_date)::int ELSE 0 END AS days_overdue
FROM installments i
LEFT JOIN installment_payment_entries pe ON pe.installment_id = i.id
WHERE i.plan_id = $1 AND i.operator_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR i.branch_id = sqlc.narg(branch_scope)::uuid)
GROUP BY i.id
ORDER BY i.installment_number ASC;

-- name: ListInstallmentPaymentsByPlan :many
SELECT * FROM installment_payment_entries
WHERE plan_id = $1 AND operator_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY created_at DESC, id DESC;

-- name: ListInstallmentReceivables :many
WITH payment_totals AS (
  SELECT plan_id, COALESCE(SUM(amount_idr), 0)::bigint AS paid_amount_idr
  FROM installment_payment_entries
  GROUP BY plan_id
), overdue_plans AS (
  SELECT i.plan_id,
         BOOL_OR(i.due_date < CURRENT_DATE AND i.amount_due_idr > COALESCE(p.paid_idr, 0)) AS has_overdue
  FROM installments i
  LEFT JOIN (
    SELECT installment_id, SUM(amount_idr)::bigint AS paid_idr
    FROM installment_payment_entries GROUP BY installment_id
  ) p ON p.installment_id = i.id
  GROUP BY i.plan_id
), rows AS (
  SELECT ip.*, p.full_name AS pilgrim_name,
         COALESCE(pt.paid_amount_idr, 0)::bigint AS paid_amount_idr,
         (ip.payable_amount_idr - COALESCE(pt.paid_amount_idr, 0))::bigint AS outstanding_amount_idr,
         CASE
           WHEN COALESCE(pt.paid_amount_idr, 0) >= ip.payable_amount_idr THEN 'PAID'
           WHEN COALESCE(op.has_overdue, false) THEN 'OVERDUE'
           WHEN COALESCE(pt.paid_amount_idr, 0) > 0 THEN 'PARTIAL'
           ELSE 'UNPAID'
         END AS computed_status
  FROM installment_plans ip
  JOIN pilgrims p ON p.id = ip.pilgrim_id
  LEFT JOIN payment_totals pt ON pt.plan_id = ip.id
  LEFT JOIN overdue_plans op ON op.plan_id = ip.id
  WHERE ip.operator_id = $1 AND ip.season_id = $2 AND ip.status = 'ACTIVE'
    AND (sqlc.narg(branch_scope)::uuid IS NULL OR ip.branch_id = sqlc.narg(branch_scope)::uuid)
)
SELECT *, COUNT(*) OVER() AS total_count
FROM rows
WHERE ($3 = '' OR computed_status = $3)
  AND ($4 = '' OR pilgrim_name ILIKE '%' || $4 || '%')
ORDER BY
  CASE computed_status WHEN 'OVERDUE' THEN 0 WHEN 'PARTIAL' THEN 1 WHEN 'UNPAID' THEN 2 ELSE 3 END,
  created_at DESC
LIMIT $5 OFFSET $6;

-- name: CountInstallmentReceivables :one
WITH payment_totals AS (
  SELECT plan_id, COALESCE(SUM(amount_idr), 0)::bigint AS paid_amount_idr
  FROM installment_payment_entries
  GROUP BY plan_id
), overdue_plans AS (
  SELECT i.plan_id,
         BOOL_OR(i.due_date < CURRENT_DATE AND i.amount_due_idr > COALESCE(p.paid_idr, 0)) AS has_overdue
  FROM installments i
  LEFT JOIN (
    SELECT installment_id, SUM(amount_idr)::bigint AS paid_idr
    FROM installment_payment_entries GROUP BY installment_id
  ) p ON p.installment_id = i.id
  GROUP BY i.plan_id
), rows AS (
  SELECT p.full_name AS pilgrim_name,
         CASE
           WHEN COALESCE(pt.paid_amount_idr, 0) >= ip.payable_amount_idr THEN 'PAID'
           WHEN COALESCE(op.has_overdue, false) THEN 'OVERDUE'
           WHEN COALESCE(pt.paid_amount_idr, 0) > 0 THEN 'PARTIAL'
           ELSE 'UNPAID'
         END AS computed_status
  FROM installment_plans ip
  JOIN pilgrims p ON p.id = ip.pilgrim_id
  LEFT JOIN payment_totals pt ON pt.plan_id = ip.id
  LEFT JOIN overdue_plans op ON op.plan_id = ip.id
  WHERE ip.operator_id = $1 AND ip.season_id = $2 AND ip.status = 'ACTIVE'
    AND (sqlc.narg(branch_scope)::uuid IS NULL OR ip.branch_id = sqlc.narg(branch_scope)::uuid)
)
SELECT COUNT(*)::bigint
FROM rows
WHERE ($3 = '' OR computed_status = $3)
  AND ($4 = '' OR pilgrim_name ILIKE '%' || $4 || '%');

-- name: GetInstallmentReceivableStats :one
WITH installment_paid AS (
  SELECT i.id, i.plan_id, i.amount_due_idr, i.due_date,
         COALESCE(SUM(pe.amount_idr), 0)::bigint AS paid_idr
  FROM installments i
  JOIN installment_plans ip ON ip.id = i.plan_id AND ip.status = 'ACTIVE'
  LEFT JOIN installment_payment_entries pe ON pe.installment_id = i.id
  WHERE i.operator_id = $1
    AND ip.season_id = $2
    AND (sqlc.narg(branch_scope)::uuid IS NULL OR i.branch_id = sqlc.narg(branch_scope)::uuid)
  GROUP BY i.id
)
SELECT
  COALESCE(SUM(amount_due_idr - paid_idr), 0)::bigint AS total_receivable_idr,
  COALESCE(SUM(amount_due_idr - paid_idr) FILTER (
    WHERE due_date < CURRENT_DATE AND paid_idr < amount_due_idr
  ), 0)::bigint AS total_overdue_idr,
  COALESCE(SUM(amount_due_idr - paid_idr) FILTER (
    WHERE due_date BETWEEN CURRENT_DATE AND CURRENT_DATE + 7 AND paid_idr < amount_due_idr
  ), 0)::bigint AS due_next_7_days_idr,
  COALESCE(SUM(paid_idr), 0)::bigint AS total_paid_idr,
  COALESCE(SUM(amount_due_idr), 0)::bigint AS total_payable_idr,
  COALESCE(SUM(amount_due_idr - paid_idr) FILTER (
    WHERE due_date >= CURRENT_DATE AND paid_idr < amount_due_idr
  ), 0)::bigint AS aging_current_idr,
  COALESCE(SUM(amount_due_idr - paid_idr) FILTER (
    WHERE due_date < CURRENT_DATE AND due_date >= CURRENT_DATE - 30 AND paid_idr < amount_due_idr
  ), 0)::bigint AS aging_1_30_idr,
  COALESCE(SUM(amount_due_idr - paid_idr) FILTER (
    WHERE due_date < CURRENT_DATE - 30 AND due_date >= CURRENT_DATE - 60 AND paid_idr < amount_due_idr
  ), 0)::bigint AS aging_31_60_idr,
  COALESCE(SUM(amount_due_idr - paid_idr) FILTER (
    WHERE due_date < CURRENT_DATE - 60 AND due_date >= CURRENT_DATE - 90 AND paid_idr < amount_due_idr
  ), 0)::bigint AS aging_61_90_idr,
  COALESCE(SUM(amount_due_idr - paid_idr) FILTER (
    WHERE due_date < CURRENT_DATE - 90 AND paid_idr < amount_due_idr
  ), 0)::bigint AS aging_over_90_idr
FROM installment_paid;

-- name: InsertInstallmentPayment :one
INSERT INTO installment_payment_entries (
  plan_id, installment_id, operator_id, branch_id, kind, amount_idr,
  method, reference, note, original_payment_id, verified_by_user_id, idempotency_key
)
VALUES ($1,$2,$3,sqlc.narg(branch_id)::uuid,$4,$5,$6,$7,$8,sqlc.narg(original_payment_id)::uuid,$9,$10)
ON CONFLICT (operator_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetInstallmentPaymentByIdempotency :one
SELECT * FROM installment_payment_entries
WHERE operator_id = $1 AND idempotency_key = $2;

-- name: GetInstallmentPaymentForReversal :one
SELECT pe.*
FROM installment_payment_entries pe
WHERE pe.id = $1 AND pe.operator_id = $2 AND pe.kind = 'PAYMENT'
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR pe.branch_id = sqlc.narg(branch_scope)::uuid)
FOR SHARE;

-- name: GetInstallmentForPayment :one
SELECT i.*, ip.status AS plan_status
FROM installments i
JOIN installment_plans ip ON ip.id = i.plan_id
WHERE i.id = $1 AND i.operator_id = $2
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR i.branch_id = sqlc.narg(branch_scope)::uuid);

-- name: GetInstallmentReceiptDelivery :one
SELECT pe.id, pe.receipt_number, pe.amount_idr, pe.method, pe.reference, pe.created_at,
       p.full_name AS pilgrim_name, p.email AS pilgrim_email,
       op.name AS operator_name, op.email AS operator_email
FROM installment_payment_entries pe
JOIN installment_plans ip ON ip.id = pe.plan_id
JOIN pilgrims p ON p.id = ip.pilgrim_id
JOIN operators op ON op.id = pe.operator_id
WHERE pe.id = $1 AND pe.operator_id = $2 AND pe.kind = 'PAYMENT';

-- name: HasInstallmentPaymentReversal :one
SELECT EXISTS (
  SELECT 1 FROM installment_payment_entries
  WHERE operator_id = $1 AND original_payment_id = $2 AND kind = 'REVERSAL'
);

-- name: GetInstallmentReminderDelivery :one
SELECT ip.id, ip.payable_amount_idr, p.full_name AS pilgrim_name,
       p.email AS pilgrim_email, op.name AS operator_name, op.email AS operator_email,
       COALESCE((SELECT SUM(entry.amount_idr) FROM installment_payment_entries entry WHERE entry.plan_id = ip.id), 0)::bigint AS paid_amount_idr,
       MIN(i.due_date) FILTER (
         WHERE i.amount_due_idr > COALESCE(paid.paid_idr, 0)
       )::date AS next_due_date
FROM installment_plans ip
JOIN pilgrims p ON p.id = ip.pilgrim_id
JOIN operators op ON op.id = ip.operator_id
JOIN installments i ON i.plan_id = ip.id
LEFT JOIN (
  SELECT installment_id, SUM(amount_idr)::bigint AS paid_idr
  FROM installment_payment_entries GROUP BY installment_id
) paid ON paid.installment_id = i.id
WHERE ip.id = $1 AND ip.operator_id = $2 AND ip.status = 'ACTIVE'
GROUP BY ip.id, p.id, op.id;

-- name: ListDueInstallmentPlanIDs :many
SELECT DISTINCT ip.id
FROM installment_plans ip
JOIN installments i ON i.plan_id = ip.id
LEFT JOIN (
  SELECT installment_id, SUM(amount_idr)::bigint AS paid_idr
  FROM installment_payment_entries GROUP BY installment_id
) paid ON paid.installment_id = i.id
WHERE ip.operator_id = $1 AND ip.season_id = $2 AND ip.status = 'ACTIVE'
  AND i.amount_due_idr > COALESCE(paid.paid_idr, 0)
  AND i.due_date <= CURRENT_DATE + 7
  AND (sqlc.narg(branch_scope)::uuid IS NULL OR ip.branch_id = sqlc.narg(branch_scope)::uuid)
ORDER BY ip.id;
