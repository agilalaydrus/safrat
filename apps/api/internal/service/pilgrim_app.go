package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PilgrimAppService struct {
	pilgrimRepository      *repository.PilgrimRepository
	productRepository      *repository.ProductRepository
	auditRepository        *repository.AuditRepository
	identityRepository     *repository.IdentityRepository
	broadcastRepository    *repository.BroadcastRepository
	journeyRepository      *repository.JourneyRepository
	ritualRepository       *repository.RitualRepository
	notificationRepository *repository.NotificationRepository
}

func NewPilgrimAppService(pilgrims *repository.PilgrimRepository, products *repository.ProductRepository, audit *repository.AuditRepository, identity *repository.IdentityRepository, broadcasts *repository.BroadcastRepository, journeys *repository.JourneyRepository, rituals *repository.RitualRepository, notifications *repository.NotificationRepository) *PilgrimAppService {
	return &PilgrimAppService{pilgrimRepository: pilgrims, productRepository: products, auditRepository: audit, identityRepository: identity, broadcastRepository: broadcasts, journeyRepository: journeys, ritualRepository: rituals, notificationRepository: notifications}
}

// certificateUnlockStatuses mirrors JourneyStatuses' tail — a certificate
// only unlocks once the pilgrim has actually landed back in Indonesia.
var certificateUnlockStatuses = map[string]bool{"ARRIVED_INDONESIA": true, "COMPLETED": true}

// GetMyCertificate is public (app_access_code), same pattern as every
// other PilgrimAppService method. Gated on journey status per the
// business rule "ARRIVED_INDONESIA otomatis unlock sertifikat digital" —
// returns Unlocked:false with every other field zero-valued until then.
func (s *PilgrimAppService) GetMyCertificate(ctx context.Context, req *hajjv1.GetMyCertificateRequest) (*hajjv1.CertificateData, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.GetMyCertificate", apperror.ErrValidation)
	}
	pilgrim, err := s.pilgrimRepository.GetByAppAccessCode(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.GetMyCertificate", apperror.ErrNotFound)
	}
	status := "REGISTERED"
	if js, err := s.journeyRepository.GetStatus(ctx, pilgrim.OperatorID, pilgrim.ID); err == nil && js != nil {
		status = js.Status
	}
	if !certificateUnlockStatuses[status] {
		return &hajjv1.CertificateData{Unlocked: false}, nil
	}
	data, err := s.pilgrimRepository.GetCertificateData(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.GetMyCertificate", apperror.ErrNotFound)
	}
	return &hajjv1.CertificateData{
		PilgrimName: data.PilgrimName, PassportNumber: data.PassportNumber, Nationality: data.Nationality,
		SeasonName: data.SeasonName, SeasonType: string(data.SeasonType),
		StartDate: timestamppb.New(data.StartDate), EndDate: timestamppb.New(data.EndDate),
		OperatorName: data.OperatorName, LicenseNumber: data.LicenseNumber,
		GroupName: data.GroupName, LeaderName: data.LeaderName,
		HotelsVisited: data.HotelsVisited, MakkahHotels: data.MakkahHotels, MadinahHotels: data.MadinahHotels,
		Unlocked: true,
	}, nil
}

// ListMyBroadcasts is public (app_access_code), same pattern as every other
// PilgrimAppService method — resolves operator+season from the code, then
// lists that season's broadcasts (same repository BroadcastService uses).
func (s *PilgrimAppService) ListMyBroadcasts(ctx context.Context, req *hajjv1.ListMyBroadcastsRequest) (*hajjv1.ListBroadcastsResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.ListMyBroadcasts", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMyBroadcasts", apperror.ErrNotFound)
	}
	broadcasts, err := s.broadcastRepository.List(ctx, info.OperatorID, info.SeasonID)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMyBroadcasts", err)
	}
	result := &hajjv1.ListBroadcastsResponse{Broadcasts: make([]*hajjv1.Broadcast, 0, len(broadcasts))}
	for _, b := range broadcasts {
		result.Broadcasts = append(result.Broadcasts, broadcastMessage(b))
	}
	return result, nil
}

func (s *PilgrimAppService) GetMyInfo(ctx context.Context, req *hajjv1.GetMyInfoRequest) (*hajjv1.PilgrimAppInfo, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.GetMyInfo", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.GetMyInfo", apperror.ErrNotFound)
	}
	result := pilgrimAppInfoMessage(info)
	movements, err := s.pilgrimRepository.ListUpcomingMovements(ctx, info.OperatorID, info.SeasonID, info.KloterID)
	if err == nil && len(movements) > 0 {
		m := movements[0]
		result.NextMovement = &hajjv1.Movement{Id: m.ID, OperatorId: m.OperatorID, SeasonId: m.SeasonID, Name: m.Name, Origin: m.Origin, Destination: m.Destination, ScheduledAt: timestamppb.New(m.ScheduledAt), Status: m.Status, Mode: m.Mode, KloterId: m.KloterID, CreatedAt: timestamppb.New(m.CreatedAt), Airline: m.Airline, FlightNumber: m.FlightNumber, TripLeg: m.TripLeg}
	}
	if stays, err := s.pilgrimRepository.ListHotelStays(ctx, req.AppAccessCode); err == nil {
		for _, stay := range stays {
			result.HotelStays = append(result.HotelStays, &hajjv1.HotelStay{HotelName: stay.HotelName, RoomNumber: stay.RoomNumber, RoomType: stay.RoomType})
		}
	}
	if js, err := s.journeyRepository.GetStatus(ctx, info.OperatorID, info.ID); err == nil && js != nil {
		result.JourneyStatus = js.Status
	} else {
		result.JourneyStatus = "REGISTERED"
	}
	return result, nil
}

