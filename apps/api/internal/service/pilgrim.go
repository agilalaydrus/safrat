package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	// A single record, so the exact subject is recorded. This is the precise
	// answer a breach notification needs, and the one nothing else can supply
	// after the fact.
	s.recordRead(ctx, operator.ID, "pilgrim_read", request.PilgrimId, "")
	return pilgrimMessage(pilgrim), nil
}

// recordRead writes an access trail for personal data.
//
// Failure is swallowed on purpose. This runs on a read path: refusing to show a
// jamaah their own record because an audit row could not be written would break
// the application to protect a log, which is the wrong way round. A missing
// entry weakens a future investigation; a failed read breaks today's work.
//
// The trade is worth stating rather than assuming, because it means the trail
// is best-effort and a breach report should say so.
func (s *PilgrimService) recordRead(ctx context.Context, operatorID, action, entityID, note string) {
	if s.auditRepository == nil {
		return
	}
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx),
		action, "pilgrim", entityID, note)
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
	// Recorded because of what has to be answerable within 72 hours of a
	// breach (UU PDP art. 46): not "some data may have been exposed" but
	// "these records, read by this account, at this time". Without a read
	// trail the only honest answer is "everyone", and every jamaah has to be
	// notified.
	//
	// One row per request carrying the scope and the count, not one per
	// pilgrim. The season plus the count is enough to enumerate who was
	// reachable, and a per-pilgrim log would multiply volume by twenty while
	// making the access log itself a more sensitive thing to hold.
	s.recordRead(ctx, operator.ID, "pilgrims_listed", request.SeasonId,
		fmt.Sprintf("%d catatan dibaca", len(pilgrims)))

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

