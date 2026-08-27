package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository struct {
	queries *db.Queries
	// pool is needed because creating an order and consuming the buyer's daily
	// limit have to be one transaction. Consuming first and inserting second
	// leaks headroom whenever the insert turns out to be a replay; inserting
	// first and consuming second creates an order the limit should have
	// refused.
	pool *pgxpool.Pool
}

func NewOrderRepository(queries *db.Queries, pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{queries: queries, pool: pool}
}

// Create records the order with every price level and settlement amount
// already computed (see service/pricing.go). They are frozen at creation time,
// so a later base-price or markup edit never rewrites a past transaction.
// Agent identifiers may be empty ("" means NULL, matching the convention used
// throughout this codebase).
// CreateOrderParams describes one order to create. A struct rather than a
// dozen positional arguments: two adjacent int64 amounts are trivially easy to
// swap at a call site and impossible to notice afterwards.
type CreateOrderParams struct {
	OperatorID   string
	SeasonID     string
	PilgrimID    string
	BuyerAgentID string
	BuyerKind    string
	ProductID    string
	// AgentID is the referrer who earns the commission, taken from the
	// pilgrim's referral, never from whoever placed the order.
	AgentID            string
	PlacedByAgentID    string
	Quantity           int32
	UnitPriceIDR       int64
	BasePriceIDR       int64
	OperatorMarkupIDR  int64
	AgentMarkupIDR     int64
	TotalPriceIDR      int64
	PlatformAmountIDR  int64
	OperatorAmountIDR  int64
	AgentCommissionIDR int64
	IdempotencyKey     string
	Destination        string

	// CountsTowardDailyLimit is true only for the digital categories the cap
	// applies to. Decided by the service, because "which products are digital"
	// is a business rule and the repository should not hold a second copy of
	// it that can drift.
	CountsTowardDailyLimit bool
	// SpendDate is the buyer's calendar day in Asia/Jakarta. Passed in rather
	// than computed here so the whole system agrees on when a day starts, and
	// so a test can pin it.
	SpendDate time.Time
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
	pilgrimUUID, err := optionalUUID(params.PilgrimID)
	if err != nil {
		return nil, false, err
	}
	buyerAgentUUID, err := optionalUUID(params.BuyerAgentID)
	if err != nil {
		return nil, false, err
	}
	productUUID, err := pgUUID(params.ProductID)
	if err != nil {
		return nil, false, err
	}
	// Digital purchases consume the buyer's daily limit, and that has to happen
	// in the same transaction as the insert. Anything else has a window: consume
	// first and a replayed idempotency key leaks headroom that never comes back
	// until midnight; insert first and an order exists that the limit should
	// have refused.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.queries.WithTx(tx)

	var spendStamp pgtype.Date
	if params.CountsTowardDailyLimit {
		spendStamp = pgtype.Date{Time: params.SpendDate, Valid: true}
	}

	order, err := qtx.CreateOrder(ctx, db.CreateOrderParams{
		OperatorID: opUUID, SeasonID: seasonUUID, PilgrimID: pilgrimUUID, ProductID: productUUID,
		AgentID: params.AgentID, Quantity: params.Quantity, UnitPriceIdr: params.UnitPriceIDR,
		TotalPriceIdr: params.TotalPriceIDR, PlatformAmountIdr: params.PlatformAmountIDR,
		OperatorAmountIdr: params.OperatorAmountIDR, AgentCommissionIdr: params.AgentCommissionIDR,
		IdempotencyKey: params.IdempotencyKey, PlacedByAgentID: params.PlacedByAgentID,
		BuyerAgentID: buyerAgentUUID, BuyerKind: params.BuyerKind,
		BasePriceIdr: params.BasePriceIDR, OperatorMarkupIdr: params.OperatorMarkupIDR,
		AgentMarkupIdr:        params.AgentMarkupIDR,
		Destination:           strings.TrimSpace(params.Destination),
		DigitalSpendCountedOn: spendStamp,
	})
	if err == nil {
		// Only a genuinely new order spends. A replay reaches the branch below
		// and never gets here, which is what keeps a retried checkout from
		// consuming the limit twice.
		if params.CountsTowardDailyLimit {
			if err := qtx.ConsumeDailyDigitalSpend(ctx, db.ConsumeDailyDigitalSpendParams{
				BuyerKind: params.BuyerKind,
				BuyerID:   buyerIdentity(pilgrimUUID, buyerAgentUUID),
				SpendDate: spendStamp,
				AmountIdr: params.TotalPriceIDR,
			}); err != nil {
				if isCheckViolation(err, "daily_digital_spend_within_limit") {
					return nil, false, apperror.ErrDailyLimitExceeded
				}
				return nil, false, err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		return toOrder(order), true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	// No row came back: this key already made an order. That order is what the
	// caller is asking for, and it already spent its share of the limit.
	existing, err := qtx.GetOrderByIdempotencyKey(ctx, db.GetOrderByIdempotencyKeyParams{
		OperatorID: opUUID, IdempotencyKey: params.IdempotencyKey,
	})
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return toOrderFromRow(db.GetOrderRow(existing)), false, nil
}

// buyerIdentity picks whichever buyer column is set. The CHECK on orders
// guarantees exactly one is, so this cannot silently return a zero UUID for a
// valid order.
// isCheckViolation reports a specific CHECK constraint failing, which is how
// the daily limit refuses. Named rather than matched on message text, so a
// different constraint failing is never mistaken for the limit.
func isCheckViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23514" && pgErr.ConstraintName == constraint
}

func buyerIdentity(pilgrim, agent pgtype.UUID) pgtype.UUID {
	if pilgrim.Valid {
		return pilgrim
	}
	return agent
}

// ReleaseDigitalSpend gives an order's daily headroom back when it stops
// holding value — refunded, failed, expired or cancelled.
//
// Safe to call for any order: one that never counted carries no stamp, and the
// statement matches nothing. That matters because it is reached from three
// settlement paths and a reconciliation sweep, and coordinating "who releases"
// between them would be one more thing to get wrong.
func (r *OrderRepository) ReleaseDigitalSpend(ctx context.Context, orderID string) error {
	id, err := pgUUID(orderID)
	if err != nil {
		return err
	}
	return r.queries.ReleaseOrderDigitalSpend(ctx, id)
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
		PilgrimID: nullableUUIDString(o.PilgrimID), BuyerAgentID: nullableUUIDString(o.BuyerAgentID), BuyerKind: o.BuyerKind,
		ProductID: uuid.UUID(o.ProductID.Bytes).String(), AgentID: nullableUUIDString(o.AgentID),
		Quantity: o.Quantity, UnitPriceIDR: o.UnitPriceIdr,
		BasePriceIDR: o.BasePriceIdr, OperatorMarkupIDR: o.OperatorMarkupIdr, AgentMarkupIDR: o.AgentMarkupIdr,
		TotalPriceIDR:     o.TotalPriceIdr,
		PlatformAmountIDR: o.PlatformAmountIdr, OperatorAmountIDR: o.OperatorAmountIdr, AgentCommissionIDR: o.AgentCommissionIdr,
		Status: o.Status, HeldReason: o.HeldReason, ReceiptNumber: o.ReceiptNumber, Destination: o.Destination, XenditInvoiceID: o.XenditInvoiceID.String, XenditInvoiceURL: o.XenditInvoiceUrl.String,
		PaidAmountIDR: int8Ptr(o.PaidAmountIdr),
		PaidAt:        timestamptzPtr(o.PaidAt), CreatedAt: o.CreatedAt.Time,
	}
}

func toOrderFromRow(o db.GetOrderRow) *domain.Order {
	base := toOrder(db.Order{
		ID: o.ID, OperatorID: o.OperatorID, SeasonID: o.SeasonID, PilgrimID: o.PilgrimID, BuyerAgentID: o.BuyerAgentID, BuyerKind: o.BuyerKind,
		ProductID: o.ProductID, AgentID: o.AgentID,
		Quantity: o.Quantity, UnitPriceIdr: o.UnitPriceIdr, TotalPriceIdr: o.TotalPriceIdr,
		BasePriceIdr: o.BasePriceIdr, OperatorMarkupIdr: o.OperatorMarkupIdr, AgentMarkupIdr: o.AgentMarkupIdr,
		PlatformAmountIdr: o.PlatformAmountIdr, OperatorAmountIdr: o.OperatorAmountIdr, AgentCommissionIdr: o.AgentCommissionIdr,
		Status: o.Status, HeldReason: o.HeldReason, PaidAmountIdr: o.PaidAmountIdr, ReceiptNumber: o.ReceiptNumber,
		Destination: o.Destination, XenditInvoiceID: o.XenditInvoiceID, XenditInvoiceUrl: o.XenditInvoiceUrl, PaidAt: o.PaidAt, CreatedAt: o.CreatedAt,
	})
	base.PilgrimName = o.PilgrimName
	base.BuyerName = o.BuyerName
	base.ProductName = o.ProductName
	base.AgentName = o.AgentName.String
	return base
}

func toOrderFromListRow(o db.ListOrdersRow) *domain.Order {
	base := toOrder(db.Order{
		ID: o.ID, OperatorID: o.OperatorID, SeasonID: o.SeasonID, PilgrimID: o.PilgrimID, BuyerAgentID: o.BuyerAgentID, BuyerKind: o.BuyerKind,
		ProductID: o.ProductID, AgentID: o.AgentID,
		Quantity: o.Quantity, UnitPriceIdr: o.UnitPriceIdr, TotalPriceIdr: o.TotalPriceIdr,
		BasePriceIdr: o.BasePriceIdr, OperatorMarkupIdr: o.OperatorMarkupIdr, AgentMarkupIdr: o.AgentMarkupIdr,
		PlatformAmountIdr: o.PlatformAmountIdr, OperatorAmountIdr: o.OperatorAmountIdr, AgentCommissionIdr: o.AgentCommissionIdr,
		Status: o.Status, HeldReason: o.HeldReason, PaidAmountIdr: o.PaidAmountIdr, ReceiptNumber: o.ReceiptNumber,
		Destination: o.Destination, XenditInvoiceID: o.XenditInvoiceID, XenditInvoiceUrl: o.XenditInvoiceUrl, PaidAt: o.PaidAt, CreatedAt: o.CreatedAt,
	})
	base.PilgrimName = o.PilgrimName
	base.BuyerName = o.BuyerName
	base.ProductName = o.ProductName
	base.AgentName = o.AgentName.String
	return base
}

// ListForBuyerAgent is the signed-in agent/Muttawwif buyer's own purchase
// history. It is intentionally distinct from their referral recap: the latter
// explains commission, while these are transactions they personally paid for.
func (r *OrderRepository) ListForBuyerAgent(ctx context.Context, operatorID, seasonID, agentID string, limit, offset int32) ([]*domain.Order, int64, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, 0, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, 0, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListOrdersForBuyerAgent(ctx, db.ListOrdersForBuyerAgentParams{
		OperatorID: opUUID, SeasonID: seasonUUID, BuyerAgentID: agentUUID,
		Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	result := make([]*domain.Order, 0, len(rows))
	for _, row := range rows {
		base := toOrder(db.Order{
			ID: row.ID, OperatorID: row.OperatorID, SeasonID: row.SeasonID, PilgrimID: row.PilgrimID,
			BuyerAgentID: row.BuyerAgentID, BuyerKind: row.BuyerKind, ProductID: row.ProductID, AgentID: row.AgentID,
			Quantity: row.Quantity, UnitPriceIdr: row.UnitPriceIdr, TotalPriceIdr: row.TotalPriceIdr,
			BasePriceIdr: row.BasePriceIdr, OperatorMarkupIdr: row.OperatorMarkupIdr, AgentMarkupIdr: row.AgentMarkupIdr,
			PlatformAmountIdr: row.PlatformAmountIdr, OperatorAmountIdr: row.OperatorAmountIdr,
			AgentCommissionIdr: row.AgentCommissionIdr, Status: row.Status, HeldReason: row.HeldReason,
			PaidAmountIdr: row.PaidAmountIdr, ReceiptNumber: row.ReceiptNumber, Destination: row.Destination,
			XenditInvoiceID: row.XenditInvoiceID, XenditInvoiceUrl: row.XenditInvoiceUrl,
			PaidAt: row.PaidAt, CreatedAt: row.CreatedAt,
		})
		base.BuyerName = row.BuyerName
		base.ProductName = row.ProductName
		base.AgentName = row.AgentName.String
		result = append(result, base)
	}
	count, err := r.queries.CountOrdersForBuyerAgent(ctx, db.CountOrdersForBuyerAgentParams{
		OperatorID: opUUID, SeasonID: seasonUUID, BuyerAgentID: agentUUID,
	})
	if err != nil {
		return nil, 0, err
	}
	return result, count, nil
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
			ReceiptNumber: row.ReceiptNumber, OperatorName: row.OperatorName,
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

// ResolveHeld moves a held order to its final state. Only a HELD order moves,
// so a repeated click resolves nothing a second time.
func (r *OrderRepository) ResolveHeld(ctx context.Context, operatorID, orderID string, accept bool) (*domain.Order, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	orderUUID, err := pgUUID(orderID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	var order db.Order
	if accept {
		order, err = r.queries.ResolveHeldOrderToPaid(ctx, db.ResolveHeldOrderToPaidParams{ID: orderUUID, OperatorID: opUUID})
	} else {
		order, err = r.queries.ResolveHeldOrderToFailed(ctx, db.ResolveHeldOrderToFailedParams{ID: orderUUID, OperatorID: opUUID})
	}
	if err != nil {
		return nil, databaseError(err)
	}
	return toOrder(order), nil
}

// int8Ptr turns a nullable bigint into a pointer, so "no payment reported yet"
// stays distinguishable from "zero was reported".
func int8Ptr(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	amount := value.Int64
	return &amount
}

// GetAny reads an order without operator scoping.
//
// Every other read here is tenant-scoped and must stay that way. This one
// exists for the supplier callback path, which is authenticated by a supplier's
// token and has no operator in hand — the operator is what it is looking up.
// Not for use anywhere a caller's identity is available.
func (r *OrderRepository) GetAny(ctx context.Context, orderID string) (*domain.Order, error) {
	id, err := pgUUID(orderID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	order, err := r.queries.GetOrderByIdempotencyKeyAny(ctx, id)
	if err != nil {
		return nil, databaseError(err)
	}
	return toOrder(order), nil
}

// MarkGatewayChecked records that these orders were asked about, whatever the
// answer was — including none.
//
// An order the gateway could not be reached about must still take its turn at
// the back of the queue, or one unreachable invoice would monopolise every
// sweep and starve the rest.
func (r *OrderRepository) MarkGatewayChecked(ctx context.Context, orderIDs []string) error {
	if len(orderIDs) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		parsed, err := pgUUID(orderID)
		if err != nil {
			continue
		}
		ids = append(ids, parsed)
	}
	if len(ids) == 0 {
		return nil
	}
	return r.queries.MarkOrdersGatewayChecked(ctx, ids)
}

// DailySpend is what an account has spent on digital products today and the
// cap in force for them.
type DailySpend struct {
	TotalIDR int64
	LimitIDR int64
}

// DailyDigitalSpend reads a buyer's day. A buyer who has bought nothing has no
// row, which is not an error — it is a day with the full limit still on it.
func (r *OrderRepository) DailyDigitalSpend(ctx context.Context, buyerKind, buyerID string, day time.Time) (DailySpend, error) {
	id, err := pgUUID(buyerID)
	if err != nil {
		return DailySpend{}, err
	}
	row, err := r.queries.GetDailyDigitalSpend(ctx, db.GetDailyDigitalSpendParams{
		BuyerKind: buyerKind, BuyerID: id,
		SpendDate: pgtype.Date{Time: day, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DailySpend{TotalIDR: 0, LimitIDR: defaultDailyDigitalLimitIDR}, nil
	}
	if err != nil {
		return DailySpend{}, err
	}
	return DailySpend{TotalIDR: row.TotalIdr, LimitIDR: row.LimitIdr}, nil
}

// defaultDailyDigitalLimitIDR mirrors the column default. Only used to
// describe a day that has no row yet; enforcement always comes from the row's
// own limit_idr, so a per-account override is never overridden by this.
const defaultDailyDigitalLimitIDR = 20_000_000