// ListMyRituals is public (app_access_code) — powers the "Ibadah Saya" tab.
func (s *PilgrimAppService) ListMyRituals(ctx context.Context, req *hajjv1.ListMyRitualsRequest) (*hajjv1.ListMyRitualsResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.ListMyRituals", apperror.ErrValidation)
	}
	statuses, err := s.ritualRepository.GetPilgrimStatusByAccessCode(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMyRituals", err)
	}
	result := &hajjv1.ListMyRitualsResponse{Rituals: make([]*hajjv1.PilgrimRitualStatus, 0, len(statuses))}
	for _, st := range statuses {
		result.Rituals = append(result.Rituals, pilgrimRitualStatusMessage(st))
	}
	return result, nil
}

// RegisterMyPushToken is public (app_access_code) — see pilgrim_push_tokens
// in migration 071 for why this is a separate table from push_subscriptions.
func (s *PilgrimAppService) RegisterMyPushToken(ctx context.Context, req *hajjv1.RegisterMyPushTokenRequest) (*hajjv1.RegisterMyPushTokenResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" || strings.TrimSpace(req.FcmToken) == "" {
		return nil, serviceError("PilgrimAppService.RegisterMyPushToken", apperror.ErrValidation)
	}
	pilgrim, err := s.pilgrimRepository.GetByAppAccessCode(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.RegisterMyPushToken", apperror.ErrNotFound)
	}
	if err := s.notificationRepository.RegisterPilgrimToken(ctx, pilgrim.OperatorID, pilgrim.ID, req.FcmToken); err != nil {
		return nil, serviceError("PilgrimAppService.RegisterMyPushToken", err)
	}
	return &hajjv1.RegisterMyPushTokenResponse{}, nil
}

func pilgrimAppInfoMessage(info *domain.PilgrimAppInfo) *hajjv1.PilgrimAppInfo {
	result := &hajjv1.PilgrimAppInfo{
		Id: info.ID, FullName: info.FullName, PassportNumber: info.PassportNumber, GroupName: info.GroupName, HotelName: info.HotelName,
		RoomNumber: info.RoomNumber, RequiresWheelchair: info.RequiresWheelchair, KloterCode: info.KloterCode, KloterEmbarkation: info.KloterEmbarkation,
		KloterFlightNumber: info.KloterFlightNumber, LinkedGoogleEmail: info.LinkedGoogleEmail, Phone: info.Phone,
		Nik: info.NIK, Address: info.Address, KycStatus: info.KYCStatus, KycRejectionReason: info.KYCRejectionReason,
		PlaceOfBirth: info.PlaceOfBirth, MaritalStatus: info.MaritalStatus, Occupation: info.Occupation, FatherName: info.FatherName,
		Status: info.Status, PaymentStatus: info.PaymentStatus,
	}
	if info.KloterDepartureDate != nil {
		result.KloterDepartureDate = timestamppb.New(*info.KloterDepartureDate)
	}
	return result
}

// SubmitMyKyc is public (app_access_code), same pattern as every other
// PilgrimAppService method — always lands in PENDING_REVIEW, kyc_source
// SELF; an admin still has to verify (PilgrimService.VerifyKyc). Never
// gates any other pilgrim feature.
func (s *PilgrimAppService) SubmitMyKyc(ctx context.Context, req *hajjv1.SubmitMyPilgrimKycRequest) (*hajjv1.PilgrimAppInfo, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.SubmitMyKyc", apperror.ErrValidation)
	}
	pilgrim, err := s.pilgrimRepository.GetByAppAccessCode(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.SubmitMyKyc", apperror.ErrNotFound)
	}
	input := domain.PilgrimKYCInput{NIK: req.Nik, Address: req.Address, PlaceOfBirth: req.PlaceOfBirth, MaritalStatus: req.MaritalStatus, Occupation: req.Occupation, FatherName: req.FatherName}
	if _, err := s.pilgrimRepository.UpdateKYC(ctx, pilgrim.OperatorID, pilgrim.ID, input, "SELF"); err != nil {
		return nil, serviceError("PilgrimAppService.SubmitMyKyc", err)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.SubmitMyKyc", err)
	}
	return pilgrimAppInfoMessage(info), nil
}

