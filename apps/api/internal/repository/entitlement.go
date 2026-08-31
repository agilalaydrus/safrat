package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
)

// EntitlementRepository is deliberately read-only. PostgreSQL triggers enforce
// the same limits at write time; this repository lets services explain a limit
// before attempting the action.
type EntitlementRepository struct{ queries *db.Queries }

func NewEntitlementRepository(queries *db.Queries) *EntitlementRepository {
	return &EntitlementRepository{queries: queries}
}

func (r *EntitlementRepository) Get(ctx context.Context, operatorID string) (domain.Entitlement, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return domain.Entitlement{}, apperror.ErrValidation
	}
	limits, err := r.queries.GetOperatorEntitlement(ctx, opUUID)
	if err != nil {
		return domain.Entitlement{}, databaseError(err)
	}
	usage, err := r.queries.GetOperatorEntitlementUsage(ctx, opUUID)
	if err != nil {
		return domain.Entitlement{}, databaseError(err)
	}
	features := make(map[string]bool)
	var rawFeatures []byte
	switch value := limits.FeatureFlags.(type) {
	case []byte:
		rawFeatures = value
	case string:
		rawFeatures = []byte(value)
	case map[string]interface{}:
		rawFeatures, err = json.Marshal(value)
		if err != nil {
			return domain.Entitlement{}, err
		}
	default:
		return domain.Entitlement{}, fmt.Errorf("decode entitlement feature flags: %T", limits.FeatureFlags)
	}
	if err := json.Unmarshal(rawFeatures, &features); err != nil {
		return domain.Entitlement{}, err
	}
	entitlement := domain.Entitlement{
		Features: features, PilgrimCount: usage.PilgrimCount, BranchCount: usage.ActiveBranchCount,
	}
	if limits.MaxPilgrims.Valid {
		value := limits.MaxPilgrims.Int32
		entitlement.MaxPilgrims = &value
	}
	if limits.MaxBranches.Valid {
		value := limits.MaxBranches.Int32
		entitlement.MaxBranches = &value
	}
	return entitlement, nil
}
