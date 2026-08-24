package service

import (
	"context"
	"errors"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OperatorService struct {
	repository       *repository.OperatorRepository
	seasonRepository *repository.SeasonRepository
}

func NewOperatorService(repository *repository.OperatorRepository, seasonRepository *repository.SeasonRepository) *OperatorService {
	return &OperatorService{repository: repository, seasonRepository: seasonRepository}
}

func (s *OperatorService) Create(ctx context.Context, authenticatedOrgID string, request *hajjv1.CreateOperatorRequest) (*hajjv1.Operator, error) {
	if request == nil {
		return nil, serviceError("OperatorService.Create", apperror.ErrValidation)
	}
	if authenticatedOrgID == "" {
		return nil, serviceError("OperatorService.Create", apperror.ErrUnauthorized)
	}
	if authenticatedOrgID != request.BetterAuthOrgId {
		return nil, serviceError("OperatorService.Create", apperror.ErrForbidden)
	}
	if request.Country != "" && len(request.Country) != 2 {
		return nil, serviceError("OperatorService.Create", apperror.ErrValidation)
	}
	operator, err := s.repository.GetByBetterAuthOrgID(ctx, request.BetterAuthOrgId)
	if err == nil {
		return operatorMessage(operator), nil
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return nil, serviceError("OperatorService.Create", err)
	}
	if request.Slug != "" {
		if !repository.IsValidOperatorSlug(request.Slug) {
			return nil, serviceError("OperatorService.Create", apperror.ErrValidation)
		}
		available, availabilityErr := s.repository.IsSlugAvailable(ctx, request.Slug)
		if availabilityErr != nil {
			return nil, serviceError("OperatorService.Create", availabilityErr)
		}
		if !available {
			return nil, serviceError("OperatorService.Create", apperror.ErrAlreadyExists)
		}
	}
	operator, err = s.repository.Create(ctx, request.BetterAuthOrgId, request.Name, request.Country, request.Email, request.LicenseNumber, request.Slug)
	if err != nil {
		return nil, serviceError("OperatorService.Create", err)
	}
	return operatorMessage(operator), nil
}

func (s *OperatorService) CheckSlug(ctx context.Context, request *hajjv1.CheckOperatorSlugRequest) (*hajjv1.CheckOperatorSlugResponse, error) {
	if request == nil || !repository.IsValidOperatorSlug(request.Slug) {
		return nil, serviceError("OperatorService.CheckSlug", apperror.ErrValidation)
	}
	available, err := s.repository.IsSlugAvailable(ctx, request.Slug)
	if err != nil {
		return nil, serviceError("OperatorService.CheckSlug", err)
	}
	return &hajjv1.CheckOperatorSlugResponse{Available: available}, nil
}

func (s *OperatorService) Update(ctx context.Context, authenticatedOrgID string, request *hajjv1.UpdateOperatorRequest) (*hajjv1.Operator, error) {
	if request == nil {
		return nil, serviceError("OperatorService.Update", apperror.ErrValidation)
	}
	if authenticatedOrgID == "" {
		return nil, serviceError("OperatorService.Update", apperror.ErrUnauthorized)
	}
	if request.Country != "" && len(request.Country) != 2 {
		return nil, serviceError("OperatorService.Update", apperror.ErrValidation)
	}
	current, err := s.repository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.Update", err)
	}
	operator, err := s.repository.Update(ctx, current.ID, request.Name, request.Country, request.Email, request.LicenseNumber)
	if err != nil {
		return nil, serviceError("OperatorService.Update", err)
	}
	return operatorMessage(operator), nil
}

func (s *OperatorService) GetMy(ctx context.Context, authenticatedOrgID string) (*hajjv1.Operator, error) {
	if authenticatedOrgID == "" {
		return nil, serviceError("OperatorService.GetMy", apperror.ErrUnauthorized)
	}
	operator, err := s.repository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.GetMy", err)
	}
	return operatorMessage(operator), nil
}

func (s *OperatorService) ListAuditLogs(ctx context.Context, authenticatedOrgID string, limit int32) ([]*hajjv1.AuditLog, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	operator, err := s.repository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.ListAuditLogs", err)
	}
	rows, err := s.repository.ListAuditLogs(ctx, operator.ID, limit)
	if err != nil {
		return nil, serviceError("OperatorService.ListAuditLogs", err)
	}
	result := make([]*hajjv1.AuditLog, 0, len(rows))
	for _, row := range rows {
		result = append(result, &hajjv1.AuditLog{Id: row.ID, Action: row.Action, EntityType: row.EntityType, EntityId: row.EntityID, Description: row.Description, CreatedAt: timestamppb.New(row.CreatedAt), ActorName: row.ActorName})
	}
	return result, nil
}

// ResolveSlug is public (see publicProcedures in internal/middleware/auth.go)
// — apps/web/middleware.ts calls it to turn a subdomain like
// vacana.tawafiqhub.id into the operator ID the existing /register, /apply,
// /waitlist path-based routes already expect. Deliberately returns only
// id + name, nothing an anonymous caller shouldn't see.
func (s *OperatorService) ResolveSlug(ctx context.Context, request *hajjv1.ResolveOperatorSlugRequest) (*hajjv1.ResolveOperatorSlugResponse, error) {
	if request == nil || !repository.IsUsableOperatorSlug(request.Slug) {
		return nil, serviceError("OperatorService.ResolveSlug", apperror.ErrValidation)
	}
	operator, err := s.repository.GetBySlug(ctx, request.Slug)
	if err != nil {
		return nil, serviceError("OperatorService.ResolveSlug", err)
	}
	// No active season is a normal state (between seasons, or before the
	// first one is created) — not an error for this call, just an empty
	// field. apps/web/middleware.ts treats it as "no default season" for a
	// bare /register or /waitlist subdomain request.
	activeSeasonID, err := s.seasonRepository.GetActiveSeasonID(ctx, operator.ID)
	if err != nil && !errors.Is(err, apperror.ErrNotFound) {
		return nil, serviceError("OperatorService.ResolveSlug", err)
	}
	return &hajjv1.ResolveOperatorSlugResponse{OperatorId: operator.ID, Name: operator.Name, ActiveSeasonId: activeSeasonID}, nil
}