func (s *PilgrimAppService) UpdateMyLocation(ctx context.Context, req *hajjv1.UpdateMyLocationRequest) (*hajjv1.UpdateMyLocationResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.UpdateMyLocation", apperror.ErrValidation)
	}
	if err := s.pilgrimRepository.UpdateLocation(ctx, req.AppAccessCode, req.Lat, req.Lng); err != nil {
		return nil, serviceError("PilgrimAppService.UpdateMyLocation", err)
	}
	return &hajjv1.UpdateMyLocationResponse{}, nil
}

func (s *PilgrimAppService) RequestWheelchair(ctx context.Context, req *hajjv1.RequestWheelchairRequest) (*hajjv1.RequestWheelchairResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.RequestWheelchair", apperror.ErrValidation)
	}
	pilgrimID, operatorID, fullName, err := s.pilgrimRepository.SetWheelchairRequest(ctx, req.AppAccessCode, req.RequiresWheelchair)
	if err != nil {
		return nil, serviceError("PilgrimAppService.RequestWheelchair", apperror.ErrNotFound)
	}
	action, message := "pilgrim_wheelchair_requested", fmt.Sprintf("%s meminta bantuan kursi roda", fullName)
	if !req.RequiresWheelchair {
		action, message = "pilgrim_wheelchair_cancelled", fmt.Sprintf("%s membatalkan permintaan kursi roda", fullName)
	}
	_ = s.auditRepository.Write(ctx, operatorID, "", action, "pilgrim", pilgrimID, message)
	return &hajjv1.RequestWheelchairResponse{RequiresWheelchair: req.RequiresWheelchair}, nil
}

// LinkGoogleAccount runs through the session-only (not org-scoped) auth
// lane — ctx carries a real, server-validated Better Auth user id, never a
// client-supplied one. userID/userEmail come from middleware, not req.
func (s *PilgrimAppService) LinkGoogleAccount(ctx context.Context, req *hajjv1.LinkGoogleAccountRequest) (*hajjv1.LinkGoogleAccountResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.LinkGoogleAccount", apperror.ErrValidation)
	}
	userID, userEmail := middleware.UserIDFromCtx(ctx), middleware.UserEmailFromCtx(ctx)
	if userID == "" {
		return nil, serviceError("PilgrimAppService.LinkGoogleAccount", apperror.ErrUnauthorized)
	}
	isStaff, err := s.pilgrimRepository.UserIsOrganizationMember(ctx, userID)
	if err != nil {
		return nil, serviceError("PilgrimAppService.LinkGoogleAccount", err)
	}
	if isStaff {
		return nil, serviceError("PilgrimAppService.LinkGoogleAccount", apperror.ErrForbidden)
	}
	if err := s.pilgrimRepository.LinkGoogleAccount(ctx, req.AppAccessCode, userID); err != nil {
		return nil, serviceError("PilgrimAppService.LinkGoogleAccount", err)
	}
	// This just changed what GetMyAccess would return for this user (a new
	// linked_pilgrim role appears) — drop the cached value instead of
	// leaving them looking role-less for up to accessCacheTTL.
	s.identityRepository.InvalidateAccessCache(userID)
	return &hajjv1.LinkGoogleAccountResponse{LinkedGoogleEmail: userEmail}, nil
}

func (s *PilgrimAppService) ListMySchedule(ctx context.Context, req *hajjv1.ListMyScheduleRequest) (*hajjv1.ListMyScheduleResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.ListMySchedule", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMySchedule", apperror.ErrNotFound)
	}
	movements, err := s.pilgrimRepository.ListUpcomingMovements(ctx, info.OperatorID, info.SeasonID, info.KloterID)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMySchedule", err)
	}
	result := &hajjv1.ListMyScheduleResponse{Movements: make([]*hajjv1.Movement, 0, len(movements))}
	for _, m := range movements {
		result.Movements = append(result.Movements, &hajjv1.Movement{Id: m.ID, OperatorId: m.OperatorID, SeasonId: m.SeasonID, Name: m.Name, Origin: m.Origin, Destination: m.Destination, ScheduledAt: timestamppb.New(m.ScheduledAt), Status: m.Status, Mode: m.Mode, KloterId: m.KloterID, CreatedAt: timestamppb.New(m.CreatedAt), Airline: m.Airline, FlightNumber: m.FlightNumber, TripLeg: m.TripLeg})
	}
	return result, nil
}

func (s *PilgrimAppService) ListMyProducts(ctx context.Context, req *hajjv1.ListMyProductsRequest) (*hajjv1.ListMyProductsResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.ListMyProducts", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMyProducts", apperror.ErrNotFound)
	}
	products, err := s.productRepository.ListBySeasonID(ctx, info.OperatorID, info.SeasonID)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMyProducts", err)
	}
	result := &hajjv1.ListMyProductsResponse{Products: make([]*hajjv1.Product, 0, len(products))}
	for _, p := range products {
		if !p.IsActive {
			continue
		}
		result.Products = append(result.Products, productMessage(p))
	}
	return result, nil
}
