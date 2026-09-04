package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PlanChangeRepository struct {
	pool *pgxpool.Pool
}

func NewPlanChangeRepository(pool *pgxpool.Pool) *PlanChangeRepository {
	return &PlanChangeRepository{pool: pool}
}

// PlanChangeOrderSnapshot is what ChangePlan needs to know about the order
// before touching it, read inside the same transaction that changes it.
type PlanChangeOrderSnapshot struct {
	OrderID               string
	PilgrimID             string
	AgentID               string
	Status                string
	FromProductID         string
	FromProductName       string
	FromRoomTier          string
	OldTotalIDR           int64
	OldAgentCommissionIDR int64
	PaidAmountIDR         int64
}

type ChangePlanInput struct {
	OperatorID            string
	OrderID               string
	ToProductID           string
	ToProductName         string
	ToRoomTier            string
	NewTotalIDR           int64
	NewUnitIDR            int64
	NewBaseIDR            int64
	NewOperatorMarkupIDR  int64
	NewAgentMarkupIDR     int64
	NewPlatformAmountIDR  int64
	NewOperatorAmountIDR  int64
	NewAgentCommissionIDR int64
	Reason                string
	ActorUserID           string
	IdempotencyKey        string
}

type ChangePlanResult struct {
	ChangeID       string
	OverpaymentIDR int64
	ShortfallIDR   int64
	CreditID       string
}

// LockOrderForPlanChange reads the one row a plan change needs, locked, so the
// price comparison happens against a row nobody else is mutating at the same
// moment — a payment settling concurrently must not be measured against a
// total that changes underneath it.
func (r *PlanChangeRepository) LockOrderForPlanChange(ctx context.Context, tx pgx.Tx, operatorID, orderID string) (PlanChangeOrderSnapshot, error) {
	snapshot := PlanChangeOrderSnapshot{}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return snapshot, apperror.ErrValidation
	}
	order, err := pgUUID(orderID)
	if err != nil {
		return snapshot, apperror.ErrValidation
	}
	// A PAID order with no recorded paid_amount_idr is not a PAID order that
	// received nothing — it is one marked paid by hand (MarkOrderPaidManually),
	// which asserts the full total was settled without recording the exact
	// bank figure. Falling back to the total rather than to zero is what keeps
	// a cash sale from looking like a total loss the moment somebody tries to
	// move its pilgrim to a different package.
	err = tx.QueryRow(ctx, `
		SELECT o.id::text, o.pilgrim_id::text, COALESCE(o.agent_id::text, ''), o.status,
		       o.product_id::text, p.name, COALESCE(o.room_tier, ''), o.total_price_idr, o.agent_commission_idr,
		       COALESCE(o.paid_amount_idr, o.total_price_idr)
		FROM orders o
		JOIN products p ON p.id = o.product_id
		WHERE o.id = $1 AND o.operator_id = $2
		FOR UPDATE OF o`, order, operator).
		Scan(&snapshot.OrderID, &snapshot.PilgrimID, &snapshot.AgentID, &snapshot.Status,
			&snapshot.FromProductID, &snapshot.FromProductName, &snapshot.FromRoomTier, &snapshot.OldTotalIDR,
			&snapshot.OldAgentCommissionIDR, &snapshot.PaidAmountIDR)
	if errors.Is(err, pgx.ErrNoRows) {
		return snapshot, apperror.ErrNotFound
	}
	if err != nil {
		return snapshot, databaseError(err)
	}
	return snapshot, nil
}

