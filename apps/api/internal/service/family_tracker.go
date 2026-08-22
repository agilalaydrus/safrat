package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FamilyTrackerService struct {
	repository        *repository.FamilyTrackerRepository
	journeyRepository *repository.JourneyRepository
	ritualRepository  *repository.RitualRepository
}

func NewFamilyTrackerService(repository *repository.FamilyTrackerRepository, journeys *repository.JourneyRepository, rituals *repository.RitualRepository) *FamilyTrackerService {
	return &FamilyTrackerService{repository: repository, journeyRepository: journeys, ritualRepository: rituals}
}

// journeyStatusFamilyLabel is deliberately coarser than the raw enum — a
// family member sees "Sedang di Makkah", never "IN_MAKKAH".
var journeyStatusFamilyLabel = map[string]string{
	"REGISTERED": "Terdaftar", "DOCUMENT_VERIFIED": "Dokumen terverifikasi", "PRE_DEPARTURE": "Persiapan keberangkatan",
	"DEPARTED_INDONESIA": "Dalam perjalanan menuju Saudi", "IN_TRANSIT": "Transit", "ARRIVED_SAUDI": "Tiba di Arab Saudi",
	"IN_MADINAH": "Sedang di Madinah", "IN_MAKKAH": "Sedang di Makkah", "IN_ARAFAH": "Sedang di Arafah",
	"IN_MUZDALIFAH": "Sedang di Muzdalifah", "IN_MINA": "Sedang di Mina", "BACK_IN_MAKKAH": "Sedang di Makkah",
	"PRE_DEPARTURE_SAUDI": "Persiapan kepulangan", "DEPARTED_SAUDI": "Dalam perjalanan pulang ke Indonesia",
	"IN_TRANSIT_RETURN": "Transit kepulangan", "ARRIVED_INDONESIA": "Telah tiba di Indonesia ✓", "COMPLETED": "Perjalanan selesai",
}

// GetFamilyStatus is public — app_access_code is the only identity token,
// no operator/pilgrim session at all. Never distinguishes "code doesn't
// exist" from any other lookup failure in its response — a family member
// or an attacker both just see "not found".
func (s *FamilyTrackerService) GetFamilyStatus(ctx context.Context, req *hajjv1.GetFamilyStatusRequest) (*hajjv1.FamilyStatus, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" || len(req.AppAccessCode) > 64 {
		return nil, serviceError("FamilyTrackerService.GetFamilyStatus", apperror.ErrValidation)
	}
	status, pilgrimID, operatorID, err := s.repository.Get(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("FamilyTrackerService.GetFamilyStatus", apperror.ErrNotFound)
	}
	msg := familyStatusMessage(status)
	if js, err := s.journeyRepository.GetStatus(ctx, operatorID, pilgrimID); err == nil && js != nil {
		msg.JourneyStatusLabel = journeyStatusFamilyLabel[js.Status]
	} else {
		msg.JourneyStatusLabel = journeyStatusFamilyLabel["REGISTERED"]
	}
	if rituals, err := s.ritualRepository.GetPilgrimStatusByAccessCode(ctx, req.AppAccessCode); err == nil {
		msg.RitualsTotal = int32(len(rituals))
		for _, r := range rituals {
			if r.Completed {
				msg.RitualsCompleted++
			}
		}
	}
	return msg, nil
}

func familyStatusMessage(value *domain.FamilyStatus) *hajjv1.FamilyStatus {
	result := &hajjv1.FamilyStatus{
		FirstName: value.FirstName, PaymentStatus: value.PaymentStatus, HotelCheckedIn: value.HotelCheckedIn,
		PilgrimStatus: value.PilgrimStatus, SeasonName: value.SeasonName, DepartureDate: timestamppb.New(value.DepartureDate),
		GroupName: value.GroupName, LeaderName: value.LeaderName, HasActiveSos: value.HasActiveSOS,
	}
	if value.LastLocationAt != nil {
		result.LastLocationAt = timestamppb.New(*value.LastLocationAt)
	}
	return result
}
