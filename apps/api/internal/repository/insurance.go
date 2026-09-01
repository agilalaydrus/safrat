package repository

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type InsuranceRepository struct {
	queries *db.Queries
}

func NewInsuranceRepository(queries *db.Queries) *InsuranceRepository {
	return &InsuranceRepository{queries: queries}
}

func (r *InsuranceRepository) CreateClaim(ctx context.Context, pilgrimID, operatorID, claimType string, incidentDate time.Time, description string, claimAmountIDR int64, filedBy string) (*domain.InsuranceClaim, error) {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateInsuranceClaim(ctx, db.CreateInsuranceClaimParams{
		ID: pilgrimUUID, OperatorID: opUUID, ClaimType: claimType, IncidentDate: pgDate(&incidentDate),
		Description: description, ClaimAmountIdr: pgtype.Int8{Int64: claimAmountIDR, Valid: true}, FiledBy: filedBy, BranchScope: scope,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.InsuranceClaim{
		ID: uuidString(row.ID), PilgrimID: uuidString(row.PilgrimID), OperatorID: uuidString(row.OperatorID),
		ClaimType: row.ClaimType, IncidentDate: row.IncidentDate.Time, Description: row.Description, Status: row.Status,
		ClaimAmountIDR: row.ClaimAmountIdr.Int64, SettledAmountIDR: row.SettledAmountIdr.Int64, FiledBy: row.FiledBy, CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *InsuranceRepository) ListClaims(ctx context.Context, operatorID string) ([]*domain.InsuranceClaim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListInsuranceClaims(ctx, db.ListInsuranceClaimsParams{OperatorID: opUUID, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.InsuranceClaim, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.InsuranceClaim{
			ID: uuidString(row.ID), PilgrimID: uuidString(row.PilgrimID), PilgrimName: row.FullName, PassportNumber: openKYC(row.PassportNumber),
			InsuranceProvider: row.InsuranceProvider, InsurancePolicyNo: row.InsurancePolicyNo, OperatorID: uuidString(row.OperatorID),
			ClaimType: row.ClaimType, IncidentDate: row.IncidentDate.Time, Description: row.Description, Status: row.Status,
			ClaimAmountIDR: row.ClaimAmountIdr.Int64, SettledAmountIDR: row.SettledAmountIdr.Int64, FiledBy: row.FiledBy, CreatedAt: row.CreatedAt.Time,
		})
	}
	return result, nil
}

func (r *InsuranceRepository) UpdateClaimStatus(ctx context.Context, operatorID, id, status string, settledAmountIDR int64) (*domain.InsuranceClaim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	idUUID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.UpdateInsuranceClaimStatus(ctx, db.UpdateInsuranceClaimStatusParams{
		ID: idUUID, OperatorID: opUUID, Status: status, SettledAmountIdr: pgtype.Int8{Int64: settledAmountIDR, Valid: settledAmountIDR > 0}, BranchScope: scope,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.InsuranceClaim{
		ID: uuidString(row.ID), PilgrimID: uuidString(row.PilgrimID), OperatorID: uuidString(row.OperatorID),
		ClaimType: row.ClaimType, IncidentDate: row.IncidentDate.Time, Description: row.Description, Status: row.Status,
		ClaimAmountIDR: row.ClaimAmountIdr.Int64, SettledAmountIDR: row.SettledAmountIdr.Int64, FiledBy: row.FiledBy, CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *InsuranceRepository) GetExportData(ctx context.Context, id, operatorID string) (*domain.InsuranceClaimExportData, error) {
	idUUID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetInsuranceClaimExportData(ctx, db.GetInsuranceClaimExportDataParams{ID: idUUID, OperatorID: opUUID, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.InsuranceClaimExportData{
		FullName: row.FullName, PassportNumber: openKYC(row.PassportNumber), DateOfBirth: row.DateOfBirth.Time, Gender: row.Gender,
		Nationality: row.Nationality, Phone: row.Phone.String, EmergencyContactName: row.EmergencyContactName, EmergencyContactPhone: row.EmergencyContactPhone,
		BloodType: row.BloodType, ChronicConditions: row.ChronicConditions, CurrentMedications: row.CurrentMedications,
		InsuranceProvider: row.InsuranceProvider, InsurancePolicyNo: row.InsurancePolicyNo, InsuranceClass: row.InsuranceClass,
		MedicalNotes: row.MedicalNotes.String, SeasonName: row.SeasonName, SeasonStartDate: row.StartDate.Time, SeasonEndDate: row.EndDate.Time,
		OperatorName: row.OperatorName, OperatorLicenseNumber: row.LicenseNumber.String, OperatorPhone: row.OperatorPhone.String,
		Claim: domain.InsuranceClaim{
			ID: uuidString(row.ID), ClaimType: row.ClaimType, IncidentDate: row.IncidentDate.Time, Description: row.Description,
			Status: row.Status, ClaimAmountIDR: row.ClaimAmountIdr.Int64, SettledAmountIDR: row.SettledAmountIdr.Int64,
			FiledBy: row.FiledBy, CreatedAt: row.CreatedAt.Time,
		},
	}, nil
}