// UpdateMyProfile saves the public-profile fields for the operator behind the
// caller's Better Auth org (same authenticatedOrgID -> GetByBetterAuthOrgID
// resolution as Update/GetMy). Flips is_profile_complete TRUE.
func (s *OperatorService) UpdateMyProfile(ctx context.Context, authenticatedOrgID string, request *hajjv1.UpdateMyProfileRequest) (*hajjv1.Operator, error) {
	if request == nil {
		return nil, serviceError("OperatorService.UpdateMyProfile", apperror.ErrValidation)
	}
	if authenticatedOrgID == "" {
		return nil, serviceError("OperatorService.UpdateMyProfile", apperror.ErrUnauthorized)
	}
	current, err := s.repository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.UpdateMyProfile", err)
	}
	brandColor := request.BrandColor
	if brandColor == "" {
		brandColor = current.BrandColor
	}
	if brandColor == "" {
		brandColor = "#059669"
	}
	updated, err := s.repository.UpdateProfile(ctx, current.ID, domain.Operator{
		LogoURL:        request.LogoUrl,
		Description:    request.Description,
		WhatsappNumber: request.WhatsappNumber,
		Website:        request.Website,
		Address:        request.Address,
		City:           request.City,
		BrandColor:     brandColor,
		HeroEyebrow:    request.HeroEyebrow,
		HeroTitle:      request.HeroTitle,
		HeroSubtitle:   request.HeroSubtitle,
		HeroImageURL:   request.HeroImageUrl,
	})
	if err != nil {
		return nil, serviceError("OperatorService.UpdateMyProfile", err)
	}
	return operatorMessage(updated), nil
}

// GetPublicProfile is public (see publicProcedures in auth.go) — the operator's
// shareable {slug}.tawafiqhub.id landing page. Returns only non-sensitive
// fields plus available
// (not-yet-ended) seasons.
func (s *OperatorService) GetPublicProfile(ctx context.Context, request *hajjv1.GetPublicProfileRequest) (*hajjv1.GetPublicProfileResponse, error) {
	if request == nil || !repository.IsUsableOperatorSlug(request.Slug) {
		return nil, serviceError("OperatorService.GetPublicProfile", apperror.ErrValidation)
	}
	operator, err := s.repository.GetBySlug(ctx, request.Slug)
	if err != nil {
		return nil, serviceError("OperatorService.GetPublicProfile", err)
	}
	seasons, err := s.seasonRepository.ListPublicSeasons(ctx, operator.ID)
	if err != nil {
		return nil, serviceError("OperatorService.GetPublicProfile", err)
	}
	summaries := make([]*hajjv1.PublicSeasonSummary, 0, len(seasons))
	for _, season := range seasons {
		summaries = append(summaries, &hajjv1.PublicSeasonSummary{
			Id:           season.ID,
			Name:         season.Name,
			Type:         string(season.Type),
			StartDate:    timestamppb.New(season.StartDate),
			EndDate:      timestamppb.New(season.EndDate),
			PilgrimCount: season.PilgrimCount,
			Slug:         season.Slug,
		})
	}
	return &hajjv1.GetPublicProfileResponse{
		OperatorId:     operator.ID,
		Name:           operator.Name,
		Slug:           operator.Slug,
		LogoUrl:        operator.LogoURL,
		Description:    operator.Description,
		WhatsappNumber: operator.WhatsappNumber,
		Website:        operator.Website,
		Address:        operator.Address,
		City:           operator.City,
		LicenseNumber:  operator.LicenseNumber,
		Country:        operator.Country,
		ActiveSeasons:  summaries,
		BrandColor:     operator.BrandColor,
		HeroEyebrow:    operator.HeroEyebrow,
		HeroTitle:      operator.HeroTitle,
		HeroSubtitle:   operator.HeroSubtitle,
		HeroImageUrl:   operator.HeroImageURL,
	}, nil
}

func operatorMessage(value *domain.Operator) *hajjv1.Operator {
	return &hajjv1.Operator{
		Id:                value.ID,
		BetterAuthOrgId:   value.BetterAuthOrgID,
		Name:              value.Name,
		Country:           value.Country,
		Email:             value.Email,
		LicenseNumber:     value.LicenseNumber,
		Slug:              value.Slug,
		CreatedAt:         timestamppb.New(value.CreatedAt),
		LogoUrl:           value.LogoURL,
		Description:       value.Description,
		WhatsappNumber:    value.WhatsappNumber,
		Website:           value.Website,
		Address:           value.Address,
		City:              value.City,
		IsProfileComplete: value.IsProfileComplete,
		BrandColor:        value.BrandColor,
		HeroEyebrow:       value.HeroEyebrow,
		HeroTitle:         value.HeroTitle,
		HeroSubtitle:      value.HeroSubtitle,
		HeroImageUrl:      value.HeroImageURL,
	}
}
