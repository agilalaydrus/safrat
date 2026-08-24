package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/storage"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OperatorService struct {
	repository           *repository.OperatorRepository
	seasonRepository     *repository.SeasonRepository
	storefrontRepository *repository.StorefrontRepository
	objectStorage        *storage.Store
}

func NewOperatorService(repository *repository.OperatorRepository, seasonRepository *repository.SeasonRepository, storefrontRepository *repository.StorefrontRepository, objectStorage *storage.Store) *OperatorService {
	return &OperatorService{repository: repository, seasonRepository: seasonRepository, storefrontRepository: storefrontRepository, objectStorage: objectStorage}
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
	content := legacyStorefrontContent(operator)
	if published, _, publishedErr := s.storefrontRepository.GetPublished(ctx, operator.ID); publishedErr == nil {
		content = &hajjv1.StorefrontContent{}
		if err := protojson.Unmarshal(published, content); err != nil {
			return nil, serviceError("OperatorService.GetPublicProfile", fmt.Errorf("decode published storefront: %w", err))
		}
	} else if !errors.Is(publishedErr, apperror.ErrNotFound) {
		return nil, serviceError("OperatorService.GetPublicProfile", publishedErr)
	}
	applyStorefrontDefaults(content, operator)
	summaries := publicSeasonMessages(seasons)
	return &hajjv1.GetPublicProfileResponse{
		OperatorId:     operator.ID,
		Name:           content.DisplayName,
		Slug:           operator.Slug,
		LogoUrl:        content.LogoUrl,
		Description:    content.Description,
		WhatsappNumber: content.WhatsappNumber,
		Website:        content.Website,
		Address:        content.Address,
		City:           content.City,
		LicenseNumber:  operator.LicenseNumber,
		Country:        operator.Country,
		ActiveSeasons:  summaries,
		BrandColor:     content.BrandColor,
		HeroEyebrow:    content.HeroEyebrow,
		HeroTitle:      content.HeroTitle,
		HeroSubtitle:   content.HeroSubtitle,
		HeroImageUrl:   content.HeroImageUrl,
		Content:        content,
	}, nil
}

func (s *OperatorService) GetMyStorefront(ctx context.Context, authenticatedOrgID string) (*hajjv1.StorefrontEditor, error) {
	operator, err := s.authenticatedOperator(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.GetMyStorefront", err)
	}
	return s.storefrontEditor(ctx, operator, nil)
}

func (s *OperatorService) SaveMyStorefrontDraft(ctx context.Context, authenticatedOrgID string, request *hajjv1.SaveMyStorefrontDraftRequest) (*hajjv1.StorefrontEditor, error) {
	if request == nil || request.Content == nil {
		return nil, serviceError("OperatorService.SaveMyStorefrontDraft", apperror.ErrValidation)
	}
	operator, err := s.authenticatedOperator(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.SaveMyStorefrontDraft", err)
	}
	content := request.Content
	applyStorefrontDefaults(content, operator)
	if err := s.validateStorefront(ctx, operator.ID, content); err != nil {
		return nil, serviceError("OperatorService.SaveMyStorefrontDraft", err)
	}
	payload, err := protojson.MarshalOptions{UseProtoNames: false}.Marshal(content)
	if err != nil {
		return nil, serviceError("OperatorService.SaveMyStorefrontDraft", err)
	}
	snapshot, err := s.storefrontRepository.SaveDraft(ctx, operator.ID, payload, request.ExpectedRevision)
	if err != nil {
		return nil, serviceError("OperatorService.SaveMyStorefrontDraft", err)
	}
	return s.storefrontEditor(ctx, operator, snapshot)
}

func (s *OperatorService) PublishMyStorefront(ctx context.Context, authenticatedOrgID string, request *hajjv1.PublishMyStorefrontRequest) (*hajjv1.StorefrontEditor, error) {
	if request == nil || request.ExpectedRevision <= 0 {
		return nil, serviceError("OperatorService.PublishMyStorefront", apperror.ErrValidation)
	}
	operator, err := s.authenticatedOperator(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.PublishMyStorefront", err)
	}
	snapshot, err := s.storefrontRepository.Get(ctx, operator.ID)
	if err != nil {
		return nil, serviceError("OperatorService.PublishMyStorefront", err)
	}
	content := &hajjv1.StorefrontContent{}
	if err := protojson.Unmarshal(snapshot.Draft, content); err != nil {
		return nil, serviceError("OperatorService.PublishMyStorefront", err)
	}
	if err := s.validateStorefront(ctx, operator.ID, content); err != nil {
		return nil, serviceError("OperatorService.PublishMyStorefront", err)
	}
	snapshot, err = s.storefrontRepository.Publish(ctx, operator.ID, request.ExpectedRevision)
	if err != nil {
		return nil, serviceError("OperatorService.PublishMyStorefront", err)
	}
	return s.storefrontEditor(ctx, operator, snapshot)
}

