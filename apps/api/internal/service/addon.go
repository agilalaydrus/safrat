package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
)

type AddonService struct {
	operatorRepository *repository.OperatorRepository
	addonRepository    *repository.AddonRepository
}

func NewAddonService(operators *repository.OperatorRepository, addons *repository.AddonRepository) *AddonService {
	return &AddonService{operatorRepository: operators, addonRepository: addons}
}

func addonItemMessage(i *domain.AddonItem) *hajjv1.AddonItem {
	return &hajjv1.AddonItem{Id: i.ID, SeasonId: i.SeasonID, Name: i.Name, UnitPriceIdr: i.UnitPriceIDR, IsActive: i.IsActive}
}

func pilgrimAddonMessage(a *domain.PilgrimAddon) *hajjv1.PilgrimAddon {
	return &hajjv1.PilgrimAddon{
		Id: a.ID, PilgrimId: a.PilgrimID, PilgrimName: a.PilgrimName, AddonItemId: a.AddonItemID,
		AddonName: a.AddonName, GroupName: a.GroupName, Quantity: a.Quantity, UnitPriceIdr: a.UnitPriceIDR,
		TotalIdr: a.TotalIDR, Paid: a.Paid, Notes: a.Notes,
	}
}

func (s *AddonService) CreateAddonItem(ctx context.Context, orgID string, req *hajjv1.CreateAddonItemRequest) (*hajjv1.AddonItem, error) {
	if req == nil || !isUUID(req.SeasonId) || strings.TrimSpace(req.Name) == "" || req.UnitPriceIdr < 0 {
		return nil, serviceError("AddonService.CreateAddonItem", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AddonService.CreateAddonItem", err)
	}
	item, err := s.addonRepository.CreateItem(ctx, op.ID, req.SeasonId, strings.TrimSpace(req.Name), req.UnitPriceIdr)
	if err != nil {
		return nil, serviceError("AddonService.CreateAddonItem", err)
	}
	return addonItemMessage(item), nil
}

func (s *AddonService) UpdateAddonItem(ctx context.Context, orgID string, req *hajjv1.UpdateAddonItemRequest) (*hajjv1.AddonItem, error) {
	if req == nil || strings.TrimSpace(req.ItemId) == "" || strings.TrimSpace(req.Name) == "" || req.UnitPriceIdr < 0 {
		return nil, serviceError("AddonService.UpdateAddonItem", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AddonService.UpdateAddonItem", err)
	}
	item, err := s.addonRepository.UpdateItem(ctx, op.ID, req.ItemId, strings.TrimSpace(req.Name), req.UnitPriceIdr, req.IsActive)
	if err != nil {
		return nil, serviceError("AddonService.UpdateAddonItem", err)
	}
	return addonItemMessage(item), nil
}

func (s *AddonService) ListAddonItems(ctx context.Context, orgID string, req *hajjv1.ListAddonItemsRequest) (*hajjv1.ListAddonItemsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("AddonService.ListAddonItems", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AddonService.ListAddonItems", err)
	}
	items, err := s.addonRepository.ListItems(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("AddonService.ListAddonItems", err)
	}
	response := &hajjv1.ListAddonItemsResponse{}
	for _, item := range items {
		response.Items = append(response.Items, addonItemMessage(item))
	}
	return response, nil
}

func (s *AddonService) AssignPilgrimAddon(ctx context.Context, orgID string, req *hajjv1.AssignPilgrimAddonRequest) (*hajjv1.PilgrimAddon, error) {
	if req == nil || strings.TrimSpace(req.PilgrimId) == "" || strings.TrimSpace(req.AddonItemId) == "" || req.Quantity < 1 || req.UnitPriceIdr < 0 {
		return nil, serviceError("AddonService.AssignPilgrimAddon", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AddonService.AssignPilgrimAddon", err)
	}
	addon, err := s.addonRepository.AssignPilgrimAddon(ctx, op.ID, req.PilgrimId, req.AddonItemId, req.Quantity, req.UnitPriceIdr, strings.TrimSpace(req.Notes))
	if err != nil {
		return nil, serviceError("AddonService.AssignPilgrimAddon", err)
	}
	return pilgrimAddonMessage(addon), nil
}

func (s *AddonService) SetPilgrimAddonPaid(ctx context.Context, orgID string, req *hajjv1.SetPilgrimAddonPaidRequest) (*hajjv1.PilgrimAddon, error) {
	if req == nil || strings.TrimSpace(req.PilgrimAddonId) == "" {
		return nil, serviceError("AddonService.SetPilgrimAddonPaid", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AddonService.SetPilgrimAddonPaid", err)
	}
	addon, err := s.addonRepository.SetPaid(ctx, op.ID, req.PilgrimAddonId, req.Paid)
	if err != nil {
		return nil, serviceError("AddonService.SetPilgrimAddonPaid", err)
	}
	return pilgrimAddonMessage(addon), nil
}

func (s *AddonService) RemovePilgrimAddon(ctx context.Context, orgID string, req *hajjv1.RemovePilgrimAddonRequest) error {
	if req == nil || strings.TrimSpace(req.PilgrimAddonId) == "" {
		return serviceError("AddonService.RemovePilgrimAddon", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("AddonService.RemovePilgrimAddon", err)
	}
	if err := s.addonRepository.Remove(ctx, op.ID, req.PilgrimAddonId); err != nil {
		return serviceError("AddonService.RemovePilgrimAddon", err)
	}
	return nil
}

func (s *AddonService) ListPilgrimAddons(ctx context.Context, orgID string, req *hajjv1.ListPilgrimAddonsRequest) (*hajjv1.ListPilgrimAddonsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("AddonService.ListPilgrimAddons", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AddonService.ListPilgrimAddons", err)
	}
	addons, err := s.addonRepository.ListPilgrimAddons(ctx, op.ID, req.SeasonId, strings.TrimSpace(req.GroupId))
	if err != nil {
		return nil, serviceError("AddonService.ListPilgrimAddons", err)
	}
	response := &hajjv1.ListPilgrimAddonsResponse{}
	for _, addon := range addons {
		response.Addons = append(response.Addons, pilgrimAddonMessage(addon))
	}
	return response, nil
}
