package service

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type VendorService struct {
	operatorRepository *repository.OperatorRepository
	vendorRepository   *repository.VendorRepository
}

func NewVendorService(operators *repository.OperatorRepository, vendors *repository.VendorRepository) *VendorService {
	return &VendorService{operatorRepository: operators, vendorRepository: vendors}
}

func parseOptionalDate(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(dueDateLayout, value)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *VendorService) CreateContract(ctx context.Context, authenticatedOrgID string, req *hajjv1.CreateVendorContractRequest) (*hajjv1.VendorContract, error) {
	if req == nil || !isUUID(req.SeasonId) || req.VendorName == "" {
		return nil, serviceError("VendorService.CreateContract", apperror.ErrValidation)
	}
	deadline, err := parseOptionalDate(req.ConfirmationDeadline)
	if err != nil {
		return nil, serviceError("VendorService.CreateContract", apperror.ErrValidation)
	}
	vendorType := req.VendorType
	if vendorType == "" {
		vendorType = "HOTEL"
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("VendorService.CreateContract", err)
	}
	contract, err := s.vendorRepository.CreateContract(ctx, operator.ID, req.SeasonId, req.VendorName, vendorType, req.ContractNumber, req.CommittedUnits, deadline, req.RatePerUnitIdr, req.DepositAmountIdr, req.Notes, req.ContactName, req.ContactPhone)
	if err != nil {
		return nil, serviceError("VendorService.CreateContract", err)
	}
	return vendorContractMessage(contract), nil
}

func (s *VendorService) UpdateContract(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdateVendorContractRequest) (*hajjv1.VendorContract, error) {
	if req == nil || !isUUID(req.Id) || req.VendorName == "" {
		return nil, serviceError("VendorService.UpdateContract", apperror.ErrValidation)
	}
	deadline, err := parseOptionalDate(req.ConfirmationDeadline)
	if err != nil {
		return nil, serviceError("VendorService.UpdateContract", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("VendorService.UpdateContract", err)
	}
	contract, err := s.vendorRepository.UpdateContract(ctx, operator.ID, req.Id, req.VendorName, req.ConfirmedUnits, deadline, req.Status, req.Notes, req.DepositPaid, req.ContactName, req.ContactPhone)
	if err != nil {
		return nil, serviceError("VendorService.UpdateContract", err)
	}
	return vendorContractMessage(contract), nil
}

func (s *VendorService) DeleteContract(ctx context.Context, authenticatedOrgID string, req *hajjv1.DeleteVendorContractRequest) (*hajjv1.DeleteVendorContractResponse, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("VendorService.DeleteContract", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("VendorService.DeleteContract", err)
	}
	if err := s.vendorRepository.DeleteContract(ctx, operator.ID, req.Id); err != nil {
		return nil, serviceError("VendorService.DeleteContract", err)
	}
	return &hajjv1.DeleteVendorContractResponse{}, nil
}

func (s *VendorService) ListContracts(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListVendorContractsRequest) (*hajjv1.ListVendorContractsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("VendorService.ListContracts", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("VendorService.ListContracts", err)
	}
	contracts, err := s.vendorRepository.ListContracts(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("VendorService.ListContracts", err)
	}
	result := &hajjv1.ListVendorContractsResponse{Contracts: make([]*hajjv1.VendorContract, 0, len(contracts))}
	for _, contract := range contracts {
		result.Contracts = append(result.Contracts, vendorContractMessage(contract))
	}
	return result, nil
}

func (s *VendorService) GetSLAStatus(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetVendorSLAStatusRequest) (*hajjv1.GetVendorSLAStatusResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("VendorService.GetSLAStatus", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("VendorService.GetSLAStatus", err)
	}
	contracts, err := s.vendorRepository.GetSLAStatus(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("VendorService.GetSLAStatus", err)
	}
	result := &hajjv1.GetVendorSLAStatusResponse{Contracts: make([]*hajjv1.VendorContract, 0, len(contracts))}
	for _, contract := range contracts {
		result.Contracts = append(result.Contracts, vendorContractMessage(contract))
	}
	return result, nil
}

func (s *VendorService) AddContractEvent(ctx context.Context, authenticatedOrgID string, req *hajjv1.AddContractEventRequest) (*hajjv1.ContractEvent, error) {
	if req == nil || !isUUID(req.ContractId) || req.EventType == "" || req.Description == "" {
		return nil, serviceError("VendorService.AddContractEvent", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("VendorService.AddContractEvent", err)
	}
	event, err := s.vendorRepository.AddEvent(ctx, req.ContractId, operator.ID, req.EventType, req.Description, middleware.UserIDFromCtx(ctx))
	if err != nil {
		return nil, serviceError("VendorService.AddContractEvent", err)
	}
	return contractEventMessage(event), nil
}

func (s *VendorService) ListContractEvents(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListContractEventsRequest) (*hajjv1.ListContractEventsResponse, error) {
	if req == nil || !isUUID(req.ContractId) {
		return nil, serviceError("VendorService.ListContractEvents", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("VendorService.ListContractEvents", err)
	}
	events, err := s.vendorRepository.ListEvents(ctx, req.ContractId, operator.ID)
	if err != nil {
		return nil, serviceError("VendorService.ListContractEvents", err)
	}
	result := &hajjv1.ListContractEventsResponse{Events: make([]*hajjv1.ContractEvent, 0, len(events))}
	for _, event := range events {
		result.Events = append(result.Events, contractEventMessage(event))
	}
	return result, nil
}

func vendorContractMessage(value *domain.VendorContract) *hajjv1.VendorContract {
	contract := &hajjv1.VendorContract{
		Id: value.ID, VendorName: value.VendorName, VendorType: value.VendorType, ContractNumber: value.ContractNumber,
		CommittedUnits: value.CommittedUnits, ConfirmedUnits: value.ConfirmedUnits,
		RatePerUnitIdr: value.RatePerUnitIDR, TotalValueIdr: value.TotalValueIDR, DepositAmountIdr: value.DepositAmountIDR,
		DepositPaid: value.DepositPaid, Status: value.Status, SlaHealth: value.SLAHealth, Notes: value.Notes,
		ContactName: value.ContactName, ContactPhone: value.ContactPhone, CreatedAt: timestamppb.New(value.CreatedAt),
	}
	if value.ConfirmationDeadline != nil {
		contract.ConfirmationDeadline = value.ConfirmationDeadline.Format(dueDateLayout)
	}
	return contract
}

func contractEventMessage(value *domain.ContractEvent) *hajjv1.ContractEvent {
	return &hajjv1.ContractEvent{
		Id: value.ID, ContractId: value.ContractID, EventType: value.EventType,
		Description: value.Description, RecordedBy: value.RecordedBy, CreatedAt: timestamppb.New(value.CreatedAt),
	}
}
