package domain

import "time"

type InventoryItem struct {
	ID          string
	OperatorID  string
	SKU         string
	Name        string
	Unit        string
	Stock       int32
	MinStock    int32
	MaxStock    int32
	UnitCostIDR int64
	// Nil means the item is not issued per pilgrim (e.g. shared signage), not
	// "needs zero" — a kloter-readiness check must be able to tell those apart.
	PerPilgrimQty   *int32
	PerPilgrimNotes string
	MOQ             int32
	LeadTimeDays    int32
	VendorName      string
	Rak             string
	LastRestockAt   *time.Time
	CreatedAt       time.Time
}

type StockMovement struct {
	ID           string
	OperatorID   string
	ItemID       string
	MovementType string // IN | OUT | ADJUSTMENT
	Quantity     int32
	Reason       string
	Reference    string
	CreatedBy    string
	CreatedAt    time.Time
}

type PurchaseOrder struct {
	ID         string
	OperatorID string
	PONumber   string
	VendorName string
	Status     string // DRAFT | ORDERED | PARTIAL | RECEIVED | CANCELLED
	ETADate    *time.Time
	Notes      string
	CreatedAt  time.Time
}

type PurchaseOrderItem struct {
	ID               string
	OperatorID       string
	POID             string
	ItemID           string
	ItemSKU          string
	ItemName         string
	Unit             string
	QuantityOrdered  int32
	QuantityReceived int32
	UnitCostIDR      int64
	CreatedAt        time.Time
}
