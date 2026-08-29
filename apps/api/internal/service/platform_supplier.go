package service

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/supplier"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The supplier catalogue, managed from the platform panel. Every method here
// goes through requirePlatformAdmin for the same reason the rest of
// PlatformService does: none of it is tenant-scoped.

func (s *PlatformService) ListSuppliers(ctx context.Context) (*hajjv1.ListSuppliersResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	suppliers, err := s.supplierRepository.ListSuppliers(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.ListSuppliers", err)
	}
	result := &hajjv1.ListSuppliersResponse{Suppliers: make([]*hajjv1.PlatformSupplier, 0, len(suppliers))}
	for _, item := range suppliers {
		result.Suppliers = append(result.Suppliers, &hajjv1.PlatformSupplier{
			Id: item.ID, Name: item.Name, Code: item.Code, BaseUrl: item.BaseURL,
			CredentialEnvVar: item.CredentialEnvVar, Status: item.Status, Notes: item.Notes,
			RouteCount: item.RouteCount, RuleCount: item.RuleCount,
			CreatedAt: timestamppb.New(item.CreatedAt),
		})
	}
	return result, nil
}

func (s *PlatformService) SaveSupplier(ctx context.Context, req *hajjv1.SaveSupplierRequest) (*hajjv1.SaveSupplierResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("PlatformService.SaveSupplier", apperror.ErrValidation)
	}
	saved, err := s.supplierRepository.SaveSupplier(ctx, repository.Supplier{
		Name: strings.TrimSpace(req.Name), Code: strings.TrimSpace(req.Code),
		BaseURL: strings.TrimSpace(req.BaseUrl), CredentialEnvVar: strings.TrimSpace(req.CredentialEnvVar),
		Status: req.Status, Notes: req.Notes,
	})
	if err != nil {
		return nil, serviceError("PlatformService.SaveSupplier", err)
	}
	s.auditPlatform(ctx, userID, "supplier_saved", saved.ID,
		"Supplier "+saved.Name+" ("+saved.Code+") disimpan dengan status "+saved.Status)
	return &hajjv1.SaveSupplierResponse{Supplier: &hajjv1.PlatformSupplier{
		Id: saved.ID, Name: saved.Name, Code: saved.Code, BaseUrl: saved.BaseURL,
		CredentialEnvVar: saved.CredentialEnvVar, Status: saved.Status, Notes: saved.Notes,
	}}, nil
}

func (s *PlatformService) ListProductRoutes(ctx context.Context) (*hajjv1.ListProductRoutesResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	routes, err := s.supplierRepository.ListRoutes(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.ListProductRoutes", err)
	}
	result := &hajjv1.ListProductRoutesResponse{Routes: make([]*hajjv1.PlatformProductRoute, 0, len(routes))}
	for _, route := range routes {
		result.Routes = append(result.Routes, &hajjv1.PlatformProductRoute{
			Id: route.ID, ProductId: route.ProductID, ProductName: route.ProductName,
			OperatorName: route.OperatorName, Category: route.Category,
			SupplierId: route.SupplierID, SupplierName: route.SupplierName,
			SupplierSku: route.SupplierSKU, IsActive: route.IsActive,
		})
	}
	return result, nil
}

func (s *PlatformService) SaveProductRoute(ctx context.Context, req *hajjv1.SaveProductRouteRequest) (*hajjv1.SaveProductRouteResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.ProductId) || !isUUID(req.SupplierId) || strings.TrimSpace(req.SupplierSku) == "" {
		return nil, serviceError("PlatformService.SaveProductRoute", apperror.ErrValidation)
	}
	if err := s.supplierRepository.SaveRoute(ctx, req.ProductId, req.SupplierId, strings.TrimSpace(req.SupplierSku), req.IsActive); err != nil {
		return nil, serviceError("PlatformService.SaveProductRoute", err)
	}
	s.auditPlatform(ctx, userID, "product_route_saved", req.ProductId,
		"Routing produk diarahkan ke supplier "+req.SupplierId+" (SKU "+req.SupplierSku+")")
	return &hajjv1.SaveProductRouteResponse{}, nil
}

