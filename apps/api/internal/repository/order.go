package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type OrderRepository struct{ queries *db.Queries }

func NewOrderRepository(queries *db.Queries) *OrderRepository {
	return &OrderRepository{queries: queries}
}

// Create records the order with its commission split already computed
// (see service/order.go) — the split is frozen at creation time, not
// recomputed from the product later, so a subsequent product margin edit
// never rewrites a past order's numbers. agentID may be empty ("" means
// NULL, matching the pilgrims/movements NULLIF(...,”) convention used
// throughout this codebase).
// CreateOrderParams describes one order to create. A struct rather than a
// dozen positional arguments: two adjacent int64 amounts are trivially easy to
// swap at a call site and impossible to notice afterwards.
type CreateOrderParams struct {
	OperatorID string
	SeasonID   string
	PilgrimID  string
	ProductID  string
	// AgentID is the referrer who earns the commission, taken from the
	// pilgrim's referral, never from whoever placed the order.
	AgentID            string
	PlacedByAgentID    string
	Quantity           int32
	UnitPriceIDR       int64
	TotalPriceIDR      int64
	PlatformAmountIDR  int64
	OperatorAmountIDR  int64
	AgentCommissionIDR int64
	IdempotencyKey     string
}

// Create records an order, or returns the one already recorded under the same
// idempotency key. The second return value reports which happened, so a caller
// does not create a second payment invoice for an order that already has one.
func (r *OrderRepository) Create(ctx context.Context, params CreateOrderParams) (*domain.Order, bool, error) {
	opUUID, err := pgUUID(params.OperatorID)
	if err != nil {
		return nil, false, err
	}
	seasonUUID, err := pgUUID(params.SeasonID)
	if err != nil {
		return nil, false, err
	}
	pilgrimUUID, err := pgUUID(params.PilgrimID)
	if err != nil {
		return nil, false, err
	}
	productUUID, err := pgUUID(params.ProductID)
	if err != nil {
		return nil, false, err
	}
	order, err := r.queries.CreateOrder(ctx, db.CreateOrderParams{
		OperatorID: opUUID, SeasonID: seasonUUID, PilgrimID: pilgrimUUID, ProductID: productUUID,
		Column5: params.AgentID, Quantity: params.Quantity, UnitPriceIdr: params.UnitPriceIDR,
		TotalPriceIdr: params.TotalPriceIDR, PlatformAmountIdr: params.PlatformAmountIDR,
		OperatorAmountIdr: params.OperatorAmountIDR, AgentCommissionIdr: params.AgentCommissionIDR,
		IdempotencyKey: params.IdempotencyKey, Column13: params.PlacedByAgentID,
	})
	if err == nil {
		return toOrder(order), true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	// No row came back: this key already made an order. That order is what the
	// caller is asking for.
	existing, err := r.queries.GetOrderByIdempotencyKey(ctx, db.GetOrderByIdempotencyKeyParams{
		OperatorID: opUUID, IdempotencyKey: params.IdempotencyKey,
	})
	if err != nil {
		return nil, false, err
	}
	return toOrderFromRow(db.GetOrderRow(existing)), false, nil
}

// MarkPaidManually is the admin cash/bank-transfer counterpart to
// MarkPaidByInvoiceID — same PENDING->PAID guard, just triggered by an
// operator's attestation instead of a Xendit webhook.
func (r *OrderRepository) MarkPaidManually(ctx context.Context, operatorID, orderID string) (*domain.Order, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	orderUUID, err := pgUUID(orderID)
	if err != nil {
		return nil, err
	}
	order, err := r.queries.MarkOrderPaidManually(ctx, db.MarkOrderPaidManuallyParams{ID: orderUUID, OperatorID: opUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	return toOrder(order), nil
}

func (r *OrderRepository) SetXenditInvoice(ctx context.Context, orderID, invoiceID, invoiceURL string) error {
	orderUUID, err := pgUUID(orderID)
	if err != nil {
		return err
	}
	return r.queries.SetOrderXenditInvoice(ctx, db.SetOrderXenditInvoiceParams{ID: orderUUID, XenditInvoiceID: pgtype.Text{String: invoiceID, Valid: true}, XenditInvoiceUrl: pgtype.Text{String: invoiceURL, Valid: true}})
}

// MarkPaidByInvoiceID is called from the Xendit webhook handler — matches
// by invoice id (not order id, which Xendit's payload doesn't carry back
// directly) and only transitions PENDING -> PAID, so a duplicate/replayed
// webhook delivery is a harmless no-op (pgx.ErrNoRows), not a double-count.
//
// paidAmountIDR is stored with the settlement, so a settled order carries the
// amount that was actually received rather than only the amount that was owed.
func (r *OrderRepository) MarkPaidByInvoiceID(ctx context.Context, invoiceID string, paidAmountIDR int64) (*domain.Order, error) {
	order, err := r.queries.MarkOrderPaidByInvoiceID(ctx, db.MarkOrderPaidByInvoiceIDParams{
		XenditInvoiceID: pgtype.Text{String: invoiceID, Valid: true},
		PaidAmountIdr:   pgtype.Int8{Int64: paidAmountIDR, Valid: true},
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toOrder(order), nil
}

func (r *OrderRepository) MarkStatusByInvoiceID(ctx context.Context, invoiceID, status string) (*domain.Order, error) {
	order, err := r.queries.MarkOrderStatusByInvoiceID(ctx, db.MarkOrderStatusByInvoiceIDParams{XenditInvoiceID: pgtype.Text{String: invoiceID, Valid: true}, Status: status})
	if err != nil {
		return nil, databaseError(err)
	}
	return toOrder(order), nil
}

func (r *OrderRepository) Get(ctx context.Context, operatorID, orderID string) (*domain.Order, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	orderUUID, err := pgUUID(orderID)
	if err != nil {
		return nil, err
	}
	order, err := r.queries.GetOrder(ctx, db.GetOrderParams{ID: orderUUID, OperatorID: opUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	return toOrderFromRow(order), nil
}

func (r *OrderRepository) ListBySeason(ctx context.Context, operatorID, seasonID string, limit, offset int32) ([]*domain.Order, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	orders, err := r.queries.ListOrders(ctx, db.ListOrdersParams{OperatorID: opUUID, SeasonID: seasonUUID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Order, 0, len(orders))
	for _, order := range orders {
		result = append(result, toOrderFromListRow(order))
	}
	return result, nil
}

func (r *OrderRepository) CountBySeason(ctx context.Context, operatorID, seasonID string) (int64, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return 0, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return 0, err
	}
	return r.queries.CountOrdersBySeason(ctx, db.CountOrdersBySeasonParams{OperatorID: opUUID, SeasonID: seasonUUID})
}

func toOrder(o db.Order) *domain.Order {
	return &domain.Order{
		ID: uuid.UUID(o.ID.Bytes).String(), OperatorID: uuid.UUID(o.OperatorID.Bytes).String(), SeasonID: uuid.UUID(o.SeasonID.Bytes).String(),
		PilgrimID: uuid.UUID(o.PilgrimID.Bytes).String(), ProductID: uuid.UUID(o.ProductID.Bytes).String(), AgentID: nullableUUIDString(o.AgentID),
		Quantity: o.Quantity, UnitPriceIDR: o.UnitPriceIdr, TotalPriceIDR: o.TotalPriceIdr,
		PlatformAmountIDR: o.PlatformAmountIdr, OperatorAmountIDR: o.OperatorAmountIdr, AgentCommissionIDR: o.AgentCommissionIdr,
		Status: o.Status, XenditInvoiceID: o.XenditInvoiceID.String, XenditInvoiceURL: o.XenditInvoiceUrl.String,
		PaidAt: timestamptzPtr(o.PaidAt), CreatedAt: o.CreatedAt.Time,
	}
}

func toOrderFromRow(o db.GetOrderRow) *domain.Order {
	base := toOrder(db.Order{
		ID: o.ID, OperatorID: o.OperatorID, SeasonID: o.SeasonID, PilgrimID: o.PilgrimID, ProductID: o.ProductID, AgentID: o.AgentID,
		Quantity: o.Quantity, UnitPriceIdr: o.UnitPriceIdr, TotalPriceIdr: o.TotalPriceIdr,
		PlatformAmountIdr: o.PlatformAmountIdr, OperatorAmountIdr: o.OperatorAmountIdr, AgentCommissionIdr: o.AgentCommissionIdr,
		Status: o.Status, XenditInvoiceID: o.XenditInvoiceID, XenditInvoiceUrl: o.XenditInvoiceUrl, PaidAt: o.PaidAt, CreatedAt: o.CreatedAt,
	})
	base.PilgrimName = o.PilgrimName
	base.ProductName = o.ProductName
	base.AgentName = o.AgentName.String
	return base
}

func toOrderFromListRow(o db.ListOrdersRow) *domain.Order {
	base := toOrder(db.Order{
		ID: o.ID, OperatorID: o.OperatorID, SeasonID: o.SeasonID, PilgrimID: o.PilgrimID, ProductID: o.ProductID, AgentID: o.AgentID,
		Quantity: o.Quantity, UnitPriceIdr: o.UnitPriceIdr, TotalPriceIdr: o.TotalPriceIdr,
		PlatformAmountIdr: o.PlatformAmountIdr, OperatorAmountIdr: o.OperatorAmountIdr, AgentCommissionIdr: o.AgentCommissionIdr,
		Status: o.Status, XenditInvoiceID: o.XenditInvoiceID, XenditInvoiceUrl: o.XenditInvoiceUrl, PaidAt: o.PaidAt, CreatedAt: o.CreatedAt,
	})
	base.PilgrimName = o.PilgrimName
	base.ProductName = o.ProductName
	base.AgentName = o.AgentName.String
	return base
}

// ListTransactionsForPilgrim returns a jamaah's own order history, refunds
// included. A refunded order stays in the list: somebody whose money was
// returned needs to see that it was, not find the transaction missing.
func (r *OrderRepository) ListTransactionsForPilgrim(ctx context.Context, pilgrimID string) ([]*domain.PilgrimTransaction, error) {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListTransactionsForPilgrim(ctx, pilgrimUUID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.PilgrimTransaction, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.PilgrimTransaction{
			OrderID: uuid.UUID(row.ID.Bytes).String(), ProductName: row.ProductName,
			Quantity: row.Quantity, AmountIDR: row.TotalPriceIdr, Status: row.Status,
			CreatedAt: row.CreatedAt.Time, PaidAt: timestamptzPtr(row.PaidAt),
			RefundedIDR: row.RefundedIdr, RefundedAt: timestamptzPtr(row.RefundedAt),
			RefundReason: row.RefundReason, CheckoutURL: row.XenditInvoiceUrl.String,
		})
	}
	return result, nil
}

// PilgrimTransactionTotals is what the operator has received and kept from this
// jamaah, and what came back.
func (r *OrderRepository) PilgrimTransactionTotals(ctx context.Context, pilgrimID string) (paid, refunded int64, err error) {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return 0, 0, apperror.ErrValidation
	}
	totals, err := r.queries.GetPilgrimTransactionTotals(ctx, pilgrimUUID)
	if err != nil {
		return 0, 0, err
	}
	return totals.TotalPaidIdr, totals.TotalRefundedIdr, nil
}

// GetByInvoiceID finds an order by the gateway's invoice id, for validating a
// webhook before acting on it.
func (r *OrderRepository) GetByInvoiceID(ctx context.Context, invoiceID string) (*domain.Order, error) {
	order, err := r.queries.GetOrderByInvoiceID(ctx, pgtype.Text{String: invoiceID, Valid: true})
	if err != nil {
		return nil, err
	}
	return toOrder(order), nil
}

// HoldByInvoiceID parks an order whose payment did not match what was owed.
func (r *OrderRepository) HoldByInvoiceID(ctx context.Context, invoiceID string, paidAmountIDR int64, reason string) (*domain.Order, error) {
	order, err := r.queries.HoldOrderByInvoiceID(ctx, db.HoldOrderByInvoiceIDParams{
		XenditInvoiceID: pgtype.Text{String: invoiceID, Valid: true},
		PaidAmountIdr:   pgtype.Int8{Int64: paidAmountIDR, Valid: true},
		HeldReason:      reason,
	})
	if err != nil {
		return nil, err
	}
	return toOrder(order), nil
}

// AwaitingSettlement is one transaction still waiting on the gateway.
type AwaitingSettlement struct {
	OrderID   string
	InvoiceID string
}

// ListAwaitingSettlement returns orders that have been pending long enough
// that a webhook should already have arrived.
func (r *OrderRepository) ListAwaitingSettlement(ctx context.Context, graceMinutes int32, limit int32) ([]AwaitingSettlement, error) {
	rows, err := r.queries.ListOrdersAwaitingSettlement(ctx, db.ListOrdersAwaitingSettlementParams{
		Column1: graceMinutes, Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	result := make([]AwaitingSettlement, 0, len(rows))
	for _, row := range rows {
		result = append(result, AwaitingSettlement{
			OrderID:   uuid.UUID(row.ID.Bytes).String(),
			InvoiceID: row.XenditInvoiceID.String,
		})
	}
	return result, nil
}
