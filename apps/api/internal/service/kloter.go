package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/events"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type KloterService struct {
	operatorRepository *repository.OperatorRepository
	kloterRepository   *repository.KloterRepository
	auditRepository    *repository.AuditRepository
	outboxRepository   *repository.OutboxRepository
	db                 *pgxpool.Pool
	eventBus           *events.Bus
}

func NewKloterService(operators *repository.OperatorRepository, kloters *repository.KloterRepository, audit *repository.AuditRepository, outbox *repository.OutboxRepository, db *pgxpool.Pool, bus *events.Bus) *KloterService {
	return &KloterService{operatorRepository: operators, kloterRepository: kloters, auditRepository: audit, outboxRepository: outbox, db: db, eventBus: bus}
}

// kloterStatusPushText is the "seluruh jamaah kloter" push copy per the
// cascade map in SPEC_GROUP_KLOTER_MUTTAWWIF.md section 4.
var kloterStatusPushText = map[string]string{
	"DEPARTED":       "Perjalanan ibadah Anda dimulai",
	"IN_SAUDI":       "Selamat tiba di Tanah Suci",
	"DEPARTED_SAUDI": "Dalam perjalanan pulang ke Indonesia",
	"COMPLETED":      "Selamat tiba! Sertifikat Anda siap diunduh",
}

// kloterStatusOrder is forward-only, except the one explicit rollback
// business rule: CONFIRMED -> DRAFT (e.g. an operator undoes an early
// confirmation before departure prep has really started).
var kloterStatusOrder = []string{"DRAFT", "CONFIRMED", "DEPARTED", "IN_SAUDI", "DEPARTED_SAUDI", "COMPLETED"}

func kloterStatusIndex(status string) int {
	for i, s := range kloterStatusOrder {
		if s == status {
			return i
		}
	}
	return -1
}

// kloterToJourneyStatus is the cascade map from section 4 of
// SPEC_GROUP_KLOTER_MUTTAWWIF.md — only kloter transitions with a direct
// 1:1 journey-status equivalent cascade; DRAFT/CONFIRMED don't.
var kloterToJourneyStatus = map[string]string{
	"DEPARTED":       "DEPARTED_INDONESIA",
	"IN_SAUDI":       "ARRIVED_SAUDI",
	"DEPARTED_SAUDI": "DEPARTED_SAUDI",
	"COMPLETED":      "ARRIVED_INDONESIA",
}

func (s *KloterService) logActivity(ctx context.Context, operatorID, action, entityID, message string) {
	_ = s.auditRepository.Write(ctx, operatorID, middleware.UserIDFromCtx(ctx), action, "kloter", entityID, message)
}

func (s *KloterService) ListKloters(ctx context.Context, orgID string, req *hajjv1.ListKlotersRequest) (*hajjv1.ListKlotersResponse, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("KloterService.ListKloters", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.ListKloters", err)
	}
	kloters, err := s.kloterRepository.ListForOperator(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("KloterService.ListKloters", err)
	}
	result := &hajjv1.ListKlotersResponse{Kloters: make([]*hajjv1.Kloter, 0, len(kloters))}
	for _, k := range kloters {
		result.Kloters = append(result.Kloters, kloterMessage(k))
	}
	return result, nil
}

func (s *KloterService) CreateKloter(ctx context.Context, orgID string, req *hajjv1.CreateKloterRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("KloterService.CreateKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.CreateKloter", err)
	}
	kloter, err := s.kloterRepository.Create(ctx, op.ID, req.SeasonId, req.Code, req.Embarkation, req.FlightNumber, timestampPtr(req.DepartureDate), req.Capacity)
	if err != nil {
		return nil, serviceError("KloterService.CreateKloter", err)
	}
	s.logActivity(ctx, op.ID, "kloter_created", kloter.ID, fmt.Sprintf("Kloter %s dibuat", kloter.Code))
	return kloterMessage(kloter), nil
}

func (s *KloterService) UpdateKloter(ctx context.Context, orgID string, req *hajjv1.UpdateKloterRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" || strings.TrimSpace(req.Code) == "" {
		return nil, serviceError("KloterService.UpdateKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloter", err)
	}
	kloter, err := s.kloterRepository.Update(ctx, op.ID, req.KloterId, req.Code, req.Embarkation, req.FlightNumber, timestampPtr(req.DepartureDate), req.Capacity)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloter", err)
	}
	s.logActivity(ctx, op.ID, "kloter_updated", kloter.ID, fmt.Sprintf("Kloter %s diperbarui", kloter.Code))
	return kloterMessage(kloter), nil
}

func (s *KloterService) UpdateKloterStatus(ctx context.Context, orgID string, req *hajjv1.UpdateKloterStatusRequest) (*hajjv1.Kloter, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" || strings.TrimSpace(req.Status) == "" {
		return nil, serviceError("KloterService.UpdateKloterStatus", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloterStatus", err)
	}
	kloter, err := s.updateStatus(ctx, op.ID, req.KloterId, req.Status)
	if err != nil {
		return nil, serviceError("KloterService.UpdateKloterStatus", err)
	}
	return kloterMessage(kloter), nil
}

