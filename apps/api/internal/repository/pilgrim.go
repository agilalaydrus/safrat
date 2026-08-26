package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type PilgrimRepository struct {
	queries *db.Queries
}

func NewPilgrimRepository(queries *db.Queries) *PilgrimRepository {
	return &PilgrimRepository{queries: queries}
}

func (r *PilgrimRepository) Create(ctx context.Context, operatorID string, input domain.PilgrimInput) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(input.SeasonID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.CreatePilgrim(ctx, db.CreatePilgrimParams{
		SeasonID:           seasonUUID,
		OperatorID:         operatorUUID,
		Column3:            input.GroupID,
		FullName:           input.FullName,
		PassportNumber:     input.PassportNumber,
		Nationality:        input.Nationality,
		DateOfBirth:        pgTimestamp(input.DateOfBirth),
		Gender:             input.Gender,
		Column9:            input.PhotoURL,
		Column10:           input.Phone,
		Column11:           input.EmergencyContact,
		PreferredLang:      input.PreferredLang,
		Column13:           input.MedicalNotes,
		RequiresWheelchair: input.RequiresWheelchair,
		Column15:           input.MahramID,
		Column16:           input.KloterID,
		Column17:           input.Email,
		Column18:           input.AgentID,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) Get(ctx context.Context, operatorID, pilgrimID string) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.GetPilgrim(ctx, db.GetPilgrimParams{ID: pilgrimUUID, OperatorID: operatorUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) GetByPassport(ctx context.Context, operatorID, seasonID, passportNumber string) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.GetPilgrimByPassport(ctx, db.GetPilgrimByPassportParams{OperatorID: operatorUUID, SeasonID: seasonUUID, PassportNumber: passportNumber})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) List(ctx context.Context, operatorID, seasonID string, limit, offset int32) ([]*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	pilgrims, err := r.queries.ListPilgrims(ctx, db.ListPilgrimsParams{OperatorID: operatorUUID, SeasonID: seasonUUID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, databaseError(err)
	}
	results := make([]*domain.Pilgrim, 0, len(pilgrims))
	for _, pilgrim := range pilgrims {
		results = append(results, toPilgrim(pilgrim))
	}
	return results, nil
}

func (r *PilgrimRepository) Update(ctx context.Context, operatorID, pilgrimID string, input domain.PilgrimInput) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.UpdatePilgrim(ctx, db.UpdatePilgrimParams{
		ID:                 pilgrimUUID,
		OperatorID:         operatorUUID,
		Column3:            input.GroupID,
		FullName:           input.FullName,
		PassportNumber:     input.PassportNumber,
		Nationality:        input.Nationality,
		DateOfBirth:        pgTimestamp(input.DateOfBirth),
		Gender:             input.Gender,
		Column9:            input.PhotoURL,
		Column10:           input.Phone,
		Column11:           input.EmergencyContact,
		PreferredLang:      input.PreferredLang,
		Column13:           input.MedicalNotes,
		RequiresWheelchair: input.RequiresWheelchair,
		Column15:           input.MahramID,
		Column16:           input.KloterID,
		Column17:           input.Email,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) MarkSubstituted(ctx context.Context, operatorID, pilgrimID string) error {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return err
	}
	if err := r.queries.MarkSubstituted(ctx, db.MarkSubstitutedParams{ID: pilgrimUUID, OperatorID: operatorUUID}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *PilgrimRepository) RegenerateAccessCode(ctx context.Context, operatorID, pilgrimID string) error {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return err
	}
	if err := r.queries.RegenerateAccessCode(ctx, db.RegenerateAccessCodeParams{ID: pilgrimUUID, OperatorID: operatorUUID}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *PilgrimRepository) GetTx(ctx context.Context, tx pgx.Tx, operatorID, pilgrimID string) (*domain.Pilgrim, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.WithTx(tx).GetPilgrim(ctx, db.GetPilgrimParams{ID: pilgrimUUID, OperatorID: operatorUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) SubstitutePilgrimTx(ctx context.Context, tx pgx.Tx, originalID, replacementID, operatorID, reason string) error {
	originalUUID, err := pgUUID(originalID)
	if err != nil {
		return err
	}
	replacementUUID, err := pgUUID(replacementID)
	if err != nil {
		return err
	}
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	return databaseError(r.queries.WithTx(tx).SubstitutePilgrim(ctx, db.SubstitutePilgrimParams{ID: originalUUID, SubstitutedByID: replacementUUID, OperatorID: operatorUUID, SubstitutionReason: reason}))
}

func (r *PilgrimRepository) ListSubstitutions(ctx context.Context, operatorID, seasonID string) ([]*domain.Substitution, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListSubstitutions(ctx, db.ListSubstitutionsParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.Substitution, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.Substitution{
			OriginalID:             uuid.UUID(row.OriginalID.Bytes).String(),
			OriginalName:           row.OriginalName,
			OriginalPassportNumber: row.OriginalPassportNumber,
			NewID:                  uuid.UUID(row.NewID.Bytes).String(),
			NewName:                row.NewName,
			Reason:                 row.Reason,
			SubstitutedAt:          row.SubstitutedAt.Time,
		})
	}
	return result, nil
}

func (r *PilgrimRepository) TransferPilgrimGroupTx(ctx context.Context, tx pgx.Tx, pilgrimID, groupID, operatorID string) error {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return err
	}
	groupUUID, err := pgUUID(groupID)
	if err != nil {
		return err
	}
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	return databaseError(r.queries.WithTx(tx).TransferPilgrimGroup(ctx, db.TransferPilgrimGroupParams{ID: pilgrimUUID, GroupID: groupUUID, OperatorID: operatorUUID}))
}

func (r *PilgrimRepository) WriteAuditLogTx(ctx context.Context, tx pgx.Tx, operatorID, userID, action, entityID, message string) error {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	entityUUID, err := pgUUID(entityID)
	if err != nil {
		return err
	}
	return databaseError(r.queries.WithTx(tx).CreateAuditLog(ctx, db.CreateAuditLogParams{OperatorID: operatorUUID, UserID: userID, Action: action, EntityType: "pilgrim", EntityID: entityUUID, Column6: message}))
}

func (r *PilgrimRepository) CountBySeason(ctx context.Context, operatorID, seasonID string) (int64, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return 0, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return 0, err
	}
	count, err := r.queries.CountPilgrimsBySeason(ctx, db.CountPilgrimsBySeasonParams{OperatorID: operatorUUID, SeasonID: seasonUUID})
	if err != nil {
		return 0, databaseError(err)
	}
	return count, nil
}

func (r *PilgrimRepository) GetStats(ctx context.Context, operatorID, seasonID string) (domain.PilgrimStats, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return domain.PilgrimStats{}, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return domain.PilgrimStats{}, err
	}
	stats, err := r.queries.GetPilgrimStats(ctx, db.GetPilgrimStatsParams{OperatorID: operatorUUID, SeasonID: seasonUUID})
	if err != nil {
		return domain.PilgrimStats{}, databaseError(err)
	}
	return domain.PilgrimStats{Total: stats.Total, Substituted: stats.Substituted, RequiresWheelchair: stats.RequiresWheelchair, UnassignedGroup: stats.UnassignedGroup, UnassignedKloter: stats.UnassignedKloter}, nil
}

func (r *PilgrimRepository) GetStatsByKloter(ctx context.Context, operatorID, seasonID, kloterID string) (domain.PilgrimStats, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return domain.PilgrimStats{}, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return domain.PilgrimStats{}, err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return domain.PilgrimStats{}, err
	}
	stats, err := r.queries.GetPilgrimStatsByKloter(ctx, db.GetPilgrimStatsByKloterParams{OperatorID: operatorUUID, SeasonID: seasonUUID, KloterID: kloterUUID})
	if err != nil {
		return domain.PilgrimStats{}, databaseError(err)
	}
	return domain.PilgrimStats{Total: stats.Total, Substituted: stats.Substituted, RequiresWheelchair: stats.RequiresWheelchair, UnassignedGroup: stats.UnassignedGroup, UnassignedKloter: stats.UnassignedKloter}, nil
}

func (r *PilgrimRepository) UpdatePayment(ctx context.Context, operatorID, pilgrimID, paymentStatus, notes string) (*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.UpdatePilgrimPayment(ctx, db.UpdatePilgrimPaymentParams{
		ID: pilgrimUUID, OperatorID: opUUID, PaymentStatus: paymentStatus, PaymentNotes: notes,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) UpdateDocuments(ctx context.Context, operatorID, pilgrimID string, input domain.PilgrimDocumentChecklistInput) (*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.UpdatePilgrimDocuments(ctx, db.UpdatePilgrimDocumentsParams{
		ID: pilgrimUUID, OperatorID: opUUID, DocumentsPassport: input.Passport, DocumentsPhoto: input.Photo, DocumentsVaccine: input.Vaccine,
		PassportExpiryDate: pgDate(input.PassportExpiry), VaccineMeningitisDate: pgDate(input.VaccineDate),
		DocumentsKtp: input.KTP, DocumentsKk: input.KK, DocumentsMahramProof: input.MahramProof,
		DocumentsVisa: input.Visa, VisaNumber: input.VisaNumber, VisaExpiryDate: pgDate(input.VisaExpiry),
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) UpdateEmergencyContact(ctx context.Context, operatorID, pilgrimID, name, phone string) (*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.UpdatePilgrimEmergencyContact(ctx, db.UpdatePilgrimEmergencyContactParams{
		ID: pilgrimUUID, OperatorID: opUUID, EmergencyContactName: name, EmergencyContactPhone: phone,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) UpdateInsurance(ctx context.Context, operatorID, pilgrimID string, input domain.PilgrimInsuranceInput) (*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.UpdatePilgrimInsurance(ctx, db.UpdatePilgrimInsuranceParams{
		ID: pilgrimUUID, OperatorID: opUUID, InsuranceProvider: input.Provider, InsurancePolicyNo: input.PolicyNo,
		InsuranceClass: input.Class, BloodType: input.BloodType, ChronicConditions: input.ChronicConditions, CurrentMedications: input.CurrentMedications,
		InsuranceStartDate: pgDate(input.StartDate), InsuranceEndDate: pgDate(input.EndDate),
		InsuranceBeneficiaryName: input.BeneficiaryName, InsuranceBeneficiaryRelation: input.BeneficiaryRelation,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) CheckInHotel(ctx context.Context, operatorID, pilgrimID string, checkedIn bool) (*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	pilgrim, err := r.queries.CheckInPilgrimHotel(ctx, db.CheckInPilgrimHotelParams{ID: pilgrimUUID, OperatorID: opUUID, HotelCheckedIn: checkedIn})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(pilgrim), nil
}

func (r *PilgrimRepository) ListWithExpiringPassports(ctx context.Context, operatorID, seasonID string, before time.Time) ([]*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListPilgrimsWithExpiringPassports(ctx, db.ListPilgrimsWithExpiringPassportsParams{
		OperatorID: opUUID, SeasonID: seasonUUID, PassportExpiryDate: pgtype.Date{Time: before, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Pilgrim, 0, len(rows))
	for _, row := range rows {
		result = append(result, toPilgrim(row))
	}
	return result, nil
}

func (r *PilgrimRepository) CreateDocument(ctx context.Context, operatorID, pilgrimID, docType, fileURL, fileName, uploadedBy string) (*domain.PilgrimDocument, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreatePilgrimDocument(ctx, db.CreatePilgrimDocumentParams{
		PilgrimID: pilgrimUUID, OperatorID: opUUID, DocType: docType, FileUrl: fileURL, FileName: fileName, UploadedBy: uploadedBy,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrimDocument(row), nil
}

// UpdateKYC is used both by an admin editing a pilgrim's KYC fields and by
// the pilgrim submitting their own (kycSource distinguishes which) — either
// way it resets status to PENDING_REVIEW and clears any prior verification,
// since the data just changed and needs a fresh look.
func (r *PilgrimRepository) UpdateKYC(ctx context.Context, operatorID, pilgrimID string, input domain.PilgrimKYCInput, kycSource string) (*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.UpdatePilgrimKyc(ctx, db.UpdatePilgrimKycParams{
		ID: pilgrimUUID, OperatorID: opUUID, Nik: input.NIK, Address: input.Address,
		PlaceOfBirth: input.PlaceOfBirth, MaritalStatus: input.MaritalStatus, Occupation: input.Occupation, FatherName: input.FatherName,
		KycStatus: "PENDING_REVIEW", KycSource: kycSource,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(row), nil
}

func (r *PilgrimRepository) VerifyKYC(ctx context.Context, operatorID, pilgrimID, verifiedBy string, approve bool, rejectionReason string) (*domain.Pilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	status := "VERIFIED"
	if !approve {
		status = "REJECTED"
	}
	row, err := r.queries.VerifyPilgrimKyc(ctx, db.VerifyPilgrimKycParams{
		ID: pilgrimUUID, OperatorID: opUUID, KycStatus: status, KycVerifiedBy: verifiedBy, KycRejectionReason: rejectionReason,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toPilgrim(row), nil
}

func (r *PilgrimRepository) ListDocuments(ctx context.Context, pilgrimID string) ([]*domain.PilgrimDocument, error) {
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListPilgrimDocuments(ctx, pilgrimUUID)
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.PilgrimDocument, 0, len(rows))
	for _, row := range rows {
		result = append(result, toPilgrimDocument(row))
	}
	return result, nil
}

func (r *PilgrimRepository) ListSeasonDocuments(ctx context.Context, operatorID, seasonID string) ([]*domain.PilgrimDocument, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListSeasonDocuments(ctx, db.ListSeasonDocumentsParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.PilgrimDocument, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.PilgrimDocument{
			ID:             uuid.UUID(row.ID.Bytes).String(),
			PilgrimID:      uuid.UUID(row.PilgrimID.Bytes).String(),
			DocType:        row.DocType,
			FileURL:        row.FileUrl,
			FileName:       row.FileName,
			UploadedBy:     row.UploadedBy,
			CreatedAt:      row.CreatedAt.Time,
			PilgrimName:    row.PilgrimName,
			PassportNumber: row.PassportNumber,
		})
	}
	return result, nil
}

func (r *PilgrimRepository) DeleteDocument(ctx context.Context, operatorID, documentID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	docUUID, err := pgUUID(documentID)
	if err != nil {
		return err
	}
	return databaseError(r.queries.DeletePilgrimDocument(ctx, db.DeletePilgrimDocumentParams{ID: docUUID, OperatorID: opUUID}))
}

func toPilgrimDocument(value db.PilgrimDocument) *domain.PilgrimDocument {
	return &domain.PilgrimDocument{
		ID:         uuid.UUID(value.ID.Bytes).String(),
		PilgrimID:  uuid.UUID(value.PilgrimID.Bytes).String(),
		DocType:    value.DocType,
		FileURL:    value.FileUrl,
		FileName:   value.FileName,
		UploadedBy: value.UploadedBy,
		CreatedAt:  value.CreatedAt.Time,
	}
}

func databaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.ErrNotFound
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		// ConstraintName lets the caller tell "this passport is already
		// registered" apart from "this email is already registered" —
		// %w keeps errors.Is(err, apperror.ErrAlreadyExists) working for
		// every existing caller that doesn't care about the distinction.
		if pgError.ConstraintName != "" {
			return fmt.Errorf("%w: %s", apperror.ErrAlreadyExists, pgError.ConstraintName)
		}
		return apperror.ErrAlreadyExists
	}
	return err
}

func toPilgrim(value db.Pilgrim) *domain.Pilgrim {
	return &domain.Pilgrim{
		ID:                           uuidString(value.ID),
		SeasonID:                     uuidString(value.SeasonID),
		OperatorID:                   uuidString(value.OperatorID),
		GroupID:                      nullableUUIDString(value.GroupID),
		FullName:                     value.FullName,
		PassportNumber:               value.PassportNumber,
		Nationality:                  value.Nationality,
		DateOfBirth:                  value.DateOfBirth.Time,
		Gender:                       value.Gender,
		PhotoURL:                     value.PhotoUrl.String,
		Phone:                        value.Phone.String,
		EmergencyContact:             value.EmergencyContact.String,
		PreferredLang:                value.PreferredLang,
		MedicalNotes:                 value.MedicalNotes.String,
		RequiresWheelchair:           value.RequiresWheelchair,
		MahramID:                     nullableUUIDString(value.MahramID),
		AgentID:                      nullableUUIDString(value.AgentID),
		IsSubstituted:                value.IsSubstituted,
		SubstitutedByID:              nullableUUIDString(value.SubstitutedByID),
		AppAccessCode:                value.AppAccessCode,
		CreatedAt:                    value.CreatedAt.Time,
		UpdatedAt:                    value.UpdatedAt.Time,
		LastLat:                      float8Ptr(value.LastLat),
		LastLng:                      float8Ptr(value.LastLng),
		LastLocationAt:               timestamptzPtr(value.LastLocationAt),
		KloterID:                     nullableUUIDString(value.KloterID),
		Email:                        value.Email.String,
		HasAccount:                   value.LinkedUserID.Valid,
		PaymentStatus:                value.PaymentStatus,
		PaymentNotes:                 value.PaymentNotes,
		EmergencyContactName:         value.EmergencyContactName,
		EmergencyContactPhone:        value.EmergencyContactPhone,
		PassportExpiryDate:           datePtr(value.PassportExpiryDate),
		VaccineMeningitisDate:        datePtr(value.VaccineMeningitisDate),
		HotelCheckedIn:               value.HotelCheckedIn,
		DocumentsPassport:            value.DocumentsPassport,
		DocumentsPhoto:               value.DocumentsPhoto,
		DocumentsVaccine:             value.DocumentsVaccine,
		Status:                       value.Status,
		InsuranceProvider:            value.InsuranceProvider,
		InsurancePolicyNo:            value.InsurancePolicyNo,
		InsuranceClass:               value.InsuranceClass,
		BloodType:                    value.BloodType,
		ChronicConditions:            value.ChronicConditions,
		CurrentMedications:           value.CurrentMedications,
		NIK:                          value.Nik,
		Address:                      value.Address,
		KYCStatus:                    value.KycStatus,
		KYCSource:                    value.KycSource,
		KYCVerifiedBy:                value.KycVerifiedBy,
		KYCVerifiedAt:                timestamptzPtr(value.KycVerifiedAt),
		KYCRejectionReason:           value.KycRejectionReason,
		DocumentsKTP:                 value.DocumentsKtp,
		DocumentsSelfie:              value.DocumentsSelfie,
		PlaceOfBirth:                 value.PlaceOfBirth,
		MaritalStatus:                value.MaritalStatus,
		Occupation:                   value.Occupation,
		FatherName:                   value.FatherName,
		DocumentsKK:                  value.DocumentsKk,
		DocumentsMahramProof:         value.DocumentsMahramProof,
		InsuranceStartDate:           datePtr(value.InsuranceStartDate),
		InsuranceEndDate:             datePtr(value.InsuranceEndDate),
		InsuranceBeneficiaryName:     value.InsuranceBeneficiaryName,
		InsuranceBeneficiaryRelation: value.InsuranceBeneficiaryRelation,
		DocumentsVisa:                value.DocumentsVisa,
		VisaNumber:                   value.VisaNumber,
		VisaExpiryDate:               datePtr(value.VisaExpiryDate),
	}
}

func datePtr(value pgtype.Date) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time
	return &v
}

func float8Ptr(value pgtype.Float8) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

func uuidString(value pgtype.UUID) string {
	return uuid.UUID(value.Bytes).String()
}

// AssignKloterIfUnset is the TRAVEL_PACKAGE paid-order cascade (see
// OrderService) — a no-op if the pilgrim already has a kloter, so it never
// clobbers an existing (manual or earlier-package) assignment.
func (r *PilgrimRepository) AssignKloterIfUnset(ctx context.Context, operatorID, pilgrimID, kloterID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return err
	}
	kloterUUID, err := pgUUID(kloterID)
	if err != nil {
		return err
	}
	return r.queries.AssignKloterIfUnset(ctx, db.AssignKloterIfUnsetParams{ID: pilgrimUUID, OperatorID: opUUID, KloterID: kloterUUID})
}

func nullableUUIDString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuidString(value)
}