func (s *PilgrimService) RegenerateAccessCode(ctx context.Context, authenticatedOrgID, pilgrimID string) (*hajjv1.Pilgrim, error) {
	if !isUUID(pilgrimID) {
		return nil, serviceError("PilgrimService.RegenerateAccessCode", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.RegenerateAccessCode", err)
	}
	if _, err := s.pilgrimRepository.Get(ctx, operator.ID, pilgrimID); err != nil {
		return nil, serviceError("PilgrimService.RegenerateAccessCode", err)
	}
	if err := s.pilgrimRepository.RegenerateAccessCode(ctx, operator.ID, pilgrimID); err != nil {
		return nil, serviceError("PilgrimService.RegenerateAccessCode", err)
	}
	pilgrim, err := s.pilgrimRepository.Get(ctx, operator.ID, pilgrimID)
	if err != nil {
		return nil, serviceError("PilgrimService.RegenerateAccessCode", err)
	}
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) UpdatePayment(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdatePilgrimPaymentRequest) (*hajjv1.Pilgrim, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("PilgrimService.UpdatePayment", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.UpdatePayment", err)
	}
	pilgrim, err := s.pilgrimRepository.UpdatePayment(ctx, operator.ID, req.PilgrimId, req.PaymentStatus, req.PaymentNotes)
	if err != nil {
		return nil, serviceError("PilgrimService.UpdatePayment", err)
	}
	s.logActivity(ctx, operator.ID, "pilgrim_payment_updated", pilgrim.ID, fmt.Sprintf("%s: status pembayaran diubah ke %s", pilgrim.FullName, req.PaymentStatus))
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) UpdateDocuments(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdatePilgrimDocumentsRequest) (*hajjv1.Pilgrim, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("PilgrimService.UpdateDocuments", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.UpdateDocuments", err)
	}
	input := domain.PilgrimDocumentChecklistInput{
		Passport: req.DocumentsPassport, Photo: req.DocumentsPhoto, Vaccine: req.DocumentsVaccine,
		KTP: req.DocumentsKtp, KK: req.DocumentsKk, MahramProof: req.DocumentsMahramProof,
		Visa: req.DocumentsVisa, VisaNumber: req.VisaNumber,
	}
	if req.PassportExpiryDate != nil {
		t := req.PassportExpiryDate.AsTime()
		input.PassportExpiry = &t
	}
	if req.VaccineMeningitisDate != nil {
		t := req.VaccineMeningitisDate.AsTime()
		input.VaccineDate = &t
	}
	if req.VisaExpiryDate != nil {
		t := req.VisaExpiryDate.AsTime()
		input.VisaExpiry = &t
	}
	pilgrim, err := s.pilgrimRepository.UpdateDocuments(ctx, operator.ID, req.PilgrimId, input)
	if err != nil {
		return nil, serviceError("PilgrimService.UpdateDocuments", err)
	}
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) UpdateEmergencyContact(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdatePilgrimEmergencyContactRequest) (*hajjv1.Pilgrim, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("PilgrimService.UpdateEmergencyContact", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.UpdateEmergencyContact", err)
	}
	pilgrim, err := s.pilgrimRepository.UpdateEmergencyContact(ctx, operator.ID, req.PilgrimId, req.EmergencyContactName, req.EmergencyContactPhone)
	if err != nil {
		return nil, serviceError("PilgrimService.UpdateEmergencyContact", err)
	}
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) UpdateInsurance(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdatePilgrimInsuranceRequest) (*hajjv1.Pilgrim, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("PilgrimService.UpdateInsurance", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.UpdateInsurance", err)
	}
	input := domain.PilgrimInsuranceInput{
		Provider: req.InsuranceProvider, PolicyNo: req.InsurancePolicyNo, Class: req.InsuranceClass,
		BloodType: req.BloodType, ChronicConditions: req.ChronicConditions, CurrentMedications: req.CurrentMedications,
		BeneficiaryName: req.InsuranceBeneficiaryName, BeneficiaryRelation: req.InsuranceBeneficiaryRelation,
	}
	if req.InsuranceStartDate != nil {
		t := req.InsuranceStartDate.AsTime()
		input.StartDate = &t
	}
	if req.InsuranceEndDate != nil {
		t := req.InsuranceEndDate.AsTime()
		input.EndDate = &t
	}
	pilgrim, err := s.pilgrimRepository.UpdateInsurance(ctx, operator.ID, req.PilgrimId, input)
	if err != nil {
		return nil, serviceError("PilgrimService.UpdateInsurance", err)
	}
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) CheckInHotel(ctx context.Context, authenticatedOrgID string, req *hajjv1.CheckInPilgrimHotelRequest) (*hajjv1.Pilgrim, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("PilgrimService.CheckInHotel", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.CheckInHotel", err)
	}
	pilgrim, err := s.pilgrimRepository.CheckInHotel(ctx, operator.ID, req.PilgrimId, req.CheckedIn)
	if err != nil {
		return nil, serviceError("PilgrimService.CheckInHotel", err)
	}
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) ListWithExpiringPassports(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListPilgrimsWithExpiringPassportsRequest) (*hajjv1.ListPilgrimsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) || req.DaysThreshold <= 0 {
		return nil, serviceError("PilgrimService.ListWithExpiringPassports", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.ListWithExpiringPassports", err)
	}
	before := time.Now().AddDate(0, 0, int(req.DaysThreshold))
	pilgrims, err := s.pilgrimRepository.ListWithExpiringPassports(ctx, operator.ID, req.SeasonId, before)
	if err != nil {
		return nil, serviceError("PilgrimService.ListWithExpiringPassports", err)
	}
	result := &hajjv1.ListPilgrimsResponse{Pilgrims: make([]*hajjv1.Pilgrim, 0, len(pilgrims))}
	for _, pilgrim := range pilgrims {
		result.Pilgrims = append(result.Pilgrims, pilgrimMessage(pilgrim))
	}
	return result, nil
}

var validDocTypes = map[string]bool{"PASSPORT": true, "PHOTO": true, "VACCINE": true, "KTP": true, "SELFIE": true, "KK": true, "MAHRAM_PROOF": true, "VISA": true, "PAYMENT_RECEIPT": true, "OTHER": true}

// CreateDocument is called from the plain HTTP multipart upload endpoint
// (see main.go), not a Connect handler — it still goes through the service
// layer so operator scoping and doc_type validation aren't duplicated.
func (s *PilgrimService) CreateDocument(ctx context.Context, authenticatedOrgID, pilgrimID, docType, fileURL, fileName string) (*domain.PilgrimDocument, error) {
	if !isUUID(pilgrimID) || !validDocTypes[docType] || fileURL == "" || fileName == "" {
		return nil, serviceError("PilgrimService.CreateDocument", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.CreateDocument", err)
	}
	if _, err := s.pilgrimRepository.Get(ctx, operator.ID, pilgrimID); err != nil {
		return nil, serviceError("PilgrimService.CreateDocument", err)
	}
	document, err := s.pilgrimRepository.CreateDocument(ctx, operator.ID, pilgrimID, docType, fileURL, fileName, "operator")
	if err != nil {
		return nil, serviceError("PilgrimService.CreateDocument", err)
	}
	return document, nil
}

// CreateDocumentSelf is the pilgrim self-upload counterpart to
// CreateDocument — called from the plain HTTP multipart endpoint
// authenticated by app_access_code (no session, no org), same as the rest
// of the pilgrim-facing surface. uploaded_by is "pilgrim" so the admin
// dashboard can tell self-submitted documents apart from operator uploads.
func (s *PilgrimService) CreateDocumentSelf(ctx context.Context, appAccessCode, docType, fileURL, fileName string) (*domain.PilgrimDocument, error) {
	if strings.TrimSpace(appAccessCode) == "" || !validDocTypes[docType] || fileURL == "" || fileName == "" {
		return nil, serviceError("PilgrimService.CreateDocumentSelf", apperror.ErrValidation)
	}
	pilgrim, err := s.pilgrimRepository.GetByAppAccessCode(ctx, appAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimService.CreateDocumentSelf", apperror.ErrNotFound)
	}
	document, err := s.pilgrimRepository.CreateDocument(ctx, pilgrim.OperatorID, pilgrim.ID, docType, fileURL, fileName, "pilgrim")
	if err != nil {
		return nil, serviceError("PilgrimService.CreateDocumentSelf", err)
	}
	return document, nil
}

func (s *PilgrimService) ListDocuments(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListPilgrimDocumentsRequest) (*hajjv1.ListPilgrimDocumentsResponse, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("PilgrimService.ListDocuments", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.ListDocuments", err)
	}
	if _, err := s.pilgrimRepository.Get(ctx, operator.ID, req.PilgrimId); err != nil {
		return nil, serviceError("PilgrimService.ListDocuments", err)
	}
	documents, err := s.pilgrimRepository.ListDocuments(ctx, req.PilgrimId)
	if err != nil {
		return nil, serviceError("PilgrimService.ListDocuments", err)
	}
	result := &hajjv1.ListPilgrimDocumentsResponse{Documents: make([]*hajjv1.PilgrimDocument, 0, len(documents))}
	for _, document := range documents {
		result.Documents = append(result.Documents, pilgrimDocumentMessage(document))
	}
	return result, nil
}

func (s *PilgrimService) DeleteDocument(ctx context.Context, authenticatedOrgID string, req *hajjv1.DeletePilgrimDocumentRequest) (*hajjv1.DeletePilgrimDocumentResponse, error) {
	if req == nil || !isUUID(req.Id) {
		return nil, serviceError("PilgrimService.DeleteDocument", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.DeleteDocument", err)
	}
	if err := s.pilgrimRepository.DeleteDocument(ctx, operator.ID, req.Id); err != nil {
		return nil, serviceError("PilgrimService.DeleteDocument", err)
	}
	return &hajjv1.DeletePilgrimDocumentResponse{}, nil
}

func (s *PilgrimService) ListSeasonDocuments(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListSeasonDocumentsRequest) (*hajjv1.ListSeasonDocumentsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("PilgrimService.ListSeasonDocuments", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.ListSeasonDocuments", err)
	}
	documents, err := s.pilgrimRepository.ListSeasonDocuments(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("PilgrimService.ListSeasonDocuments", err)
	}
	result := &hajjv1.ListSeasonDocumentsResponse{Documents: make([]*hajjv1.PilgrimDocument, 0, len(documents))}
	for _, document := range documents {
		result.Documents = append(result.Documents, pilgrimDocumentMessage(document))
	}
	return result, nil
}

func pilgrimDocumentMessage(value *domain.PilgrimDocument) *hajjv1.PilgrimDocument {
	return &hajjv1.PilgrimDocument{
		Id: value.ID, PilgrimId: value.PilgrimID, DocType: value.DocType,
		FileUrl: value.FileURL, FileName: value.FileName, UploadedBy: value.UploadedBy,
		CreatedAt: timestamppb.New(value.CreatedAt), PilgrimName: value.PilgrimName, PassportNumber: value.PassportNumber,
	}
}

func (s *PilgrimService) ListSubstitutions(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListSubstitutionsRequest) (*hajjv1.ListSubstitutionsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("PilgrimService.ListSubstitutions", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.ListSubstitutions", err)
	}
	rows, err := s.pilgrimRepository.ListSubstitutions(ctx, operator.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("PilgrimService.ListSubstitutions", err)
	}
	result := &hajjv1.ListSubstitutionsResponse{Substitutions: make([]*hajjv1.Substitution, 0, len(rows))}
	for _, row := range rows {
		result.Substitutions = append(result.Substitutions, &hajjv1.Substitution{
			OriginalId: row.OriginalID, OriginalName: row.OriginalName, OriginalPassportNumber: row.OriginalPassportNumber,
			NewId: row.NewID, NewName: row.NewName, Reason: row.Reason, SubstitutedAt: timestamppb.New(row.SubstitutedAt),
		})
	}
	return result, nil
}

func (s *PilgrimService) SubstitutePilgrim(ctx context.Context, authenticatedOrgID, originalID, replacementID, reason string) (*hajjv1.SubstitutePilgrimResult, error) {
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
	if original.Status == "CANCELLED" || replacement.Status == "CANCELLED" {
		return nil, serviceError("PilgrimService.SubstitutePilgrim", preconditionError("cancelled pilgrims cannot be substituted"))
	}
	if err := s.pilgrimRepository.SubstitutePilgrimTx(ctx, tx, originalID, replacementID, operator.ID, reason); err != nil {
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
	if request == nil || !isUUID(request.GetSeasonId()) || request.GetFullName() == "" || request.GetPassportNumber() == "" || request.GetNationality() == "" || request.GetDateOfBirth() == nil || !validGender(request.GetGender()) || !validOptionalUUID(request.GetGroupId()) || !validOptionalUUID(request.GetMahramId()) || !validOptionalUUID(request.GetKloterId()) || !validOptionalUUID(request.GetAgentId()) {
		return domain.PilgrimInput{}, apperror.ErrValidation
	}
	return pilgrimInput(request.SeasonId, request.GroupId, request.FullName, request.PassportNumber, request.Nationality, request.DateOfBirth.AsTime(), request.Gender, request.PhotoUrl, request.Phone, request.EmergencyContact, request.PreferredLang, request.MedicalNotes, request.RequiresWheelchair, request.MahramId, request.KloterId, request.Email, request.AgentId), nil
}

func updateInput(request *hajjv1.UpdatePilgrimRequest) (domain.PilgrimInput, error) {
	if request == nil || !isUUID(request.GetPilgrimId()) || request.GetFullName() == "" || request.GetPassportNumber() == "" || request.GetNationality() == "" || request.GetDateOfBirth() == nil || !validGender(request.GetGender()) || !validOptionalUUID(request.GetGroupId()) || !validOptionalUUID(request.GetMahramId()) || !validOptionalUUID(request.GetKloterId()) {
		return domain.PilgrimInput{}, apperror.ErrValidation
	}
	return pilgrimInput("", request.GroupId, request.FullName, request.PassportNumber, request.Nationality, request.DateOfBirth.AsTime(), request.Gender, request.PhotoUrl, request.Phone, request.EmergencyContact, request.PreferredLang, request.MedicalNotes, request.RequiresWheelchair, request.MahramId, request.KloterId, request.Email, ""), nil
}

func pilgrimInput(seasonID, groupID, fullName, passportNumber, nationality string, dateOfBirth time.Time, gender hajjv1.Gender, photoURL, phone, emergencyContact, preferredLang, medicalNotes string, requiresWheelchair bool, mahramID, kloterID, email, agentID string) domain.PilgrimInput {
	if preferredLang == "" {
		preferredLang = "ar"
	}
	return domain.PilgrimInput{SeasonID: seasonID, GroupID: groupID, FullName: fullName, PassportNumber: passportNumber, Nationality: nationality, DateOfBirth: dateOfBirth, Gender: gender.String()[7:], PhotoURL: photoURL, Phone: phone, EmergencyContact: emergencyContact, PreferredLang: preferredLang, MedicalNotes: medicalNotes, RequiresWheelchair: requiresWheelchair, MahramID: mahramID, KloterID: kloterID, Email: email, AgentID: agentID}
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
	result := &hajjv1.Pilgrim{Id: value.ID, SeasonId: value.SeasonID, OperatorId: value.OperatorID, GroupId: value.GroupID, FullName: value.FullName, PassportNumber: value.PassportNumber, Nationality: value.Nationality, DateOfBirth: timestamppb.New(value.DateOfBirth), Gender: gender, PhotoUrl: value.PhotoURL, Phone: value.Phone, EmergencyContact: value.EmergencyContact, PreferredLang: value.PreferredLang, MedicalNotes: value.MedicalNotes, RequiresWheelchair: value.RequiresWheelchair, MahramId: value.MahramID, IsSubstituted: value.IsSubstituted, SubstitutedById: value.SubstitutedByID, AppAccessCode: value.AppAccessCode, CreatedAt: timestamppb.New(value.CreatedAt), UpdatedAt: timestamppb.New(value.UpdatedAt), KloterId: value.KloterID, Email: value.Email, HasAccount: value.HasAccount,
		PaymentStatus: value.PaymentStatus, PaymentNotes: value.PaymentNotes,
		EmergencyContactName: value.EmergencyContactName, EmergencyContactPhone: value.EmergencyContactPhone,
		HotelCheckedIn: value.HotelCheckedIn, DocumentsPassport: value.DocumentsPassport, DocumentsPhoto: value.DocumentsPhoto, DocumentsVaccine: value.DocumentsVaccine,
		Status:            value.Status,
		InsuranceProvider: value.InsuranceProvider, InsurancePolicyNo: value.InsurancePolicyNo, InsuranceClass: value.InsuranceClass,
		BloodType: value.BloodType, ChronicConditions: value.ChronicConditions, CurrentMedications: value.CurrentMedications,
		Nik: value.NIK, Address: value.Address, KycStatus: value.KYCStatus, KycSource: value.KYCSource,
		KycVerifiedBy: value.KYCVerifiedBy, KycRejectionReason: value.KYCRejectionReason,
		DocumentsKtp: value.DocumentsKTP, DocumentsSelfie: value.DocumentsSelfie,
		PlaceOfBirth: value.PlaceOfBirth, MaritalStatus: value.MaritalStatus, Occupation: value.Occupation, FatherName: value.FatherName,
		DocumentsKk: value.DocumentsKK, DocumentsMahramProof: value.DocumentsMahramProof,
		InsuranceBeneficiaryName: value.InsuranceBeneficiaryName, InsuranceBeneficiaryRelation: value.InsuranceBeneficiaryRelation,
		DocumentsVisa: value.DocumentsVisa, VisaNumber: value.VisaNumber,
	}
	if value.PassportExpiryDate != nil {
		result.PassportExpiryDate = timestamppb.New(*value.PassportExpiryDate)
	}
	if value.VaccineMeningitisDate != nil {
		result.VaccineMeningitisDate = timestamppb.New(*value.VaccineMeningitisDate)
	}
	if value.InsuranceStartDate != nil {
		result.InsuranceStartDate = timestamppb.New(*value.InsuranceStartDate)
	}
	if value.InsuranceEndDate != nil {
		result.InsuranceEndDate = timestamppb.New(*value.InsuranceEndDate)
	}
	if value.VisaExpiryDate != nil {
		result.VisaExpiryDate = timestamppb.New(*value.VisaExpiryDate)
	}
	if value.KYCVerifiedAt != nil {
		result.KycVerifiedAt = timestamppb.New(*value.KYCVerifiedAt)
	}
	return result
}

func (s *PilgrimService) UpdateKyc(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdatePilgrimKycRequest) (*hajjv1.Pilgrim, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("PilgrimService.UpdateKyc", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.UpdateKyc", err)
	}
	input := domain.PilgrimKYCInput{NIK: req.Nik, Address: req.Address, PlaceOfBirth: req.PlaceOfBirth, MaritalStatus: req.MaritalStatus, Occupation: req.Occupation, FatherName: req.FatherName}
	pilgrim, err := s.pilgrimRepository.UpdateKYC(ctx, operator.ID, req.PilgrimId, input, "ADMIN")
	if err != nil {
		return nil, serviceError("PilgrimService.UpdateKyc", err)
	}
	return pilgrimMessage(pilgrim), nil
}

func (s *PilgrimService) VerifyKyc(ctx context.Context, authenticatedOrgID, verifiedBy string, req *hajjv1.VerifyPilgrimKycRequest) (*hajjv1.Pilgrim, error) {
	if req == nil || !isUUID(req.PilgrimId) {
		return nil, serviceError("PilgrimService.VerifyKyc", apperror.ErrValidation)
	}
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("PilgrimService.VerifyKyc", err)
	}
	pilgrim, err := s.pilgrimRepository.VerifyKYC(ctx, operator.ID, req.PilgrimId, verifiedBy, req.Approve, req.RejectionReason)
	if err != nil {
		return nil, serviceError("PilgrimService.VerifyKyc", err)
	}
	return pilgrimMessage(pilgrim), nil
}
