package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PilgrimService struct {
	operatorRepository      *repository.OperatorRepository
	pilgrimRepository       *repository.PilgrimRepository
	accommodationRepository *repository.AccommodationRepository
	transportRepository     *repository.TransportRepository
	auditRepository         *repository.AuditRepository
	db                      *pgxpool.Pool
}

func NewPilgrimService(operatorRepository *repository.OperatorRepository, pilgrimRepository *repository.PilgrimRepository, accommodationRepository *repository.AccommodationRepository, transportRepository *repository.TransportRepository, auditRepository *repository.AuditRepository, db *pgxpool.Pool) *PilgrimService {
	return &PilgrimService{operatorRepository: operatorRepository, pilgrimRepository: pilgrimRepository, accommodationRepository: accommodationRepository, transportRepository: transportRepository, auditRepository: auditRepository, db: db}
}

// logActivity records a real, timestamped entry for the dashboard's Aktivitas
// Terbaru feed. Fire-and-forget by design — an audit-log failure must never
// fail the operation it's describing.
func (s *PilgrimService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "pilgrim", entityID, message)
}

func (s *PilgrimService) Create(ctx context.Context, authenticatedOrgID string, request *hajjv1.CreatePilgrimRequest) (*hajjv1.Pilgrim, error) {
	input, err := createInput(request)
	if err != nil {
		return nil, serviceError("PilgrimService.Create", err)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.Create", err)
	}
	_, err = s.pilgrimRepository.GetByPassport(ctx, operator.ID, input.SeasonID, input.PassportNumber)
	if err == nil {
		return nil, serviceError("PilgrimService.Create", apperror.ErrAlreadyExists)
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return nil, serviceError("PilgrimService.Create", err)
	}
	pilgrim, err := s.pilgrimRepository.Create(ctx, operator.ID, input)
	if err != nil {
		return nil, serviceError("PilgrimService.Create", err)
	}
	s.logActivity(ctx, operator.ID, "pilgrim_created", pilgrim.ID, fmt.Sprintf("Jamaah %s ditambahkan", pilgrim.FullName))
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) Get(ctx context.Context, authenticatedOrgID string, request *hajjv1.GetPilgrimRequest) (*hajjv1.Pilgrim, error) {
	if request == nil || !isUUID(request.GetPilgrimId()) {
		return nil, serviceError("PilgrimService.Get", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.Get", err)
	}
	pilgrim, err := s.pilgrimRepository.Get(ctx, operator.ID, request.PilgrimId)
	if err != nil {
		return nil, serviceError("PilgrimService.Get", err)
	}
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) List(ctx context.Context, authenticatedOrgID string, request *hajjv1.ListPilgrimsRequest) (*hajjv1.ListPilgrimsResponse, error) {
	if request == nil || !isUUID(request.GetSeasonId()) {
		return nil, serviceError("PilgrimService.List", apperror.ErrValidation)
	}
	if request.Limit <= 0 || request.Limit > 100 {
		request.Limit = 20
	}
	if request.Offset < 0 {
		request.Offset = 0
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.List", err)
	}
	pilgrims, err := s.pilgrimRepository.List(ctx, operator.ID, request.SeasonId, request.Limit, request.Offset)
	if err != nil {
		return nil, serviceError("PilgrimService.List", err)
	}
	count, err := s.pilgrimRepository.CountBySeason(ctx, operator.ID, request.SeasonId)
	if err != nil {
		return nil, serviceError("PilgrimService.List", err)
	}
	response := &hajjv1.ListPilgrimsResponse{Pilgrims: make([]*hajjv1.Pilgrim, 0, len(pilgrims)), TotalCount: count}
	for _, pilgrim := range pilgrims {
		response.Pilgrims = append(response.Pilgrims, pilgrimMessage(pilgrim))
	}
	return response, nil
}

func (s *PilgrimService) Stats(ctx context.Context, authenticatedOrgID string, request *hajjv1.GetPilgrimStatsRequest) (*hajjv1.PilgrimStats, error) {
	if request == nil || !isUUID(request.GetSeasonId()) || !validOptionalUUID(request.GetKloterId()) {
		return nil, serviceError("PilgrimService.Stats", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.Stats", err)
	}
	var stats domain.PilgrimStats
	if request.KloterId != "" {
		stats, err = s.pilgrimRepository.GetStatsByKloter(ctx, operator.ID, request.SeasonId, request.KloterId)
	} else {
		stats, err = s.pilgrimRepository.GetStats(ctx, operator.ID, request.SeasonId)
	}
	if err != nil {
		return nil, serviceError("PilgrimService.Stats", err)
	}
	return &hajjv1.PilgrimStats{Total: stats.Total, Substituted: stats.Substituted, RequiresWheelchair: stats.RequiresWheelchair, UnassignedGroup: stats.UnassignedGroup, UnassignedKloter: stats.UnassignedKloter}, nil
}

func (s *PilgrimService) Update(ctx context.Context, authenticatedOrgID string, request *hajjv1.UpdatePilgrimRequest) (*hajjv1.Pilgrim, error) {
	input, err := updateInput(request)
	if err != nil {
		return nil, serviceError("PilgrimService.Update", err)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.Update", err)
	}
	existing, err := s.pilgrimRepository.Get(ctx, operator.ID, request.PilgrimId)
	if err != nil {
		return nil, serviceError("PilgrimService.Update", err)
	}
	if input.PassportNumber != existing.PassportNumber {
		matched, lookupErr := s.pilgrimRepository.GetByPassport(ctx, operator.ID, existing.SeasonID, input.PassportNumber)
		if lookupErr == nil && matched.ID != existing.ID {
			return nil, serviceError("PilgrimService.Update", apperror.ErrAlreadyExists)
		}
		if lookupErr != nil && !errors.Is(lookupErr, apperror.ErrNotFound) {
			return nil, serviceError("PilgrimService.Update", lookupErr)
		}
	}
	pilgrim, err := s.pilgrimRepository.Update(ctx, operator.ID, request.PilgrimId, input)
	if err != nil {
		return nil, serviceError("PilgrimService.Update", err)
	}
	switch {
	case input.GroupID != existing.GroupID:
		s.logActivity(ctx, operator.ID, "pilgrim_group_changed", pilgrim.ID, fmt.Sprintf("Rombongan %s diperbarui", pilgrim.FullName))
	case input.KloterID != existing.KloterID:
		s.logActivity(ctx, operator.ID, "pilgrim_kloter_changed", pilgrim.ID, fmt.Sprintf("Kloter %s diperbarui", pilgrim.FullName))
	case input.RequiresWheelchair != existing.RequiresWheelchair:
		s.logActivity(ctx, operator.ID, "pilgrim_wheelchair_changed", pilgrim.ID, fmt.Sprintf("Kebutuhan kursi roda %s diperbarui", pilgrim.FullName))
	default:
		s.logActivity(ctx, operator.ID, "pilgrim_updated", pilgrim.ID, fmt.Sprintf("Data jamaah %s diperbarui", pilgrim.FullName))
	}
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) MarkSubstituted(ctx context.Context, authenticatedOrgID, pilgrimID string) (*hajjv1.Pilgrim, error) {
	if !isUUID(pilgrimID) {
		return nil, serviceError("PilgrimService.MarkSubstituted", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.MarkSubstituted", err)
	}
	if _, err := s.pilgrimRepository.Get(ctx, operator.ID, pilgrimID); err != nil {
		return nil, serviceError("PilgrimService.MarkSubstituted", err)
	}
	if err := s.pilgrimRepository.MarkSubstituted(ctx, operator.ID, pilgrimID); err != nil {
		return nil, serviceError("PilgrimService.MarkSubstituted", err)
	}
	pilgrim, err := s.pilgrimRepository.Get(ctx, operator.ID, pilgrimID)
	if err != nil {
		return nil, serviceError("PilgrimService.MarkSubstituted", err)
	}
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) SubstitutePilgrim(ctx context.Context, authenticatedOrgID, originalID, replacementID string) (*hajjv1.SubstitutePilgrimResult, error) {
	if !isUUID(originalID) || !isUUID(replacementID) {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", apperror.ErrValidation)
	}
	if originalID == replacementID {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", preconditionError("original and replacement pilgrims must be different"))
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	original, err := s.pilgrimRepository.GetTx(ctx, tx, operator.ID, originalID)
	if err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	replacement, err := s.pilgrimRepository.GetTx(ctx, tx, operator.ID, replacementID)
	if err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	if original.SeasonID != replacement.SeasonID {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", preconditionError("replacement pilgrim must belong to the same season"))
	}
	if original.IsSubstituted {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", preconditionError("original pilgrim is already substituted"))
	}
	if replacement.IsSubstituted {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", preconditionError("replacement pilgrim is already substituted"))
	}
	if err := s.pilgrimRepository.SubstitutePilgrimTx(ctx, tx, originalID, replacementID, operator.ID); err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	if original.GroupID != "" {
		if err := s.pilgrimRepository.TransferPilgrimGroupTx(ctx, tx, replacementID, original.GroupID, operator.ID); err != nil {
			return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
		}
	}
	if err := s.accommodationRepository.TransferAllocationTx(ctx, tx, originalID, replacementID, operator.ID); err != nil && !errors.Is(err, apperror.ErrNotFound) {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	if err := s.transportRepository.UnassignPilgrimAllSeatsTx(ctx, tx, operator.ID, originalID); err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	if err := s.pilgrimRepository.WriteAuditLogTx(ctx, tx, operator.ID, middleware.UserIDFromCtx(ctx), "substitution", originalID, fmt.Sprintf("pilgrim %s substituted by %s", originalID, replacementID)); err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	updated, err := s.pilgrimRepository.Get(ctx, operator.ID, originalID)
	if err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	newPilgrim, err := s.pilgrimRepository.Get(ctx, operator.ID, replacementID)
	if err != nil {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", err)
	}
	return &hajjv1.SubstitutePilgrimResult{Original: pilgrimMessage(updated), Replacement: pilgrimMessage(newPilgrim)}, nil
}

func preconditionError(message string) error {
	return fmt.Errorf("%s: %w", message, apperror.ErrFailedPrecondition)
}

func createInput(request *hajjv1.CreatePilgrimRequest) (domain.PilgrimInput, error) {
	if request == nil || !isUUID(request.GetSeasonId()) || request.GetFullName() == "" || request.GetPassportNumber() == "" || request.GetNationality() == "" || request.GetDateOfBirth() == nil || !validGender(request.GetGender()) || !validOptionalUUID(request.GetGroupId()) || !validOptionalUUID(request.GetMahramId()) || !validOptionalUUID(request.GetKloterId()) {
		return domain.PilgrimInput{}, apperror.ErrValidation
	}
	return pilgrimInput(request.SeasonId, request.GroupId, request.FullName, request.PassportNumber, request.Nationality, request.DateOfBirth.AsTime(), request.Gender, request.PhotoUrl, request.Phone, request.EmergencyContact, request.PreferredLang, request.MedicalNotes, request.RequiresWheelchair, request.MahramId, request.KloterId), nil
}

func updateInput(request *hajjv1.UpdatePilgrimRequest) (domain.PilgrimInput, error) {
	if request == nil || !isUUID(request.GetPilgrimId()) || request.GetFullName() == "" || request.GetPassportNumber() == "" || request.GetNationality() == "" || request.GetDateOfBirth() == nil || !validGender(request.GetGender()) || !validOptionalUUID(request.GetGroupId()) || !validOptionalUUID(request.GetMahramId()) || !validOptionalUUID(request.GetKloterId()) {
		return domain.PilgrimInput{}, apperror.ErrValidation
	}
	return pilgrimInput("", request.GroupId, request.FullName, request.PassportNumber, request.Nationality, request.DateOfBirth.AsTime(), request.Gender, request.PhotoUrl, request.Phone, request.EmergencyContact, request.PreferredLang, request.MedicalNotes, request.RequiresWheelchair, request.MahramId, request.KloterId), nil
}

func pilgrimInput(seasonID, groupID, fullName, passportNumber, nationality string, dateOfBirth time.Time, gender hajjv1.Gender, photoURL, phone, emergencyContact, preferredLang, medicalNotes string, requiresWheelchair bool, mahramID, kloterID string) domain.PilgrimInput {
	if preferredLang == "" {
		preferredLang = "ar"
	}
	return domain.PilgrimInput{SeasonID: seasonID, GroupID: groupID, FullName: fullName, PassportNumber: passportNumber, Nationality: nationality, DateOfBirth: dateOfBirth, Gender: gender.String()[7:], PhotoURL: photoURL, Phone: phone, EmergencyContact: emergencyContact, PreferredLang: preferredLang, MedicalNotes: medicalNotes, RequiresWheelchair: requiresWheelchair, MahramID: mahramID, KloterID: kloterID}
}

func validGender(value hajjv1.Gender) bool {
	return value == hajjv1.Gender_GENDER_MALE || value == hajjv1.Gender_GENDER_FEMALE
}

func isUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func validOptionalUUID(value string) bool {
	return value == "" || isUUID(value)
}

func pilgrimMessage(value *domain.Pilgrim) *hajjv1.Pilgrim {
	gender := hajjv1.Gender_GENDER_MALE
	if value.Gender == "FEMALE" {
		gender = hajjv1.Gender_GENDER_FEMALE
	}
	return &hajjv1.Pilgrim{Id: value.ID, SeasonId: value.SeasonID, OperatorId: value.OperatorID, GroupId: value.GroupID, FullName: value.FullName, PassportNumber: value.PassportNumber, Nationality: value.Nationality, DateOfBirth: timestamppb.New(value.DateOfBirth), Gender: gender, PhotoUrl: value.PhotoURL, Phone: value.Phone, EmergencyContact: value.EmergencyContact, PreferredLang: value.PreferredLang, MedicalNotes: value.MedicalNotes, RequiresWheelchair: value.RequiresWheelchair, MahramId: value.MahramID, IsSubstituted: value.IsSubstituted, SubstitutedById: value.SubstitutedByID, AppAccessCode: value.AppAccessCode, CreatedAt: timestamppb.New(value.CreatedAt), UpdatedAt: timestamppb.New(value.UpdatedAt), KloterId: value.KloterID}
}
