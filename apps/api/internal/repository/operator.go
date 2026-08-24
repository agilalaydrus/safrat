package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

const operatorSlugBaseMaxLength = 55

var (
	operatorSlugSeparatorPattern = regexp.MustCompile(`[^a-z0-9]+`)
	operatorSlugPattern          = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	operatorLegalPrefixes        = map[string]struct{}{
		"pt": {}, "cv": {}, "ud": {}, "pd": {}, "fa": {},
		"kbih": {}, "kbihu": {}, "yayasan": {},
	}
	operatorReservedSlugs = map[string]struct{}{
		"admin": {}, "api": {}, "app": {}, "auth": {}, "dashboard": {},
		"docs": {}, "help": {}, "status": {}, "support": {}, "www": {},
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
// any real data). With Redis configured, the shared value keeps this 5-minute
// TTL while each replica's L1 is bounded to 30 seconds and invalidated by
// pub/sub on writes.
const operatorCacheTTL = 5 * time.Minute
const operatorRedisLocalCacheTTL = 30 * time.Second

const operatorCacheInvalidationChannel = "safrat:cache:operator:invalidate"

type operatorCacheEntry struct {
	value     *domain.Operator
	expiresAt time.Time
}

type OperatorRepository struct {
	queries *db.Queries
	rdb     *redis.Client
	logger  *slog.Logger

	mu    sync.Mutex
	cache map[string]operatorCacheEntry // betterAuthOrgID -> cached Operator
}

func NewOperatorRepository(queries *db.Queries) *OperatorRepository {
	return &OperatorRepository{queries: queries, cache: make(map[string]operatorCacheEntry)}
}

// NewRedisOperatorRepository adds a shared Redis cache plus pub/sub
// invalidation to the small per-process hot cache. PostgreSQL remains the
// source of truth and every Redis failure degrades to the normal DB path.
func NewRedisOperatorRepository(ctx context.Context, queries *db.Queries, rdb *redis.Client, logger *slog.Logger) *OperatorRepository {
	repository := &OperatorRepository{queries: queries, rdb: rdb, logger: logger, cache: make(map[string]operatorCacheEntry)}
	if rdb != nil {
		go repository.listenForInvalidations(ctx)
	}
	return repository
}

func (r *OperatorRepository) Create(ctx context.Context, betterAuthOrgID, name, country, email, licenseNumber, requestedSlug string) (*domain.Operator, error) {
	slug := requestedSlug
	if slug != "" {
		// Service-layer availability checks are a friendly preflight, not a
		// security boundary. Keep repository callers and future jobs from ever
		// persisting an invalid or platform-reserved hostname.
		if !IsValidOperatorSlug(slug) {
			return nil, apperror.ErrValidation
		}
		if IsReservedOperatorSlug(slug) {
			return nil, apperror.ErrAlreadyExists
		}
	} else {
		var err error
		slug, err = r.uniqueSlug(ctx, name)
		if err != nil {
			return nil, err
		}
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
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "operators_slug_key" {
			return nil, apperror.ErrAlreadyExists
		}
		return nil, err
	}
	result := toOperator(operator)
	r.cacheOperator(ctx, result, true)
	return result, nil
}

// IsSlugAvailable is only a friendly onboarding preflight. The database's
// unique index remains the final authority, so concurrent signups cannot claim
// the same subdomain even if both preflight checks initially return true.
func (r *OperatorRepository) IsSlugAvailable(ctx context.Context, slug string) (bool, error) {
	if !IsValidOperatorSlug(slug) {
		return false, nil
	}
	if IsReservedOperatorSlug(slug) {
		return false, nil
	}
	exists, err := r.queries.OperatorSlugExists(ctx, pgtype.Text{String: slug, Valid: true})
	return !exists, err
}

func IsValidOperatorSlug(slug string) bool {
	return len(slug) >= 3 && len(slug) <= 63 && operatorSlugPattern.MatchString(slug)
}

func IsReservedOperatorSlug(slug string) bool {
	_, reserved := operatorReservedSlugs[slug]
	return reserved
}

func IsUsableOperatorSlug(slug string) bool {
	return IsValidOperatorSlug(slug) && !IsReservedOperatorSlug(slug)
}

// uniqueSlug tries the readable base candidate, then -2, -3, ... until it
// finds one not already taken. Bounded at 50 attempts — if the name is generic
// enough to collide 50 times, something else is wrong; the
// operator still gets created (Create's slug ends up empty, not a hard
// failure), just without a subdomain until someone sets one manually.
func (r *OperatorRepository) uniqueSlug(ctx context.Context, name string) (string, error) {
	base := slugBase(name)
	if !IsValidOperatorSlug(base) {
		return "", nil
	}
	for attempt := 1; attempt <= 50; attempt++ {
		candidate := base
		if attempt > 1 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		if IsReservedOperatorSlug(candidate) {
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
	if !IsUsableOperatorSlug(slug) {
		return nil, apperror.ErrValidation
	}
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
	r.cacheOperator(ctx, result, true)
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
		BrandColor:     profile.BrandColor,
		HeroEyebrow:    profile.HeroEyebrow,
		HeroTitle:      profile.HeroTitle,
		HeroSubtitle:   profile.HeroSubtitle,
		HeroImageUrl:   profile.HeroImageURL,
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
	r.cacheOperator(ctx, result, true)
	return result, nil
}

func (r *OperatorRepository) GetByBetterAuthOrgID(ctx context.Context, betterAuthOrgID string) (*domain.Operator, error) {
	if cached, ok := r.cachedOperator(betterAuthOrgID); ok {
		return cached, nil
	}
	if cached, ok := r.redisCachedOperator(ctx, betterAuthOrgID); ok {
		r.setLocalCache(betterAuthOrgID, cached)
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
	r.cacheOperator(ctx, result, false)
	return result, nil
}

func operatorRedisKey(betterAuthOrgID string) string {
	return "safrat:cache:operator:org:" + betterAuthOrgID
}

func (r *OperatorRepository) setLocalCache(betterAuthOrgID string, operator *domain.Operator) {
	ttl := operatorCacheTTL
	if r.rdb != nil {
		ttl = operatorRedisLocalCacheTTL
	}
	r.mu.Lock()
	r.cache[betterAuthOrgID] = operatorCacheEntry{value: operator, expiresAt: time.Now().Add(ttl)}
	r.mu.Unlock()
}

func (r *OperatorRepository) cacheOperator(ctx context.Context, operator *domain.Operator, invalidatePeers bool) {
	if operator == nil || operator.BetterAuthOrgID == "" {
		return
	}
	r.setLocalCache(operator.BetterAuthOrgID, operator)
	if r.rdb == nil {
		return
	}
	raw, err := json.Marshal(operator)
	if err != nil {
		return
	}
	if err := r.rdb.Set(ctx, operatorRedisKey(operator.BetterAuthOrgID), raw, operatorCacheTTL).Err(); err != nil {
		r.logRedisFailure("set operator cache", err)
		return
	}
	if invalidatePeers {
		if err := r.rdb.Publish(ctx, operatorCacheInvalidationChannel, operator.BetterAuthOrgID).Err(); err != nil {
			r.logRedisFailure("publish operator cache invalidation", err)
		}
	}
}

func (r *OperatorRepository) redisCachedOperator(ctx context.Context, betterAuthOrgID string) (*domain.Operator, bool) {
	if r.rdb == nil {
		return nil, false
	}
	raw, err := r.rdb.Get(ctx, operatorRedisKey(betterAuthOrgID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false
	}
	if err != nil {
		r.logRedisFailure("get operator cache", err)
		return nil, false
	}
	var operator domain.Operator
	if err := json.Unmarshal(raw, &operator); err != nil || operator.BetterAuthOrgID != betterAuthOrgID {
		_ = r.rdb.Del(ctx, operatorRedisKey(betterAuthOrgID)).Err()
		return nil, false
	}
	return &operator, true
}

func (r *OperatorRepository) listenForInvalidations(ctx context.Context) {
	pubsub := r.rdb.Subscribe(ctx, operatorCacheInvalidationChannel)
	defer func() { _ = pubsub.Close() }()
	for {
		message, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() == nil {
				r.logRedisFailure("receive operator cache invalidation", err)
			}
			return
		}
		r.mu.Lock()
		delete(r.cache, message.Payload)
		r.mu.Unlock()
	}
}

func (r *OperatorRepository) logRedisFailure(operation string, err error) {
	if r.logger != nil {
		r.logger.Warn(operation, "error", err)
	}
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
		BrandColor:        value.BrandColor,
		HeroEyebrow:       value.HeroEyebrow,
		HeroTitle:         value.HeroTitle,
		HeroSubtitle:      value.HeroSubtitle,
		HeroImageURL:      value.HeroImageUrl,
	}
}