func (s *OperatorService) CreateStorefrontUpload(ctx context.Context, authenticatedOrgID string, request *hajjv1.CreateStorefrontUploadRequest) (*hajjv1.CreateStorefrontUploadResponse, error) {
	if request == nil || request.SizeBytes <= 0 || request.SizeBytes > storage.MaxStorefrontBytes {
		return nil, serviceError("OperatorService.CreateStorefrontUpload", apperror.ErrValidation)
	}
	operator, err := s.authenticatedOperator(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.CreateStorefrontUpload", err)
	}
	kinds := map[hajjv1.StorefrontAssetKind]string{
		hajjv1.StorefrontAssetKind_STOREFRONT_ASSET_KIND_LOGO:    "logo",
		hajjv1.StorefrontAssetKind_STOREFRONT_ASSET_KIND_HERO:    "hero",
		hajjv1.StorefrontAssetKind_STOREFRONT_ASSET_KIND_GALLERY: "gallery",
		hajjv1.StorefrontAssetKind_STOREFRONT_ASSET_KIND_PACKAGE: "package",
	}
	kind, ok := kinds[request.Kind]
	if !ok {
		return nil, serviceError("OperatorService.CreateStorefrontUpload", apperror.ErrValidation)
	}
	upload, err := s.objectStorage.PresignStorefrontUpload(ctx, operator.ID, kind, request.SizeBytes)
	if errors.Is(err, storage.ErrNotConfigured) {
		return nil, serviceError("OperatorService.CreateStorefrontUpload", fmt.Errorf("%w: object storage belum dikonfigurasi", apperror.ErrFailedPrecondition))
	}
	if err != nil {
		return nil, serviceError("OperatorService.CreateStorefrontUpload", err)
	}
	return &hajjv1.CreateStorefrontUploadResponse{
		UploadUrl: upload.UploadURL, Method: "PUT",
		ContentType: storage.StorefrontContentType, ExpiresAt: timestamppb.New(upload.ExpiresAt), ObjectKey: upload.ObjectKey,
	}, nil
}

func (s *OperatorService) ConfirmStorefrontUpload(ctx context.Context, authenticatedOrgID string, request *hajjv1.ConfirmStorefrontUploadRequest) (*hajjv1.ConfirmStorefrontUploadResponse, error) {
	if request == nil || strings.TrimSpace(request.ObjectKey) == "" {
		return nil, serviceError("OperatorService.ConfirmStorefrontUpload", apperror.ErrValidation)
	}
	operator, err := s.authenticatedOperator(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("OperatorService.ConfirmStorefrontUpload", err)
	}
	publicURL, err := s.objectStorage.ConfirmStorefrontUpload(ctx, operator.ID, request.ObjectKey)
	if errors.Is(err, storage.ErrNotConfigured) {
		return nil, serviceError("OperatorService.ConfirmStorefrontUpload", apperror.ErrFailedPrecondition)
	}
	if err != nil {
		return nil, serviceError("OperatorService.ConfirmStorefrontUpload", fmt.Errorf("%w: %v", apperror.ErrValidation, err))
	}
	return &hajjv1.ConfirmStorefrontUploadResponse{PublicUrl: publicURL}, nil
}

func (s *OperatorService) authenticatedOperator(ctx context.Context, authenticatedOrgID string) (*domain.Operator, error) {
	if authenticatedOrgID == "" {
		return nil, apperror.ErrUnauthorized
	}
	return s.repository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
}

func (s *OperatorService) storefrontEditor(ctx context.Context, operator *domain.Operator, snapshot *domain.StorefrontSnapshot) (*hajjv1.StorefrontEditor, error) {
	if snapshot == nil {
		var err error
		snapshot, err = s.storefrontRepository.Get(ctx, operator.ID)
		if err != nil && !errors.Is(err, apperror.ErrNotFound) {
			return nil, err
		}
	}
	content := legacyStorefrontContent(operator)
	var draftRevision, publishedRevision int64
	var publishedAt *timestamppb.Timestamp
	if snapshot != nil {
		content = &hajjv1.StorefrontContent{}
		if err := protojson.Unmarshal(snapshot.Draft, content); err != nil {
			return nil, fmt.Errorf("decode storefront draft: %w", err)
		}
		draftRevision = snapshot.DraftRevision
		publishedRevision = snapshot.PublishedRevision
		if snapshot.PublishedAt != nil {
			publishedAt = timestamppb.New(*snapshot.PublishedAt)
		}
	}
	applyStorefrontDefaults(content, operator)
	seasons, err := s.seasonRepository.ListPublicSeasons(ctx, operator.ID)
	if err != nil {
		return nil, err
	}
	return &hajjv1.StorefrontEditor{
		Content: content, ActiveSeasons: publicSeasonMessages(seasons),
		DraftRevision: draftRevision, PublishedRevision: publishedRevision, PublishedAt: publishedAt,
		OperatorName: operator.Name, OperatorSlug: operator.Slug, LicenseNumber: operator.LicenseNumber, Country: operator.Country,
	}, nil
}

