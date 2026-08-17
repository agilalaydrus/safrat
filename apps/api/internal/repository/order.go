package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
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
func (r *OrderRepository) Create(ctx context.Context, operatorID, seasonID, pilgrimID, productID, agentID string, quantity int32, unitPriceIDR, totalPriceIDR, platformAmountIDR, operatorAmountIDR, agentCommissionIDR int64) (*domain.Order, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	productUUID, err := pgUUID(productID)
	if err != nil {
		return nil, err
	}
	order, err := r.queries.CreateOrder(ctx, db.CreateOrderParams{
		OperatorID: opUUID, SeasonID: seasonUUID, PilgrimID: pilgrimUUID, ProductID: productUUID,
		Column5: agentID, Quantity: quantity, UnitPriceIdr: unitPriceIDR, TotalPriceIdr: totalPriceIDR,
		PlatformAmountIdr: platformAmountIDR, OperatorAmountIdr: operatorAmountIDR, AgentCommissionIdr: agentCommissionIDR,
	})
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
func (r *OrderRepository) MarkPaidByInvoiceID(ctx context.Context, invoiceID string) (*domain.Order, error) {
	order, err := r.queries.MarkOrderPaidByInvoiceID(ctx, pgtype.Text{String: invoiceID, Valid: true})
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

func (r *OrderRepository) ListBySeason(ctx context.Context, operatorID, seasonID string) ([]*domain.Order, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	orders, err := r.queries.ListOrders(ctx, db.ListOrdersParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Order, 0, len(orders))
	for _, order := range orders {
		result = append(result, toOrderFromListRow(order))
	}
	return result, nil
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