func (s *PlatformService) ListResponseRules(ctx context.Context, req *hajjv1.ListResponseRulesRequest) (*hajjv1.ListResponseRulesResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.SupplierId) {
		return nil, serviceError("PlatformService.ListResponseRules", apperror.ErrValidation)
	}
	rules, err := s.supplierRepository.ListRules(ctx, req.SupplierId)
	if err != nil {
		return nil, serviceError("PlatformService.ListResponseRules", err)
	}
	result := &hajjv1.ListResponseRulesResponse{Rules: make([]*hajjv1.SupplierResponseRule, 0, len(rules))}
	for _, rule := range rules {
		result.Rules = append(result.Rules, &hajjv1.SupplierResponseRule{
			Id: rule.ID, SupplierId: rule.SupplierID, Priority: rule.Priority,
			Pattern: rule.Pattern, Outcome: string(rule.Outcome),
			ReferenceGroup: rule.ReferenceGroup, CostGroup: rule.CostGroup,
			Description: rule.Description, IsActive: rule.IsActive,
			CreatedAt: timestamppb.New(rule.CreatedAt),
		})
	}
	return result, nil
}

func (s *PlatformService) CreateResponseRule(ctx context.Context, req *hajjv1.CreateResponseRuleRequest) (*hajjv1.CreateResponseRuleResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.SupplierId) {
		return nil, serviceError("PlatformService.CreateResponseRule", apperror.ErrValidation)
	}
	rule := supplier.Rule{
		Priority: req.Priority, Pattern: req.Pattern, Outcome: supplier.Outcome(req.Outcome),
		ReferenceGroup: strings.TrimSpace(req.ReferenceGroup), CostGroup: strings.TrimSpace(req.CostGroup),
	}
	// Validated here, at the moment of saving. A pattern that does not compile,
	// or names a capture group it never defines, must be refused in the panel —
	// not discovered over live transactions at three in the morning.
	if _, err := supplier.Compile(rule); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	id, err := s.supplierRepository.CreateRule(ctx, repository.StoredRule{
		Rule: rule, SupplierID: req.SupplierId, Description: req.Description, IsActive: true,
	})
	if err != nil {
		return nil, serviceError("PlatformService.CreateResponseRule", err)
	}
	s.auditPlatform(ctx, userID, "response_rule_created", id,
		"Aturan baca respons supplier ditambahkan: "+req.Outcome+" — "+req.Pattern)
	return &hajjv1.CreateResponseRuleResponse{Rule: &hajjv1.SupplierResponseRule{
		Id: id, SupplierId: req.SupplierId, Priority: req.Priority, Pattern: req.Pattern,
		Outcome: req.Outcome, ReferenceGroup: rule.ReferenceGroup, CostGroup: rule.CostGroup,
		Description: req.Description, IsActive: true,
	}}, nil
}

func (s *PlatformService) SetResponseRuleActive(ctx context.Context, req *hajjv1.SetResponseRuleActiveRequest) (*hajjv1.SetResponseRuleActiveResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.RuleId) {
		return nil, serviceError("PlatformService.SetResponseRuleActive", apperror.ErrValidation)
	}
	if err := s.supplierRepository.SetRuleActive(ctx, req.RuleId, req.IsActive); err != nil {
		return nil, serviceError("PlatformService.SetResponseRuleActive", err)
	}
	state := "dinonaktifkan"
	if req.IsActive {
		state = "diaktifkan"
	}
	s.auditPlatform(ctx, userID, "response_rule_toggled", req.RuleId, "Aturan baca respons "+state)
	return &hajjv1.SetResponseRuleActiveResponse{}, nil
}

// TestResponseRules runs a sample through the live rules without touching
// anything. Writing a pattern blind against real money is how bad rules reach
// production, so the panel can try one first.
func (s *PlatformService) TestResponseRules(ctx context.Context, req *hajjv1.TestResponseRulesRequest) (*hajjv1.TestResponseRulesResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.SupplierId) || strings.TrimSpace(req.SampleResponse) == "" {
		return nil, serviceError("PlatformService.TestResponseRules", apperror.ErrValidation)
	}
	rules, err := s.supplierRepository.ActiveRulesFor(ctx, req.SupplierId)
	if err != nil {
		return nil, serviceError("PlatformService.TestResponseRules", err)
	}
	reading := supplier.Read(rules, req.SampleResponse)
	result := &hajjv1.TestResponseRulesResponse{
		Outcome: string(reading.Outcome), MatchedRuleId: reading.RuleID, Reference: reading.Reference,
	}
	if reading.CostIDR != nil {
		result.CostIdr = *reading.CostIDR
		result.CostReported = true
	}
	// Surfaced rather than swallowed: a rule that cannot be applied is coverage
	// missing, and the tester is where somebody is already looking.
	for _, skipped := range reading.SkippedRules {
		result.SkippedRules = append(result.SkippedRules, &hajjv1.SkippedResponseRule{
			RuleId: skipped.RuleID, Reason: skipped.Reason,
		})
	}
	return result, nil
}

