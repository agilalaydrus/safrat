package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const operatorSlugBaseMaxLength = 55

var (
	operatorSlugSeparatorPattern = regexp.MustCompile(`[^a-z0-9]+`)
	operatorLegalPrefixes        = map[string]struct{}{
		"pt": {}, "cv": {}, "ud": {}, "pd": {}, "fa": {},
		"kbih": {}, "kbihu": {}, "yayasan": {},
	}
	operatorReservedSlugs = map[string]struct{}{
		"app": {}, "api": {}, "www": {},
	}
)

// slugBase derives a readable, DNS-label-safe candidate from the full operator
// name. A generic Indonesian legal-entity prefix is removed when another word
// follows it: "PT Vacana Indonesia" -> "vacana-indonesia", not the useless
// "pt". The base is bounded so uniqueness suffixes still fit in a 63-character
// DNS label.
func slugBase(name string) string {
	normalized := operatorSlugSeparatorPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	normalized = strings.Trim(normalized, "-")
	parts := strings.Split(normalized, "-")
	if len(parts) > 1 {
		if _, generic := operatorLegalPrefixes[parts[0]]; generic {
			parts = parts[1:]
		}
	}
	base := strings.Join(parts, "-")
	if len(base) > operatorSlugBaseMaxLength {
		base = strings.TrimRight(base[:operatorSlugBaseMaxLength], "-")
	}
	return base
}

// operatorCacheTTL is generous — operator rows (name/country/license) change
// essentially never after onboarding, and GetByBetterAuthOrgID is the
// hottest lookup in the whole API: it's the first thing ~50 service methods
// do on every authenticated RPC (resolve orgID -> operator before touching
// any real data). Same in-memory, per-process tradeoff as the MyAccess
// cache in identity.go — fine for the single-API-instance deployment; move
// to Redis if the API ever runs more than one replica.
const operatorCacheTTL = 5 * time.Minute

type operatorCacheEntry struct {
	value     *domain.Operator
	expiresAt time.Time
}

type OperatorRepository struct {
	queries *db.Queries

	mu    sync.Mutex
	cache map[string]operatorCacheEntry // betterAuthOrgID -> cached Operator
}

func NewOperatorRepository(queries *db.Queries) *OperatorRepository {
	return &OperatorRepository{queries: queries, cache: make(map[string]operatorCacheEntry)}
}

