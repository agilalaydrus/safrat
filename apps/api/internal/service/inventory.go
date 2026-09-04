package service

import (
	"context"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type InventoryService struct {
	operatorRepository  *repository.OperatorRepository
	inventoryRepository *repository.InventoryRepository
}

func NewInventoryService(operators *repository.OperatorRepository, inventory *repository.InventoryRepository) *InventoryService {
	return &InventoryService{operatorRepository: operators, inventoryRepository: inventory}
}

func inventoryItemMessage(item *domain.InventoryItem) *hajjv1.InventoryItem {
	msg := &hajjv1.InventoryItem{
		Id: item.ID, Sku: item.SKU, Name: item.Name, Unit: item.Unit,
		Stock: item.Stock, MinStock: item.MinStock, MaxStock: item.MaxStock,
		UnitCostIdr: item.UnitCostIDR, PerPilgrimNotes: item.PerPilgrimNotes,
		Moq: item.MOQ, LeadTimeDays: item.LeadTimeDays, VendorName: item.VendorName, Rak: item.Rak,
		CreatedAt: timestamppb.New(item.CreatedAt),
	}
	if item.PerPilgrimQty != nil {
		msg.PerPilgrimTracked = true
		msg.PerPilgrimQty = *item.PerPilgrimQty
	}
	if item.LastRestockAt != nil {
		msg.LastRestockAt = timestamppb.New(*item.LastRestockAt)
	}
	return msg
}

func perPilgrimQtyFromRequest(tracked bool, qty int32) *int32 {
	if !tracked {
		return nil
	}
	return &qty
}

func (s *InventoryService) CreateItem(ctx context.Context, orgID string, req *hajjv1.CreateInventoryItemRequest) (*hajjv1.InventoryItem, error) {
	if req == nil || strings.TrimSpace(req.Sku) == "" || strings.TrimSpace(req.Name) == "" {
		return nil, serviceError("InventoryService.CreateItem", apperror.ErrValidation)
	}
	if req.MaxStock > 0 && req.MinStock > req.MaxStock {
		return nil, serviceError("InventoryService.CreateItem", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.CreateItem", err)
	}
	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		unit = "pcs"
	}
	item, err := s.inventoryRepository.CreateItem(ctx, op.ID, strings.ToUpper(strings.TrimSpace(req.Sku)), strings.TrimSpace(req.Name), unit,
		req.MinStock, req.MaxStock, req.UnitCostIdr, perPilgrimQtyFromRequest(req.PerPilgrimTracked, req.PerPilgrimQty), req.PerPilgrimNotes,
		req.Moq, req.LeadTimeDays, strings.TrimSpace(req.VendorName), strings.TrimSpace(req.Rak))
	if err != nil {
		return nil, serviceError("InventoryService.CreateItem", err)
	}
	return inventoryItemMessage(item), nil
}

func (s *InventoryService) UpdateItem(ctx context.Context, orgID string, req *hajjv1.UpdateInventoryItemRequest) (*hajjv1.InventoryItem, error) {
	if req == nil || strings.TrimSpace(req.ItemId) == "" || strings.TrimSpace(req.Name) == "" {
		return nil, serviceError("InventoryService.UpdateItem", apperror.ErrValidation)
	}
	if req.MaxStock > 0 && req.MinStock > req.MaxStock {
		return nil, serviceError("InventoryService.UpdateItem", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.UpdateItem", err)
	}
	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		unit = "pcs"
	}
	item, err := s.inventoryRepository.UpdateItem(ctx, op.ID, req.ItemId, strings.TrimSpace(req.Name), unit,
		req.MinStock, req.MaxStock, req.UnitCostIdr, perPilgrimQtyFromRequest(req.PerPilgrimTracked, req.PerPilgrimQty), req.PerPilgrimNotes,
		req.Moq, req.LeadTimeDays, strings.TrimSpace(req.VendorName), strings.TrimSpace(req.Rak))
	if err != nil {
		return nil, serviceError("InventoryService.UpdateItem", err)
	}
	return inventoryItemMessage(item), nil
}

func (s *InventoryService) DeleteItem(ctx context.Context, orgID string, req *hajjv1.DeleteInventoryItemRequest) error {
	if req == nil || strings.TrimSpace(req.ItemId) == "" {
		return serviceError("InventoryService.DeleteItem", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("InventoryService.DeleteItem", err)
	}
	if err := s.inventoryRepository.DeleteItem(ctx, op.ID, req.ItemId); err != nil {
		return serviceError("InventoryService.DeleteItem", err)
	}
	return nil
}

func (s *InventoryService) ListItems(ctx context.Context, orgID string) (*hajjv1.ListInventoryItemsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.ListItems", err)
	}
	items, err := s.inventoryRepository.ListItems(ctx, op.ID)
	if err != nil {
		return nil, serviceError("InventoryService.ListItems", err)
	}
	response := &hajjv1.ListInventoryItemsResponse{}
	for _, item := range items {
		response.Items = append(response.Items, inventoryItemMessage(item))
	}
	return response, nil
}

func (s *InventoryService) AdjustStock(ctx context.Context, orgID, actorID string, req *hajjv1.AdjustStockRequest) (*hajjv1.InventoryItem, error) {
	if req == nil || strings.TrimSpace(req.ItemId) == "" || req.Quantity <= 0 {
		return nil, serviceError("InventoryService.AdjustStock", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.AdjustStock", err)
	}
	item, err := s.inventoryRepository.AdjustStock(ctx, op.ID, req.ItemId, strings.ToUpper(strings.TrimSpace(req.MovementType)),
		req.Quantity, strings.TrimSpace(req.Reason), strings.TrimSpace(req.Reference), actorID)
	if err != nil {
		return nil, serviceError("InventoryService.AdjustStock", err)
	}
	return inventoryItemMessage(item), nil
}

func (s *InventoryService) ListStockMovements(ctx context.Context, orgID string, req *hajjv1.ListStockMovementsRequest) (*hajjv1.ListStockMovementsResponse, error) {
	if req == nil || strings.TrimSpace(req.ItemId) == "" {
		return nil, serviceError("InventoryService.ListStockMovements", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.ListStockMovements", err)
	}
	movements, err := s.inventoryRepository.ListStockMovements(ctx, op.ID, req.ItemId, 200)
	if err != nil {
		return nil, serviceError("InventoryService.ListStockMovements", err)
	}
	response := &hajjv1.ListStockMovementsResponse{}
	for _, m := range movements {
		response.Movements = append(response.Movements, &hajjv1.StockMovement{
			Id: m.ID, ItemId: m.ItemID, MovementType: m.MovementType, Quantity: m.Quantity,
			Reason: m.Reason, Reference: m.Reference, CreatedBy: m.CreatedBy, CreatedAt: timestamppb.New(m.CreatedAt),
		})
	}
	return response, nil
}

func purchaseOrderMessage(po *domain.PurchaseOrder) *hajjv1.PurchaseOrder {
	msg := &hajjv1.PurchaseOrder{
		Id: po.ID, PoNumber: po.PONumber, VendorName: po.VendorName, Status: po.Status,
		Notes: po.Notes, CreatedAt: timestamppb.New(po.CreatedAt),
	}
	if po.ETADate != nil {
		msg.EtaDate = timestamppb.New(*po.ETADate)
	}
	return msg
}

func (s *InventoryService) CreatePurchaseOrder(ctx context.Context, orgID string, req *hajjv1.CreatePurchaseOrderRequest) (*hajjv1.PurchaseOrder, error) {
	if req == nil || strings.TrimSpace(req.PoNumber) == "" {
		return nil, serviceError("InventoryService.CreatePurchaseOrder", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.CreatePurchaseOrder", err)
	}
	var eta *time.Time
	if req.EtaDate != nil {
		t := req.EtaDate.AsTime()
		eta = &t
	}
	po, err := s.inventoryRepository.CreatePurchaseOrder(ctx, op.ID, strings.TrimSpace(req.PoNumber), strings.TrimSpace(req.VendorName), eta, strings.TrimSpace(req.Notes))
	if err != nil {
		return nil, serviceError("InventoryService.CreatePurchaseOrder", err)
	}
	return purchaseOrderMessage(po), nil
}

func (s *InventoryService) UpdatePurchaseOrderStatus(ctx context.Context, orgID string, req *hajjv1.UpdatePurchaseOrderStatusRequest) (*hajjv1.PurchaseOrder, error) {
	if req == nil || strings.TrimSpace(req.PoId) == "" || strings.TrimSpace(req.Status) == "" {
		return nil, serviceError("InventoryService.UpdatePurchaseOrderStatus", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.UpdatePurchaseOrderStatus", err)
	}
	po, err := s.inventoryRepository.UpdatePurchaseOrderStatus(ctx, op.ID, req.PoId, strings.ToUpper(strings.TrimSpace(req.Status)))
	if err != nil {
		return nil, serviceError("InventoryService.UpdatePurchaseOrderStatus", err)
	}
	return purchaseOrderMessage(po), nil
}

func (s *InventoryService) ListPurchaseOrders(ctx context.Context, orgID string) (*hajjv1.ListPurchaseOrdersResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.ListPurchaseOrders", err)
	}
	orders, err := s.inventoryRepository.ListPurchaseOrders(ctx, op.ID)
	if err != nil {
		return nil, serviceError("InventoryService.ListPurchaseOrders", err)
	}
	response := &hajjv1.ListPurchaseOrdersResponse{}
	for _, po := range orders {
		response.Orders = append(response.Orders, purchaseOrderMessage(po))
	}
	return response, nil
}

func (s *InventoryService) AddPurchaseOrderItem(ctx context.Context, orgID string, req *hajjv1.AddPurchaseOrderItemRequest) error {
	if req == nil || strings.TrimSpace(req.PoId) == "" || strings.TrimSpace(req.ItemId) == "" || req.QuantityOrdered <= 0 {
		return serviceError("InventoryService.AddPurchaseOrderItem", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("InventoryService.AddPurchaseOrderItem", err)
	}
	if err := s.inventoryRepository.AddPurchaseOrderItem(ctx, op.ID, req.PoId, req.ItemId, req.QuantityOrdered, req.UnitCostIdr); err != nil {
		return serviceError("InventoryService.AddPurchaseOrderItem", err)
	}
	return nil
}

func purchaseOrderItemMessage(item *domain.PurchaseOrderItem) *hajjv1.PurchaseOrderItem {
	return &hajjv1.PurchaseOrderItem{
		Id: item.ID, PoId: item.POID, ItemId: item.ItemID, ItemSku: item.ItemSKU, ItemName: item.ItemName,
		Unit: item.Unit, QuantityOrdered: item.QuantityOrdered, QuantityReceived: item.QuantityReceived,
		UnitCostIdr: item.UnitCostIDR,
	}
}

func (s *InventoryService) ListPurchaseOrderItems(ctx context.Context, orgID string, req *hajjv1.ListPurchaseOrderItemsRequest) (*hajjv1.ListPurchaseOrderItemsResponse, error) {
	if req == nil || strings.TrimSpace(req.PoId) == "" {
		return nil, serviceError("InventoryService.ListPurchaseOrderItems", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.ListPurchaseOrderItems", err)
	}
	items, err := s.inventoryRepository.ListPurchaseOrderItems(ctx, op.ID, req.PoId)
	if err != nil {
		return nil, serviceError("InventoryService.ListPurchaseOrderItems", err)
	}
	response := &hajjv1.ListPurchaseOrderItemsResponse{}
	for _, item := range items {
		response.Items = append(response.Items, purchaseOrderItemMessage(item))
	}
	return response, nil
}

func (s *InventoryService) ReceivePurchaseOrderItem(ctx context.Context, orgID, actorID string, req *hajjv1.ReceivePurchaseOrderItemRequest) (*hajjv1.PurchaseOrderItem, error) {
	if req == nil || strings.TrimSpace(req.PoItemId) == "" || req.Quantity <= 0 {
		return nil, serviceError("InventoryService.ReceivePurchaseOrderItem", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.ReceivePurchaseOrderItem", err)
	}
	item, err := s.inventoryRepository.ReceivePurchaseOrderItem(ctx, op.ID, req.PoItemId, req.Quantity, actorID)
	if err != nil {
		return nil, serviceError("InventoryService.ReceivePurchaseOrderItem", err)
	}
	return purchaseOrderItemMessage(item), nil
}

func (s *InventoryService) GetSummary(ctx context.Context, orgID string) (*hajjv1.GetInventorySummaryResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("InventoryService.GetSummary", err)
	}
	summary, err := s.inventoryRepository.Summary(ctx, op.ID)
	if err != nil {
		return nil, serviceError("InventoryService.GetSummary", err)
	}
	return &hajjv1.GetInventorySummaryResponse{
		ValuationIdr: summary.ValuationIDR, BelowMinimumCount: summary.BelowMinimum,
		OpenPurchaseOrders: summary.OpenPurchaseOrders, StockTurnoverRatio: summary.StockTurnoverRatio,
	}, nil
}