// updateStatus is the operator-independent core, reused by
// TripService.UpdateTripKloterStatus (a Tour Leader assigned to the kloter
// via kloter_staff — already resolved its own operatorID via
// authorizeKloter, no need to re-derive it here).
func (s *KloterService) updateStatus(ctx context.Context, operatorID, kloterID, status string) (*domain.Kloter, error) {
	targetIdx := kloterStatusIndex(status)
	if targetIdx == -1 {
		return nil, apperror.ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := s.kloterRepository.GetForOperatorForUpdateTx(ctx, tx, operatorID, kloterID)
	if err != nil {
		return nil, err
	}
	currentIdx := kloterStatusIndex(current.Status)
	isRollback := current.Status == "CONFIRMED" && status == "DRAFT"
	if !isRollback && targetIdx <= currentIdx {
		return nil, errors.New("status kloter hanya bisa maju")
	}
	kloter, err := s.kloterRepository.UpdateStatusTx(ctx, tx, operatorID, kloterID, status)
	if err != nil {
		return nil, err
	}
	journeyStatus := kloterToJourneyStatus[status]
	if err := s.outboxRepository.EnqueueTx(ctx, tx, operatorID, domain.EventKloterStatusUpdated, kloter.ID, domain.KloterStatusUpdatedPayload{
		KloterID: kloter.ID, KloterCode: kloter.Code, Status: status, JourneyStatus: journeyStatus, UpdatedBy: middleware.UserIDFromCtx(ctx), NotificationBody: kloterStatusPushText[status],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.logActivity(ctx, operatorID, "kloter_status_changed", kloter.ID, fmt.Sprintf("Status kloter %s: %s -> %s", kloter.Code, current.Status, status))
	s.eventBus.Publish(operatorID, "kloter_status", kloter.ID)
	return kloter, nil
}

func (s *KloterService) DeleteKloter(ctx context.Context, orgID string, req *hajjv1.DeleteKloterRequest) (*emptypb.Empty, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.DeleteKloter", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.DeleteKloter", err)
	}
	if err := s.kloterRepository.Delete(ctx, op.ID, req.KloterId); err != nil {
		return nil, serviceError("KloterService.DeleteKloter", err)
	}
	return &emptypb.Empty{}, nil
}

func kloterMessage(k *domain.Kloter) *hajjv1.Kloter {
	msg := &hajjv1.Kloter{Id: k.ID, SeasonId: k.SeasonID, Code: k.Code, Embarkation: k.Embarkation, FlightNumber: k.FlightNumber, Capacity: k.Capacity, PilgrimCount: k.PilgrimCount, Status: k.Status, Notes: k.Notes}
	if k.DepartureDate != nil {
		msg.DepartureDate = timestamppb.New(*k.DepartureDate)
	}
	return msg
}

// GetManifest builds the list that leaves the office.
//
// The readiness figures are computed here rather than in SQL so that "what is
// missing" and "how many are ready" come from one definition of required — the
// same one the row itself reports.
func (s *KloterService) GetManifest(ctx context.Context, orgID string, req *hajjv1.GetKloterManifestRequest) (*hajjv1.GetKloterManifestResponse, error) {
	if req == nil || strings.TrimSpace(req.KloterId) == "" {
		return nil, serviceError("KloterService.GetManifest", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("KloterService.GetManifest", err)
	}
	manifest, err := s.kloterRepository.Manifest(ctx, op.ID, req.KloterId)
	if err != nil {
		return nil, serviceError("KloterService.GetManifest", err)
	}

	response := &hajjv1.GetKloterManifestResponse{
		KloterId: manifest.KloterID, KloterCode: manifest.Code,
		FlightNumber: manifest.FlightNumber, Embarkation: manifest.Embarkation,
		Capacity: manifest.Capacity, MissingByDocument: map[string]int32{},
	}
	if manifest.DepartureDate != nil {
		response.DepartureDate = timestamppb.New(*manifest.DepartureDate)
	}
	for _, row := range manifest.Rows {
		missing := row.MissingDocuments()
		message := &hajjv1.ManifestRow{
			PilgrimId: row.PilgrimID, FullName: row.FullName, PassportNumber: row.PassportNumber,
			Gender: row.Gender, GroupName: row.GroupName, RoomLabel: row.RoomLabel, Phone: row.Phone,
			HasPassport: row.HasPassport, HasVaccine: row.HasVaccine, HasPhoto: row.HasPhoto,
			HasKtp: row.HasKTP, HasKk: row.HasKK, HasMahramProof: row.HasMahramProof,
			MahramProofRequired: row.MahramProofRequired,
			DocumentsComplete:   len(missing) == 0,
			MissingDocuments:    missing,
		}
		if row.DateOfBirth != nil {
			message.DateOfBirth = timestamppb.New(*row.DateOfBirth)
		}
		if len(missing) == 0 {
			response.ReadyCount++
		}
		for _, document := range missing {
			response.MissingByDocument[document]++
		}
		response.Rows = append(response.Rows, message)
	}
	return response, nil
}