func (r *OperatorRepository) Create(ctx context.Context, betterAuthOrgID, name, country, email, licenseNumber string) (*domain.Operator, error) {
	slug, err := r.uniqueSlug(ctx, name)
	if err != nil {
		return nil, err
	}
	operator, err := r.queries.CreateOperator(ctx, db.CreateOperatorParams{
		BetterAuthOrgID: betterAuthOrgID,
		Name:            name,
		Country:         country,
		Email:           email,
		Column5:         licenseNumber,
		Slug:            pgtype.Text{String: slug, Valid: slug != ""},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return r.GetByBetterAuthOrgID(ctx, betterAuthOrgID)
	}
	if err != nil {
		return nil, err
	}
	return toOperator(operator), nil
}

// uniqueSlug tries the readable base candidate, then -2, -3, ... until it
// finds one not already taken. Bounded at 50 attempts — if the name is generic
// enough to collide 50 times, something else is wrong; the
// operator still gets created (Create's slug ends up empty, not a hard
// failure), just without a subdomain until someone sets one manually.
func (r *OperatorRepository) uniqueSlug(ctx context.Context, name string) (string, error) {
	base := slugBase(name)
	if base == "" {
		return "", nil
	}
	for attempt := 1; attempt <= 50; attempt++ {
		candidate := base
		if attempt > 1 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		if _, reserved := operatorReservedSlugs[candidate]; reserved {
			continue
		}
		exists, err := r.queries.OperatorSlugExists(ctx, pgtype.Text{String: candidate, Valid: true})
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", nil
}

func (r *OperatorRepository) GetBySlug(ctx context.Context, slug string) (*domain.Operator, error) {
	operator, err := r.queries.GetOperatorBySlug(ctx, pgtype.Text{String: slug, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toOperator(operator), nil
}

func (r *OperatorRepository) Update(ctx context.Context, operatorID, name, country, email, licenseNumber string) (*domain.Operator, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	operator, err := r.queries.UpdateOperator(ctx, db.UpdateOperatorParams{
		ID:      id,
		Name:    name,
		Country: country,
		Email:   email,
		Column5: licenseNumber,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := toOperator(operator)
	r.mu.Lock()
	r.cache[result.BetterAuthOrgID] = operatorCacheEntry{value: result, expiresAt: time.Now().Add(operatorCacheTTL)}
	r.mu.Unlock()
	return result, nil
}

func (r *OperatorRepository) UpdateProfile(ctx context.Context, operatorID string, profile domain.Operator) (*domain.Operator, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	operator, err := r.queries.UpdateOperatorProfile(ctx, db.UpdateOperatorProfileParams{
		ID:             id,
		LogoUrl:        pgtype.Text{String: profile.LogoURL, Valid: profile.LogoURL != ""},
		Description:    profile.Description,
		WhatsappNumber: profile.WhatsappNumber,
		Website:        profile.Website,
		Address:        profile.Address,
		City:           profile.City,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := toOperator(operator)
	// Same cache invalidation as Update — GetByBetterAuthOrgID is cached, and
	// a stale entry would hide the just-saved profile from every subsequent
	// authenticated RPC for up to operatorCacheTTL.
	r.mu.Lock()
	r.cache[result.BetterAuthOrgID] = operatorCacheEntry{value: result, expiresAt: time.Now().Add(operatorCacheTTL)}
	r.mu.Unlock()
	return result, nil
}

func (r *OperatorRepository) GetByBetterAuthOrgID(ctx context.Context, betterAuthOrgID string) (*domain.Operator, error) {
	if cached, ok := r.cachedOperator(betterAuthOrgID); ok {
		return cached, nil
	}
	operator, err := r.queries.GetOperatorByBetterAuthOrgID(ctx, betterAuthOrgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := toOperator(operator)
	r.mu.Lock()
	r.cache[betterAuthOrgID] = operatorCacheEntry{value: result, expiresAt: time.Now().Add(operatorCacheTTL)}
	r.mu.Unlock()
	return result, nil
}

func (r *OperatorRepository) cachedOperator(betterAuthOrgID string) (*domain.Operator, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.cache[betterAuthOrgID]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

func (r *OperatorRepository) GetByID(ctx context.Context, operatorID string) (*domain.Operator, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	operator, err := r.queries.GetOperatorByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toOperator(operator), nil
}

func (r *OperatorRepository) ListIDs(ctx context.Context) ([]string, error) {
	rows, err := r.queries.ListOperatorIDs(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, uuid.UUID(row.Bytes).String())
	}
	return ids, nil
}

func (r *OperatorRepository) ListAuditLogs(ctx context.Context, operatorID string, limit int32) ([]*domain.AuditLog, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListAuditLogs(ctx, db.ListAuditLogsParams{OperatorID: id, Limit: limit})
	if err != nil {
		return nil, err
	}
	logs := make([]*domain.AuditLog, 0, len(rows))
	for _, row := range rows {
		logs = append(logs, &domain.AuditLog{ID: uuid.UUID(row.ID.Bytes).String(), Action: row.Action, EntityType: row.EntityType, EntityID: uuid.UUID(row.EntityID.Bytes).String(), Description: row.Description, CreatedAt: row.CreatedAt.Time, ActorName: row.ActorName})
	}
	return logs, nil
}

func toOperator(value db.Operator) *domain.Operator {
	return &domain.Operator{
		ID:                uuid.UUID(value.ID.Bytes).String(),
		BetterAuthOrgID:   value.BetterAuthOrgID,
		Name:              value.Name,
		Country:           value.Country,
		Email:             value.Email,
		LicenseNumber:     value.LicenseNumber.String,
		Slug:              value.Slug.String,
		CreatedAt:         value.CreatedAt.Time,
		LogoURL:           value.LogoUrl.String,
		Description:       value.Description,
		WhatsappNumber:    value.WhatsappNumber,
		Website:           value.Website,
		Address:           value.Address,
		City:              value.City,
		IsProfileComplete: value.IsProfileComplete,
	}
}
