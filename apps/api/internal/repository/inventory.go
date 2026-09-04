package repository

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InventoryRepository owns the warehouse: items, their stock ledger, and
// purchase orders. Stock changes always go through AdjustStock inside the
// same transaction as the movement row that explains them — the ledger and
// the running total are not allowed to drift apart.
type InventoryRepository struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewInventoryRepository(queries *db.Queries, pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{queries: queries, pool: pool}
}

func int4OrNil(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}
func int4Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	value := v.Int32
	return &value
}

func toInventoryItem(row db.InventoryItem) *domain.InventoryItem {
	return &domain.InventoryItem{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SKU: row.Sku, Name: row.Name,
		Unit: row.Unit, Stock: row.Stock, MinStock: row.MinStock, MaxStock: row.MaxStock,
		UnitCostIDR: row.UnitCostIdr, PerPilgrimQty: int4Ptr(row.PerPilgrimQty), PerPilgrimNotes: row.PerPilgrimNotes,
		MOQ: row.Moq, LeadTimeDays: row.LeadTimeDays, VendorName: row.VendorName, Rak: row.Rak,
		LastRestockAt: pgTimeToPtr(row.LastRestockAt), CreatedAt: row.CreatedAt.Time,
	}
}

func (r *InventoryRepository) CreateItem(ctx context.Context, operatorID, sku, name, unit string, minStock, maxStock int32, unitCostIDR int64, perPilgrimQty *int32, perPilgrimNotes string, moq, leadTimeDays int32, vendorName, rak string) (*domain.InventoryItem, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateInventoryItem(ctx, db.CreateInventoryItemParams{
		OperatorID: op, Sku: sku, Name: name, Unit: unit, MinStock: minStock, MaxStock: maxStock,
		UnitCostIdr: unitCostIDR, PerPilgrimQty: int4OrNil(perPilgrimQty), PerPilgrimNotes: perPilgrimNotes,
		Moq: moq, LeadTimeDays: leadTimeDays, VendorName: vendorName, Rak: rak,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toInventoryItem(row), nil
}

func (r *InventoryRepository) UpdateItem(ctx context.Context, operatorID, id, name, unit string, minStock, maxStock int32, unitCostIDR int64, perPilgrimQty *int32, perPilgrimNotes string, moq, leadTimeDays int32, vendorName, rak string) (*domain.InventoryItem, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	itemID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpdateInventoryItem(ctx, db.UpdateInventoryItemParams{
		ID: itemID, OperatorID: op, Name: name, Unit: unit, MinStock: minStock, MaxStock: maxStock,
		UnitCostIdr: unitCostIDR, PerPilgrimQty: int4OrNil(perPilgrimQty), PerPilgrimNotes: perPilgrimNotes,
		Moq: moq, LeadTimeDays: leadTimeDays, VendorName: vendorName, Rak: rak,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toInventoryItem(row), nil
}

func (r *InventoryRepository) DeleteItem(ctx context.Context, operatorID, id string) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	itemID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	if err := r.queries.DeleteInventoryItem(ctx, db.DeleteInventoryItemParams{ID: itemID, OperatorID: op}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *InventoryRepository) ListItems(ctx context.Context, operatorID string) ([]*domain.InventoryItem, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListInventoryItems(ctx, op)
	if err != nil {
		return nil, databaseError(err)
	}
	items := make([]*domain.InventoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toInventoryItem(row))
	}
	return items, nil
}

// AdjustStock records a movement and updates the running total in one
// transaction — a reader must never be able to see one without the other.
func (r *InventoryRepository) AdjustStock(ctx context.Context, operatorID, itemID, movementType string, quantity int32, reason, reference, createdBy string) (*domain.InventoryItem, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	item, err := pgUUID(itemID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if quantity == 0 || (movementType != "IN" && movementType != "OUT" && movementType != "ADJUSTMENT") {
		return nil, apperror.ErrValidation
	}
	delta := quantity
	if movementType == "OUT" {
		delta = -quantity
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.queries.WithTx(tx)

	updated, err := qtx.AdjustInventoryStock(ctx, db.AdjustInventoryStockParams{ID: item, OperatorID: op, Delta: delta})
	if err != nil {
		return nil, databaseError(err)
	}
	if updated.Stock < 0 {
		return nil, apperror.ErrValidation
	}
	if _, err := qtx.CreateStockMovement(ctx, db.CreateStockMovementParams{
		OperatorID: op, ItemID: item, MovementType: movementType, Quantity: quantity,
		Reason: reason, Reference: reference, CreatedBy: createdBy,
	}); err != nil {
		return nil, databaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, databaseError(err)
	}
	return toInventoryItem(updated), nil
}

func toStockMovement(row db.InventoryStockMovement) *domain.StockMovement {
	return &domain.StockMovement{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), ItemID: uuidString(row.ItemID),
		MovementType: row.MovementType, Quantity: row.Quantity, Reason: row.Reason, Reference: row.Reference,
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt.Time,
	}
}

func (r *InventoryRepository) ListStockMovements(ctx context.Context, operatorID, itemID string, limit int32) ([]*domain.StockMovement, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	item, err := pgUUID(itemID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListStockMovements(ctx, db.ListStockMovementsParams{OperatorID: op, ItemID: item, Limit: limit})
	if err != nil {
		return nil, databaseError(err)
	}
	movements := make([]*domain.StockMovement, 0, len(rows))
	for _, row := range rows {
		movements = append(movements, toStockMovement(row))
	}
	return movements, nil
}

func toPurchaseOrder(row db.PurchaseOrder) *domain.PurchaseOrder {
	return &domain.PurchaseOrder{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), PONumber: row.PoNumber,
		VendorName: row.VendorName, Status: row.Status, ETADate: timePtr(row.EtaDate), Notes: row.Notes,
		CreatedAt: row.CreatedAt.Time,
	}
}

func (r *InventoryRepository) CreatePurchaseOrder(ctx context.Context, operatorID, poNumber, vendorName string, eta *time.Time, notes string) (*domain.PurchaseOrder, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreatePurchaseOrder(ctx, db.CreatePurchaseOrderParams{
		OperatorID: op, PoNumber: poNumber, VendorName: vendorName, EtaDate: pgDate(eta), Notes: notes,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPurchaseOrder(row), nil
}

func (r *InventoryRepository) UpdatePurchaseOrderStatus(ctx context.Context, operatorID, id, status string) (*domain.PurchaseOrder, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	poID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpdatePurchaseOrderStatus(ctx, db.UpdatePurchaseOrderStatusParams{ID: poID, OperatorID: op, Status: status})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPurchaseOrder(row), nil
}

func (r *InventoryRepository) ListPurchaseOrders(ctx context.Context, operatorID string) ([]*domain.PurchaseOrder, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListPurchaseOrders(ctx, op)
	if err != nil {
		return nil, databaseError(err)
	}
	orders := make([]*domain.PurchaseOrder, 0, len(rows))
	for _, row := range rows {
		orders = append(orders, toPurchaseOrder(row))
	}
	return orders, nil
}

func (r *InventoryRepository) GetPurchaseOrder(ctx context.Context, operatorID, id string) (*domain.PurchaseOrder, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	poID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.GetPurchaseOrder(ctx, db.GetPurchaseOrderParams{ID: poID, OperatorID: op})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPurchaseOrder(row), nil
}

func toPurchaseOrderItem(row db.ListPurchaseOrderItemsRow) *domain.PurchaseOrderItem {
	return &domain.PurchaseOrderItem{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), POID: uuidString(row.PoID),
		ItemID: uuidString(row.ItemID), ItemSKU: row.Sku, ItemName: row.ItemName, Unit: row.Unit,
		QuantityOrdered: row.QuantityOrdered, QuantityReceived: row.QuantityReceived,
		UnitCostIDR: row.UnitCostIdr, CreatedAt: row.CreatedAt.Time,
	}
}

func (r *InventoryRepository) AddPurchaseOrderItem(ctx context.Context, operatorID, poID, itemID string, quantityOrdered int32, unitCostIDR int64) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	po, err := pgUUID(poID)
	if err != nil {
		return apperror.ErrValidation
	}
	item, err := pgUUID(itemID)
	if err != nil {
		return apperror.ErrValidation
	}
	if _, err := r.queries.CreatePurchaseOrderItem(ctx, db.CreatePurchaseOrderItemParams{
		OperatorID: op, PoID: po, ItemID: item, QuantityOrdered: quantityOrdered, UnitCostIdr: unitCostIDR,
	}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *InventoryRepository) ListPurchaseOrderItems(ctx context.Context, operatorID, poID string) ([]*domain.PurchaseOrderItem, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	po, err := pgUUID(poID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListPurchaseOrderItems(ctx, db.ListPurchaseOrderItemsParams{PoID: po, OperatorID: op})
	if err != nil {
		return nil, databaseError(err)
	}
	items := make([]*domain.PurchaseOrderItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toPurchaseOrderItem(row))
	}
	return items, nil
}

// ReceivePurchaseOrderItem records a delivery: bumps the PO line's received
// quantity, adds the same amount to warehouse stock (with a movement row
// pointing back at the PO), and rolls the PO's own status to PARTIAL or
// RECEIVED depending on whether anything is still outstanding — all in one
// transaction, so a line can never show "received" while the shelf does not
// yet reflect it.
func (r *InventoryRepository) ReceivePurchaseOrderItem(ctx context.Context, operatorID, poItemID string, quantity int32, createdBy string) (*domain.PurchaseOrderItem, error) {
	if quantity <= 0 {
		return nil, apperror.ErrValidation
	}
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	poItem, err := pgUUID(poItemID)
	if err != nil {
		return nil, apperror.ErrValidation
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.queries.WithTx(tx)

	updated, err := qtx.ReceivePurchaseOrderItem(ctx, db.ReceivePurchaseOrderItemParams{ID: poItem, OperatorID: op, Delta: quantity})
	if err != nil {
		return nil, databaseError(err)
	}
	if updated.QuantityReceived > updated.QuantityOrdered {
		return nil, apperror.ErrValidation
	}

	if _, err := qtx.AdjustInventoryStock(ctx, db.AdjustInventoryStockParams{ID: updated.ItemID, OperatorID: op, Delta: quantity}); err != nil {
		return nil, databaseError(err)
	}
	poNumber := uuidString(updated.PoID)
	if _, err := qtx.CreateStockMovement(ctx, db.CreateStockMovementParams{
		OperatorID: op, ItemID: updated.ItemID, MovementType: "IN", Quantity: quantity,
		Reason: "Penerimaan PO", Reference: poNumber, CreatedBy: createdBy,
	}); err != nil {
		return nil, databaseError(err)
	}

	totals, err := qtx.PurchaseOrderReceiptTotals(ctx, db.PurchaseOrderReceiptTotalsParams{PoID: updated.PoID, OperatorID: op})
	if err != nil {
		return nil, databaseError(err)
	}
	status := "PARTIAL"
	if totals.ReceivedTotal >= totals.OrderedTotal {
		status = "RECEIVED"
	}
	if _, err := qtx.UpdatePurchaseOrderStatus(ctx, db.UpdatePurchaseOrderStatusParams{ID: updated.PoID, OperatorID: op, Status: status}); err != nil {
		return nil, databaseError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, databaseError(err)
	}
	items, err := r.ListPurchaseOrderItems(ctx, operatorID, uuidString(updated.PoID))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == uuidString(updated.ID) {
			return item, nil
		}
	}
	return nil, apperror.ErrNotFound
}

// InventorySummary is the warehouse dashboard's KPI row.
type InventorySummary struct {
	ValuationIDR       int64
	BelowMinimum       int32
	OpenPurchaseOrders int32
	// StockTurnoverRatio is a simplified proxy — total OUT quantity over the
	// last 90 days divided by current total stock on hand — not a full
	// COGS/average-inventory turnover figure. Good enough to flag "moving
	// fast" vs "not moving", not precise enough to be reported as accounting
	// turnover.
	StockTurnoverRatio float64
}

func (r *InventoryRepository) Summary(ctx context.Context, operatorID string) (*InventorySummary, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	valuation, err := r.queries.InventoryValuation(ctx, op)
	if err != nil {
		return nil, databaseError(err)
	}
	belowMin, err := r.queries.InventoryBelowMinimumCount(ctx, op)
	if err != nil {
		return nil, databaseError(err)
	}
	openPOs, err := r.queries.PurchaseOrdersOpenCount(ctx, op)
	if err != nil {
		return nil, databaseError(err)
	}
	outSince, err := r.queries.StockOutQuantitySince(ctx, db.StockOutQuantitySinceParams{
		OperatorID: op, CreatedAt: pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -90), Valid: true},
	})
	if err != nil {
		return nil, databaseError(err)
	}
	items, err := r.ListItems(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	var totalStock int64
	for _, item := range items {
		totalStock += int64(item.Stock)
	}
	var turnover float64
	if totalStock > 0 {
		turnover = float64(outSince) / float64(totalStock)
	}
	return &InventorySummary{
		ValuationIDR: valuation, BelowMinimum: belowMin, OpenPurchaseOrders: openPOs,
		StockTurnoverRatio: turnover,
	}, nil
}