// ChangePlan writes the new order state, the change record, and — if the new
// package cost less than what was already paid — an open credit, all in the
// caller's transaction.
//
// The order's own price columns are overwritten to describe the package the
// pilgrim is actually on now; pilgrim_plan_changes is what keeps the old
// numbers from being lost, since paid_amount_idr describes money that really
// moved and must never be edited to match a different story.
func (r *PlanChangeRepository) ChangePlan(ctx context.Context, tx pgx.Tx, snapshot PlanChangeOrderSnapshot, input ChangePlanInput) (ChangePlanResult, error) {
	result := ChangePlanResult{}
	if len(strings.TrimSpace(input.Reason)) < 10 {
		return result, apperror.ErrValidation
	}

	var existing string
	err := tx.QueryRow(ctx, `SELECT id::text FROM pilgrim_plan_changes WHERE idempotency_key = $1`,
		input.IdempotencyKey).Scan(&existing)
	if err == nil {
		return result, apperror.ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, databaseError(err)
	}

	shortfall, overpayment := int64(0), int64(0)
	if input.NewTotalIDR > snapshot.PaidAmountIDR {
		shortfall = input.NewTotalIDR - snapshot.PaidAmountIDR
	} else if input.NewTotalIDR < snapshot.PaidAmountIDR {
		overpayment = snapshot.PaidAmountIDR - input.NewTotalIDR
	}

	var toRoomTier *string
	if input.ToRoomTier != "" {
		toRoomTier = &input.ToRoomTier
	}
	var fromRoomTier *string
	if snapshot.FromRoomTier != "" {
		fromRoomTier = &snapshot.FromRoomTier
	}

	if err := tx.QueryRow(ctx, `
		INSERT INTO pilgrim_plan_changes (
		  operator_id, pilgrim_id, order_id, from_product_id, from_product_name,
		  to_product_id, to_product_name, from_room_tier, to_room_tier,
		  old_total_idr, new_total_idr, paid_before_idr, shortfall_idr, overpayment_idr,
		  reason, changed_by, idempotency_key
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING id::text`,
		input.OperatorID, snapshot.PilgrimID, snapshot.OrderID, snapshot.FromProductID, snapshot.FromProductName,
		input.ToProductID, input.ToProductName, fromRoomTier, toRoomTier,
		snapshot.OldTotalIDR, input.NewTotalIDR, snapshot.PaidAmountIDR,
		shortfall, overpayment, strings.TrimSpace(input.Reason), input.ActorUserID, input.IdempotencyKey,
	).Scan(&result.ChangeID); err != nil {
		if IsUniqueViolation(err, "pilgrim_plan_changes_idempotency_key_key") {
			return result, apperror.ErrConflict
		}
		return result, databaseError(err)
	}

	var roomTierValue *string
	if input.ToRoomTier != "" {
		roomTierValue = &input.ToRoomTier
	}
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET
		  product_id = $2, room_tier = $3,
		  unit_price_idr = $4, base_price_idr = $5, operator_markup_idr = $6, agent_markup_idr = $7,
		  total_price_idr = $8, platform_amount_idr = $9, operator_amount_idr = $10, agent_commission_idr = $11,
		  updated_at = NOW()
		WHERE id = $1`,
		snapshot.OrderID, input.ToProductID, roomTierValue,
		input.NewUnitIDR, input.NewBaseIDR, input.NewOperatorMarkupIDR, input.NewAgentMarkupIDR,
		input.NewTotalIDR, input.NewPlatformAmountIDR, input.NewOperatorAmountIDR, input.NewAgentCommissionIDR); err != nil {
		return result, databaseError(err)
	}

	if overpayment > 0 {
		if err := tx.QueryRow(ctx, `
			INSERT INTO pilgrim_credits (operator_id, pilgrim_id, amount_idr, source, source_id, reason)
			VALUES ($1,$2,$3,'PLAN_CHANGE',$4,$5)
			RETURNING id::text`,
			input.OperatorID, snapshot.PilgrimID, overpayment, result.ChangeID,
			"Kelebihan bayar dari pindah paket "+snapshot.FromProductName+" ke "+input.ToProductName,
		).Scan(&result.CreditID); err != nil {
			return result, databaseError(err)
		}
	}
	result.ShortfallIDR = shortfall
	result.OverpaymentIDR = overpayment
	return result, nil
}

type PlanChangeRow struct {
	ID              string
	PilgrimID       string
	PilgrimName     string
	OrderID         string
	FromProductName string
	ToProductName   string
	OldTotalIDR     int64
	NewTotalIDR     int64
	OverpaymentIDR  int64
	ShortfallIDR    int64
	Reason          string
	ChangedBy       string
	CreatedAt       time.Time
}

func (r *PlanChangeRepository) ListForPilgrim(ctx context.Context, operatorID, pilgrimID string, limit int32) ([]PlanChangeRow, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	pilgrim, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT c.id::text, c.pilgrim_id::text, p.full_name, c.order_id::text,
		       c.from_product_name, c.to_product_name, c.old_total_idr, c.new_total_idr,
		       c.overpayment_idr, c.shortfall_idr,
		       COALESCE(NULLIF(u.email, ''), c.changed_by), c.reason, c.created_at
		FROM pilgrim_plan_changes c
		JOIN pilgrims p ON p.id = c.pilgrim_id
		LEFT JOIN "user" u ON u.id = c.changed_by
		WHERE c.operator_id = $1 AND c.pilgrim_id = $2
		ORDER BY c.created_at DESC LIMIT $3`, operator, pilgrim, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	changes := make([]PlanChangeRow, 0)
	for rows.Next() {
		var row PlanChangeRow
		if err := rows.Scan(&row.ID, &row.PilgrimID, &row.PilgrimName, &row.OrderID,
			&row.FromProductName, &row.ToProductName, &row.OldTotalIDR, &row.NewTotalIDR,
			&row.OverpaymentIDR, &row.ShortfallIDR, &row.ChangedBy, &row.Reason, &row.CreatedAt); err != nil {
			return nil, err
		}
		changes = append(changes, row)
	}
	return changes, rows.Err()
}