func (s *PlatformService) ListSupplierLogs(ctx context.Context, req *hajjv1.ListSupplierLogsRequest) (*hajjv1.ListSupplierLogsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	unmatchedOnly := req != nil && req.UnmatchedOnly
	limit := int32(50)
	if req != nil && req.Limit > 0 {
		limit = req.Limit
	}
	logs, err := s.supplierRepository.ListLogs(ctx, unmatchedOnly, limit)
	if err != nil {
		return nil, serviceError("PlatformService.ListSupplierLogs", err)
	}
	result := &hajjv1.ListSupplierLogsResponse{Logs: make([]*hajjv1.SupplierLogEntry, 0, len(logs))}
	for _, entry := range logs {
		message := &hajjv1.SupplierLogEntry{
			Id: entry.ID, SupplierName: entry.SupplierName, OrderId: entry.OrderID,
			Direction: entry.Direction, Endpoint: entry.Endpoint,
			RequestBody: entry.RequestBody, ResponseBody: entry.ResponseBody,
			Outcome: entry.Outcome, SupplierReference: entry.SupplierReference,
			Error: entry.Error, CreatedAt: timestamppb.New(entry.CreatedAt),
		}
		if entry.HTTPStatus != nil {
			message.HttpStatus = *entry.HTTPStatus
		}
		if entry.CostIDR != nil {
			message.CostIdr = *entry.CostIDR
		}
		result.Logs = append(result.Logs, message)
	}
	return result, nil
}

// auditPlatform records a platform-side change.
//
// Written with no operator, because a platform action belongs to no tenant.
// Attributing it to whichever travel happened to be involved would put platform
// actions in a customer's trail where they do not belong.
//
// The error is reported rather than discarded. It used to be ignored, and every
// one of these was failing against a NOT NULL column — the code claimed an
// audit trail it did not have, which is worse than having none. Found by a test
// asserting that reading an identity leaves a trace.
func (s *PlatformService) auditPlatform(ctx context.Context, userID, action, entityID, detail string) {
	if s.auditRepository == nil {
		return
	}
	if err := s.auditRepository.Write(ctx, "", userID, action, "platform", entityID, detail); err != nil {
		sentry.CaptureException(fmt.Errorf("PlatformService.auditPlatform: %s: %w", action, err))
	}
}

// ListTransactions is every transaction across every tenant, paid or not.
//
// A transaction exists in this system from the moment it is created, not from
// the moment it is paid: an abandoned checkout is still a record, and one that
// appeared nowhere would be a gap nobody could investigate afterwards.
func (s *PlatformService) ListTransactions(ctx context.Context, req *hajjv1.ListTransactionsRequest) (*hajjv1.ListTransactionsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	needsAttention := req != nil && req.NeedsAttention
	limit := int32(100)
	if req != nil && req.Limit > 0 {
		limit = req.Limit
	}
	transactions, err := s.platformRepository.ListTransactions(ctx, needsAttention, limit)
	if err != nil {
		return nil, serviceError("PlatformService.ListTransactions", err)
	}
	result := &hajjv1.ListTransactionsResponse{Transactions: make([]*hajjv1.PlatformTransaction, 0, len(transactions))}
	for _, item := range transactions {
		message := &hajjv1.PlatformTransaction{
			OrderId: item.OrderID, ReceiptNumber: item.ReceiptNumber, OperatorName: item.OperatorName,
			PilgrimName: item.PilgrimName, ProductName: item.ProductName, Category: item.Category,
			AmountIdr: item.AmountIDR, NetPaidIdr: item.NetPaidIDR, Status: item.Status,
			HeldReason: item.HeldReason, RiskLevel: item.RiskLevel, RiskReason: item.RiskReason,
			FulfilmentStatus: item.FulfilmentStatus,
			SupplierName:     item.SupplierName, SupplierReference: item.SupplierReference,
			FulfilmentError: item.FulfilmentError, CreatedAt: timestamppb.New(item.CreatedAt),
		}
		if item.PaidAmountIDR != nil {
			message.PaidAmountIdr = *item.PaidAmountIDR
		}
		if item.PaidAt != nil {
			message.PaidAt = timestamppb.New(*item.PaidAt)
		}
		result.Transactions = append(result.Transactions, message)
	}
	return result, nil
}
