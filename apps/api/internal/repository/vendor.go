package repository

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
)

type VendorRepository struct {
	queries *db.Queries
}

func NewVendorRepository(queries *db.Queries) *VendorRepository {
	return &VendorRepository{queries: queries}
}

func (r *VendorRepository) CreateContract(ctx context.Context, operatorID, seasonID, vendorName, vendorType, contractNumber string, committedUnits int32, deadline *time.Time, ratePerUnitIDR, depositAmountIDR int64, notes, contactName, contactPhone string) (*domain.VendorContract, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateVendorContract(ctx, db.CreateVendorContractParams{
		OperatorID: opUUID, SeasonID: seasonUUID, VendorName: vendorName, VendorType: vendorType, ContractNumber: contractNumber,
		CommittedUnits: committedUnits, ConfirmationDeadline: pgDate(deadline), RatePerUnitIdr: ratePerUnitIDR,
		DepositAmountIdr: depositAmountIDR, Notes: notes, ContactName: contactName, ContactPhone: contactPhone,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	contract := toVendorContract(row)
	return contract, nil
}

func (r *VendorRepository) UpdateContract(ctx context.Context, operatorID, id, vendorName string, confirmedUnits int32, deadline *time.Time, status, notes string, depositPaid bool, contactName, contactPhone string) (*domain.VendorContract, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpdateVendorContract(ctx, db.UpdateVendorContractParams{
		ID: idUUID, OperatorID: opUUID, VendorName: vendorName, ConfirmedUnits: confirmedUnits,
		ConfirmationDeadline: pgDate(deadline), Status: status, Notes: notes, DepositPaid: depositPaid,
		ContactName: contactName, ContactPhone: contactPhone,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toVendorContract(row), nil
}

func (r *VendorRepository) DeleteContract(ctx context.Context, operatorID, id string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	return databaseError(r.queries.DeleteVendorContract(ctx, db.DeleteVendorContractParams{ID: idUUID, OperatorID: opUUID}))
}

func (r *VendorRepository) ListContracts(ctx context.Context, operatorID, seasonID string) ([]*domain.VendorContract, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListVendorContracts(ctx, db.ListVendorContractsParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.VendorContract, 0, len(rows))
	for _, row := range rows {
		contract := &domain.VendorContract{
			ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
			VendorName: row.VendorName, VendorType: row.VendorType, ContractNumber: row.ContractNumber,
			CommittedUnits: row.CommittedUnits, ConfirmedUnits: row.ConfirmedUnits, ConfirmationDeadline: timePtr(row.ConfirmationDeadline),
			RatePerUnitIDR: row.RatePerUnitIdr, TotalValueIDR: row.TotalValueIdr.Int64, DepositAmountIDR: row.DepositAmountIdr,
			DepositPaid: row.DepositPaid, Status: row.Status, Notes: row.Notes, ContactName: row.ContactName,
			ContactPhone: row.ContactPhone, CreatedAt: row.CreatedAt.Time,
		}
		result = append(result, contract)
	}
	return result, nil
}

func (r *VendorRepository) GetSLAStatus(ctx context.Context, operatorID, seasonID string) ([]*domain.VendorContract, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.GetVendorSLAStatus(ctx, db.GetVendorSLAStatusParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.VendorContract, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.VendorContract{
			ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
			VendorName: row.VendorName, VendorType: row.VendorType, ContractNumber: row.ContractNumber,
			CommittedUnits: row.CommittedUnits, ConfirmedUnits: row.ConfirmedUnits, ConfirmationDeadline: timePtr(row.ConfirmationDeadline),
			RatePerUnitIDR: row.RatePerUnitIdr, TotalValueIDR: row.TotalValueIdr.Int64, DepositAmountIDR: row.DepositAmountIdr,
			DepositPaid: row.DepositPaid, Status: row.Status, SLAHealth: row.SlaHealth, Notes: row.Notes,
			ContactName: row.ContactName, ContactPhone: row.ContactPhone, CreatedAt: row.CreatedAt.Time,
		})
	}
	return result, nil
}

func (r *VendorRepository) AddEvent(ctx context.Context, contractID, operatorID, eventType, description, recordedBy string) (*domain.ContractEvent, error) {
	contractUUID, err := pgUUID(contractID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateContractEvent(ctx, db.CreateContractEventParams{
		ContractID: contractUUID, OperatorID: opUUID, EventType: eventType, Description: description, RecordedBy: recordedBy,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toContractEvent(row), nil
}

func (r *VendorRepository) ListEvents(ctx context.Context, contractID, operatorID string) ([]*domain.ContractEvent, error) {
	contractUUID, err := pgUUID(contractID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListContractEvents(ctx, db.ListContractEventsParams{ContractID: contractUUID, OperatorID: opUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.ContractEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, toContractEvent(row))
	}
	return result, nil
}

func toVendorContract(value db.VendorContract) *domain.VendorContract {
	return &domain.VendorContract{
		ID: uuidString(value.ID), OperatorID: uuidString(value.OperatorID), SeasonID: uuidString(value.SeasonID),
		VendorName: value.VendorName, VendorType: value.VendorType, ContractNumber: value.ContractNumber,
		CommittedUnits: value.CommittedUnits, ConfirmedUnits: value.ConfirmedUnits, ConfirmationDeadline: timePtr(value.ConfirmationDeadline),
		RatePerUnitIDR: value.RatePerUnitIdr, TotalValueIDR: value.TotalValueIdr.Int64, DepositAmountIDR: value.DepositAmountIdr,
		DepositPaid: value.DepositPaid, Status: value.Status, Notes: value.Notes,
		ContactName: value.ContactName, ContactPhone: value.ContactPhone, CreatedAt: value.CreatedAt.Time,
	}
}

func toContractEvent(value db.VendorContractEvent) *domain.ContractEvent {
	return &domain.ContractEvent{
		ID: uuidString(value.ID), ContractID: uuidString(value.ContractID), EventType: value.EventType,
		Description: value.Description, RecordedBy: value.RecordedBy, CreatedAt: value.CreatedAt.Time,
	}
}