type PilgrimCreditRow struct {
	ID               string
	PilgrimID        string
	PilgrimName      string
	AmountIDR        int64
	Source           string
	Reason           string
	Status           string
	AppliedToOrderID string
	AppliedNote      string
	ResolvedBy       string
	CreatedAt        time.Time
	ResolvedAt       *time.Time
}

// ListCreditsForOperator answers the question a screen asks most: who is
// owed money right now. Open credits sort first because they are the ones
// somebody has to act on.
func (r *PlanChangeRepository) ListCreditsForOperator(ctx context.Context, operatorID, pilgrimID string, onlyOpen bool, limit int32) ([]PilgrimCreditRow, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
		SELECT c.id::text, c.pilgrim_id::text, p.full_name, c.amount_idr, c.source, c.reason, c.status,
		       COALESCE(c.applied_to_order_id::text, ''), c.applied_note,
		       COALESCE(NULLIF(u.email, ''), c.resolved_by, ''), c.created_at, c.resolved_at
		FROM pilgrim_credits c
		JOIN pilgrims p ON p.id = c.pilgrim_id
		LEFT JOIN "user" u ON u.id = c.resolved_by
		WHERE c.operator_id = $1`
	args := []any{operator}
	if onlyOpen {
		query += ` AND c.status = 'OPEN'`
	}
	if strings.TrimSpace(pilgrimID) != "" {
		pilgrim, err := pgUUID(pilgrimID)
		if err != nil {
			return nil, apperror.ErrValidation
		}
		args = append(args, pilgrim)
		query += fmt.Sprintf(` AND c.pilgrim_id = $%d`, len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(` ORDER BY (c.status = 'OPEN') DESC, c.created_at DESC LIMIT $%d`, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	credits := make([]PilgrimCreditRow, 0)
	for rows.Next() {
		var row PilgrimCreditRow
		if err := rows.Scan(&row.ID, &row.PilgrimID, &row.PilgrimName, &row.AmountIDR, &row.Source, &row.Reason,
			&row.Status, &row.AppliedToOrderID, &row.AppliedNote, &row.ResolvedBy, &row.CreatedAt, &row.ResolvedAt); err != nil {
			return nil, err
		}
		credits = append(credits, row)
	}
	return credits, rows.Err()
}

// ResolveCredit closes an open credit, either against a specific future order
// or as a straight refund. It is bookkeeping, not a payment: nothing here
// moves money or edits another order's own numbers — it records what staff
// did and where the credit went, which is what makes it findable later.
func (r *PlanChangeRepository) ResolveCredit(ctx context.Context, operatorID, creditID, status, appliedToOrderID, note, actorUserID string) error {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	credit, err := pgUUID(creditID)
	if err != nil {
		return apperror.ErrValidation
	}
	if status != "APPLIED" && status != "REFUNDED" {
		return apperror.ErrValidation
	}
	var order any
	if status == "APPLIED" {
		if !isUUIDString(appliedToOrderID) {
			return apperror.ErrValidation
		}
		parsed, err := pgUUID(appliedToOrderID)
		if err != nil {
			return apperror.ErrValidation
		}
		order = parsed
	}
	command, err := r.pool.Exec(ctx, `
		UPDATE pilgrim_credits
		SET status = $3, applied_to_order_id = $4, applied_note = $5, resolved_by = $6, resolved_at = NOW()
		WHERE id = $1 AND operator_id = $2 AND status = 'OPEN'`,
		credit, operator, status, order, strings.TrimSpace(note), actorUserID)
	if err != nil {
		return databaseError(err)
	}
	if command.RowsAffected() == 0 {
		// Either it does not belong to this operator, or somebody already
		// resolved it — both are "nothing to do here", not a fault, and a
		// second staff member closing the same credit twice must not succeed
		// twice.
		return apperror.ErrConflict
	}
	return nil
}

func isUUIDString(value string) bool {
	_, err := pgUUID(value)
	return err == nil
}