func (s *OperatorService) validateStorefront(ctx context.Context, operatorID string, content *hajjv1.StorefrontContent) error {
	if content.DisplayName == "" || !validHTTPURLOrEmpty(content.LogoUrl) || !validHTTPURLOrEmpty(content.Website) || !validHTTPURLOrEmpty(content.HeroImageUrl) {
		return apperror.ErrValidation
	}
	if len(content.Packages) > 20 || len(content.Gallery) > 12 || len(content.Testimonials) > 6 || len(content.Faqs) > 10 {
		return apperror.ErrValidation
	}
	seasons, err := s.seasonRepository.ListPublicSeasons(ctx, operatorID)
	if err != nil {
		return err
	}
	seasonIDs := make(map[string]struct{}, len(seasons))
	for _, season := range seasons {
		seasonIDs[season.ID] = struct{}{}
	}
	seenPackages := make(map[string]struct{}, len(content.Packages))
	for _, item := range content.Packages {
		if item == nil || !validHTTPURLOrEmpty(item.ImageUrl) || len(item.Facilities) > 12 || len(item.Itinerary) > 20 {
			return apperror.ErrValidation
		}
		if _, ok := seasonIDs[item.SeasonId]; !ok {
			return apperror.ErrValidation
		}
		if _, duplicate := seenPackages[item.SeasonId]; duplicate {
			return apperror.ErrValidation
		}
		seenPackages[item.SeasonId] = struct{}{}
		for _, facility := range item.Facilities {
			if strings.TrimSpace(facility) == "" || len(facility) > 120 {
				return apperror.ErrValidation
			}
		}
		for _, itinerary := range item.Itinerary {
			if itinerary == nil || strings.TrimSpace(itinerary.Title) == "" {
				return apperror.ErrValidation
			}
		}
	}
	for _, item := range content.Gallery {
		if item == nil || !validHTTPURL(item.ImageUrl) || strings.TrimSpace(item.AltText) == "" {
			return apperror.ErrValidation
		}
	}
	for _, item := range content.Testimonials {
		if item == nil || strings.TrimSpace(item.Quote) == "" || strings.TrimSpace(item.Name) == "" {
			return apperror.ErrValidation
		}
	}
	for _, item := range content.Faqs {
		if item == nil || strings.TrimSpace(item.Question) == "" || strings.TrimSpace(item.Answer) == "" {
			return apperror.ErrValidation
		}
	}
	return nil
}

func validHTTPURLOrEmpty(value string) bool {
	return strings.TrimSpace(value) == "" || validHTTPURL(value)
}

func validHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func legacyStorefrontContent(operator *domain.Operator) *hajjv1.StorefrontContent {
	return &hajjv1.StorefrontContent{
		DisplayName: operator.Name, LogoUrl: operator.LogoURL, Description: operator.Description,
		WhatsappNumber: operator.WhatsappNumber, Website: operator.Website, Address: operator.Address,
		City: operator.City, BrandColor: operator.BrandColor, HeroEyebrow: operator.HeroEyebrow,
		HeroTitle: operator.HeroTitle, HeroSubtitle: operator.HeroSubtitle, HeroImageUrl: operator.HeroImageURL,
	}
}

func applyStorefrontDefaults(content *hajjv1.StorefrontContent, operator *domain.Operator) {
	content.DisplayName = strings.TrimSpace(content.DisplayName)
	if content.DisplayName == "" {
		content.DisplayName = operator.Name
	}
	if content.BrandColor == "" {
		content.BrandColor = "#059669"
	}
}

func publicSeasonMessages(seasons []*domain.PublicSeasonSummary) []*hajjv1.PublicSeasonSummary {
	result := make([]*hajjv1.PublicSeasonSummary, 0, len(seasons))
	for _, season := range seasons {
		result = append(result, &hajjv1.PublicSeasonSummary{
			Id: season.ID, Name: season.Name, Type: string(season.Type), StartDate: timestamppb.New(season.StartDate),
			EndDate: timestamppb.New(season.EndDate), PilgrimCount: season.PilgrimCount, Slug: season.Slug,
		})
	}
	return result
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
