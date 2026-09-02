package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PlatformPlanLimit struct {
	Plan         string
	MaxPilgrims  *int32
	MaxBranches  *int32
	FeatureFlags map[string]bool
	UpdatedAt    time.Time
}

type PlatformPlanOverride struct {
	OperatorID           string
	OperatorName         string
	Plan                 string
	MaxPilgrims          *int32
	MaxBranches          *int32
	FeatureFlagOverrides map[string]bool
	Note                 string
	ExpiresAt            *time.Time
	UpdatedBy            string
	UpdatedAt            time.Time
}

type AffectedPlanTenant struct {
	OperatorID        string
	Name              string
	PilgrimCount      int32
	ActiveBranchCount int32
	CurrentPilgrimMax *int32
	CurrentBranchMax  *int32
	Reasons           []string
}

type PlanLimitChange struct {
	Plan                string
	MaxPilgrims         *int32
	MaxBranches         *int32
	FeatureFlags        map[string]bool
	Reason              string
	ActorUserID         string
	IdempotencyKey      string
	GrandfatherAffected bool
}

type PlanOverrideChange struct {
	OperatorID           string
	MaxPilgrims          *int32
	MaxBranches          *int32
	FeatureFlagOverrides map[string]bool
	Note                 string
	ExpiresAt            *time.Time
	ActorUserID          string
	IdempotencyKey       string
}

func (r *PlatformRepository) ListPlanLimits(ctx context.Context) ([]PlatformPlanLimit, error) {
	rows, err := r.queries.ListPlatformPlanLimits(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]PlatformPlanLimit, 0, len(rows))
	for _, row := range rows {
		flags, err := decodeBoolMap(row.FeatureFlags)
		if err != nil {
			return nil, err
		}
		result = append(result, PlatformPlanLimit{
			Plan: row.Plan, MaxPilgrims: int32Ptr(row.MaxPilgrims), MaxBranches: int32Ptr(row.MaxBranches),
			FeatureFlags: flags, UpdatedAt: row.UpdatedAt.Time,
		})
	}
	return result, nil
}

func (r *PlatformRepository) GetPlanLimit(ctx context.Context, plan string) (PlatformPlanLimit, error) {
	row, err := r.queries.GetPlatformPlanLimit(ctx, db.Plan(plan))
	if err != nil {
		return PlatformPlanLimit{}, databaseError(err)
	}
	flags, err := decodeBoolMap(row.FeatureFlags)
	if err != nil {
		return PlatformPlanLimit{}, err
	}
	return PlatformPlanLimit{
		Plan: row.Plan, MaxPilgrims: int32Ptr(row.MaxPilgrims), MaxBranches: int32Ptr(row.MaxBranches),
		FeatureFlags: flags, UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *PlatformRepository) ListPlanOverrides(ctx context.Context, includeExpired bool) ([]PlatformPlanOverride, error) {
	rows, err := r.queries.ListPlatformPlanOverrides(ctx, includeExpired)
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]PlatformPlanOverride, 0, len(rows))
	for _, row := range rows {
		item, err := platformPlanOverride(row.OperatorID, row.OperatorName, row.Plan, row.MaxPilgrims,
			row.MaxBranches, row.FeatureFlagOverrides, row.Note, row.ExpiresAt, row.UpdatedBy, row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *PlatformRepository) GetPlanOverride(ctx context.Context, operatorID string) (PlatformPlanOverride, error) {
	id, err := pgUUID(operatorID)
	if err != nil {
		return PlatformPlanOverride{}, apperror.ErrValidation
	}
	row, err := r.queries.GetPlatformPlanOverride(ctx, id)
	if err != nil {
		return PlatformPlanOverride{}, databaseError(err)
	}
	return platformPlanOverride(row.OperatorID, row.OperatorName, row.Plan, row.MaxPilgrims,
		row.MaxBranches, row.FeatureFlagOverrides, row.Note, row.ExpiresAt, row.UpdatedBy, row.UpdatedAt)
}

func (r *PlatformRepository) PreviewPlanLimitChange(ctx context.Context, change PlanLimitChange) ([]AffectedPlanTenant, error) {
	rows, err := r.queries.ListTenantsAffectedByPlanLimit(ctx, db.ListTenantsAffectedByPlanLimitParams{
		Plan: db.Plan(change.Plan), MaxPilgrims: nullableInt4(change.MaxPilgrims),
		MaxBranches: nullableInt4(change.MaxBranches), BranchesEnabled: change.FeatureFlags["branches"],
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return affectedPlanTenants(rows, change), nil
}

// SetPlanLimit changes a commercial term, writes its approval evidence and
// audit row, and optionally protects affected tenants in one serializable
// transaction. The advisory lock makes retries for one plan deterministic;
// database unique constraints remain the final idempotency authority.
func (r *PlatformRepository) SetPlanLimit(ctx context.Context, change PlanLimitChange) (PlatformPlanLimit, int32, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PlatformPlanLimit{}, 0, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "platform-plan:"+change.Plan); err != nil {
		return PlatformPlanLimit{}, 0, databaseError(err)
	}

	fingerprint, requestJSON, err := requestFingerprint(struct {
		Plan                string          `json:"plan"`
		MaxPilgrims         *int32          `json:"max_pilgrims"`
		MaxBranches         *int32          `json:"max_branches"`
		FeatureFlags        map[string]bool `json:"feature_flags"`
		Reason              string          `json:"reason"`
		GrandfatherAffected bool            `json:"grandfather_affected"`
	}{change.Plan, change.MaxPilgrims, change.MaxBranches, change.FeatureFlags, change.Reason, change.GrandfatherAffected})
	if err != nil {
		return PlatformPlanLimit{}, 0, err
	}

	var existingFingerprint string
	var existingGrandfathered int32
	err = tx.QueryRow(ctx, `
		SELECT payload->>'request_fingerprint', COALESCE((payload->>'grandfathered_tenants')::int, 0)
		FROM privileged_actions
		WHERE requested_by = $1 AND kind = 'SET_PLAN_LIMIT' AND idempotency_key = $2`,
		change.ActorUserID, change.IdempotencyKey).Scan(&existingFingerprint, &existingGrandfathered)
	if err == nil {
		if existingFingerprint != fingerprint {
			return PlatformPlanLimit{}, 0, apperror.ErrConflict
		}
		limit, err := getPlanLimitWith(db.New(tx), ctx, change.Plan)
		return limit, existingGrandfathered, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return PlatformPlanLimit{}, 0, databaseError(err)
	}

	q := db.New(tx)
	rows, err := q.ListTenantsAffectedByPlanLimit(ctx, db.ListTenantsAffectedByPlanLimitParams{
		Plan: db.Plan(change.Plan), MaxPilgrims: nullableInt4(change.MaxPilgrims),
		MaxBranches: nullableInt4(change.MaxBranches), BranchesEnabled: change.FeatureFlags["branches"],
	})
	if err != nil {
		return PlatformPlanLimit{}, 0, databaseError(err)
	}

	grandfathered := int32(0)
	if change.GrandfatherAffected {
		for _, row := range rows {
			pilgrimMax := pgtype.Int4{}
			branchMax := pgtype.Int4{}
			flagOverrides := map[string]bool{}
			if change.MaxPilgrims != nil && row.PilgrimCount > *change.MaxPilgrims {
				pilgrimMax = row.CurrentMaxPilgrims
				if !pilgrimMax.Valid {
					pilgrimMax = pgtype.Int4{Int32: row.PilgrimCount, Valid: true}
				}
			}
			if change.MaxBranches != nil && row.ActiveBranchCount > *change.MaxBranches {
				branchMax = row.CurrentMaxBranches
				if !branchMax.Valid {
					branchMax = pgtype.Int4{Int32: row.ActiveBranchCount, Valid: true}
				}
			}
			if !change.FeatureFlags["branches"] && row.ActiveBranchCount > 0 {
				flagOverrides["branches"] = true
			}
			flagJSON, _ := json.Marshal(flagOverrides)
			note := fmt.Sprintf("grandfathered dari batas %s, %s", change.Plan, time.Now().UTC().Format("2006-01-02"))
			if _, err := tx.Exec(ctx, `
				INSERT INTO plan_overrides (
				  operator_id, max_pilgrims, max_branches, feature_flag_overrides, note, updated_by
				) VALUES ($1,$2,$3,$4,$5,$6)
				ON CONFLICT (operator_id) DO UPDATE SET
				  max_pilgrims = COALESCE(EXCLUDED.max_pilgrims, plan_overrides.max_pilgrims),
				  max_branches = COALESCE(EXCLUDED.max_branches, plan_overrides.max_branches),
				  feature_flag_overrides = plan_overrides.feature_flag_overrides || EXCLUDED.feature_flag_overrides,
				  note = plan_overrides.note || '; ' || EXCLUDED.note,
				  updated_by = EXCLUDED.updated_by,
				  updated_at = NOW()`, row.OperatorID, pilgrimMax, branchMax, flagJSON, note, change.ActorUserID); err != nil {
				return PlatformPlanLimit{}, 0, databaseError(err)
			}
			grandfathered++
		}
	}

	flagsJSON, err := json.Marshal(change.FeatureFlags)
	if err != nil {
		return PlatformPlanLimit{}, 0, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE plan_limits
		SET max_pilgrims = $2, max_branches = $3, feature_flags = $4, updated_at = NOW()
		WHERE plan = $1::plan`, change.Plan, nullableInt4(change.MaxPilgrims), nullableInt4(change.MaxBranches), flagsJSON); err != nil {
		return PlatformPlanLimit{}, 0, databaseError(err)
	}

	payload := map[string]any{
		"request": json.RawMessage(requestJSON), "request_fingerprint": fingerprint,
		"grandfathered_tenants": grandfathered,
	}
	payloadJSON, _ := json.Marshal(payload)
	if _, err := tx.Exec(ctx, `
		INSERT INTO privileged_actions (
		  kind, payload, reason, requested_by, approved_by, executed_at, idempotency_key
		) VALUES ('SET_PLAN_LIMIT',$1,$2,$3,$3,NOW(),$4)`,
		payloadJSON, change.Reason, change.ActorUserID, change.IdempotencyKey); err != nil {
		return PlatformPlanLimit{}, 0, databaseError(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (operator_id,user_id,action,entity_type,entity_id,metadata)
		VALUES (NULL,$1,'plan_limit_changed','plan_limit',$2,$3)`, change.ActorUserID, change.Plan,
		map[string]any{"message": change.Reason, "idempotency_key": change.IdempotencyKey, "request_fingerprint": fingerprint, "grandfathered_tenants": grandfathered}); err != nil {
		return PlatformPlanLimit{}, 0, databaseError(err)
	}

	limit, err := getPlanLimitWith(q, ctx, change.Plan)
	if err != nil {
		return PlatformPlanLimit{}, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PlatformPlanLimit{}, 0, databaseError(err)
	}
	return limit, grandfathered, nil
}

func (r *PlatformRepository) SetPlanOverride(ctx context.Context, change PlanOverrideChange) (PlatformPlanOverride, error) {
	operatorID, err := pgUUID(change.OperatorID)
	if err != nil {
		return PlatformPlanOverride{}, apperror.ErrValidation
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PlatformPlanOverride{}, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "platform-override:"+change.OperatorID); err != nil {
		return PlatformPlanOverride{}, databaseError(err)
	}
	fingerprint, _, err := requestFingerprint(change)
	if err != nil {
		return PlatformPlanOverride{}, err
	}
	duplicate, err := checkPlatformMutationKey(ctx, tx, change.ActorUserID, "plan_override_set", change.IdempotencyKey, fingerprint)
	if err != nil {
		return PlatformPlanOverride{}, err
	}
	q := db.New(tx)
	if duplicate {
		row, err := q.GetPlatformPlanOverride(ctx, operatorID)
		if err != nil {
			return PlatformPlanOverride{}, databaseError(err)
		}
		return platformPlanOverride(row.OperatorID, row.OperatorName, row.Plan, row.MaxPilgrims,
			row.MaxBranches, row.FeatureFlagOverrides, row.Note, row.ExpiresAt, row.UpdatedBy, row.UpdatedAt)
	}
	flagsJSON, err := json.Marshal(change.FeatureFlagOverrides)
	if err != nil {
		return PlatformPlanOverride{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO plan_overrides (
		  operator_id,max_pilgrims,max_branches,feature_flag_overrides,note,expires_at,updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (operator_id) DO UPDATE SET
		  max_pilgrims = EXCLUDED.max_pilgrims,
		  max_branches = EXCLUDED.max_branches,
		  feature_flag_overrides = EXCLUDED.feature_flag_overrides,
		  note = EXCLUDED.note,
		  expires_at = EXCLUDED.expires_at,
		  updated_by = EXCLUDED.updated_by,
		  updated_at = NOW()`, operatorID, nullableInt4(change.MaxPilgrims), nullableInt4(change.MaxBranches),
		flagsJSON, change.Note, nullableTime(change.ExpiresAt), change.ActorUserID); err != nil {
		return PlatformPlanOverride{}, databaseError(err)
	}
	if err := insertPlatformMutationAudit(ctx, tx, change.OperatorID, change.ActorUserID, "plan_override_set",
		"plan_override", change.OperatorID, change.Note, change.IdempotencyKey, fingerprint); err != nil {
		return PlatformPlanOverride{}, err
	}
	row, err := q.GetPlatformPlanOverride(ctx, operatorID)
	if err != nil {
		return PlatformPlanOverride{}, databaseError(err)
	}
	result, err := platformPlanOverride(row.OperatorID, row.OperatorName, row.Plan, row.MaxPilgrims,
		row.MaxBranches, row.FeatureFlagOverrides, row.Note, row.ExpiresAt, row.UpdatedBy, row.UpdatedAt)
	if err != nil {
		return PlatformPlanOverride{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PlatformPlanOverride{}, databaseError(err)
	}
	return result, nil
}

func (r *PlatformRepository) DeletePlanOverride(ctx context.Context, operatorID, actorUserID, reason, idempotencyKey string) error {
	id, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "platform-override:"+operatorID); err != nil {
		return databaseError(err)
	}
	fingerprint, _, err := requestFingerprint(struct {
		OperatorID string `json:"operator_id"`
		Reason     string `json:"reason"`
	}{operatorID, reason})
	if err != nil {
		return err
	}
	duplicate, err := checkPlatformMutationKey(ctx, tx, actorUserID, "plan_override_deleted", idempotencyKey, fingerprint)
	if err != nil {
		return err
	}
	if duplicate {
		return nil
	}
	command, err := tx.Exec(ctx, `DELETE FROM plan_overrides WHERE operator_id = $1`, id)
	if err != nil {
		return databaseError(err)
	}
	if command.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	if err := insertPlatformMutationAudit(ctx, tx, operatorID, actorUserID, "plan_override_deleted",
		"plan_override", operatorID, reason, idempotencyKey, fingerprint); err != nil {
		return err
	}
	return databaseError(tx.Commit(ctx))
}

func (r *PlatformRepository) ExpirePlanOverrides(ctx context.Context) (int32, error) {
	count, err := r.queries.ExpirePlatformPlanOverrides(ctx)
	return count, databaseError(err)
}

func affectedPlanTenants(rows []db.ListTenantsAffectedByPlanLimitRow, change PlanLimitChange) []AffectedPlanTenant {
	result := make([]AffectedPlanTenant, 0, len(rows))
	for _, row := range rows {
		reasons := make([]string, 0, 3)
		if change.MaxPilgrims != nil && row.PilgrimCount > *change.MaxPilgrims {
			reasons = append(reasons, "jamaah")
		}
		if change.MaxBranches != nil && row.ActiveBranchCount > *change.MaxBranches {
			reasons = append(reasons, "cabang")
		}
		if !change.FeatureFlags["branches"] && row.ActiveBranchCount > 0 {
			reasons = append(reasons, "fitur cabang")
		}
		result = append(result, AffectedPlanTenant{
			OperatorID: uuidString(row.OperatorID), Name: row.Name, PilgrimCount: row.PilgrimCount,
			ActiveBranchCount: row.ActiveBranchCount, CurrentPilgrimMax: int32Ptr(row.CurrentMaxPilgrims),
			CurrentBranchMax: int32Ptr(row.CurrentMaxBranches), Reasons: reasons,
		})
	}
	return result
}

func getPlanLimitWith(q *db.Queries, ctx context.Context, plan string) (PlatformPlanLimit, error) {
	row, err := q.GetPlatformPlanLimit(ctx, db.Plan(plan))
	if err != nil {
		return PlatformPlanLimit{}, databaseError(err)
	}
	flags, err := decodeBoolMap(row.FeatureFlags)
	if err != nil {
		return PlatformPlanLimit{}, err
	}
	return PlatformPlanLimit{Plan: row.Plan, MaxPilgrims: int32Ptr(row.MaxPilgrims),
		MaxBranches: int32Ptr(row.MaxBranches), FeatureFlags: flags, UpdatedAt: row.UpdatedAt.Time}, nil
}

func platformPlanOverride(operatorID pgtype.UUID, operatorName, plan string, maxPilgrims, maxBranches pgtype.Int4,
	flagsJSON []byte, note string, expiresAt pgtype.Timestamptz, updatedBy string, updatedAt pgtype.Timestamptz) (PlatformPlanOverride, error) {
	flags, err := decodeBoolMap(flagsJSON)
	if err != nil {
		return PlatformPlanOverride{}, err
	}
	var expiry *time.Time
	if expiresAt.Valid {
		value := expiresAt.Time
		expiry = &value
	}
	return PlatformPlanOverride{
		OperatorID: uuidString(operatorID), OperatorName: operatorName, Plan: plan,
		MaxPilgrims: int32Ptr(maxPilgrims), MaxBranches: int32Ptr(maxBranches), FeatureFlagOverrides: flags,
		Note: note, ExpiresAt: expiry, UpdatedBy: updatedBy, UpdatedAt: updatedAt.Time,
	}, nil
}

func decodeBoolMap(raw []byte) (map[string]bool, error) {
	result := map[string]bool{}
	if len(raw) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode feature flags: %w", err)
	}
	return result, nil
}

func int32Ptr(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	result := value.Int32
	return &result
}

func nullableInt4(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *value, Valid: true}
}

func nullableTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func requestFingerprint(value any) (string, []byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), raw, nil
}

func checkPlatformMutationKey(ctx context.Context, tx pgx.Tx, userID, action, key, fingerprint string) (bool, error) {
	var existing string
	err := tx.QueryRow(ctx, `
		SELECT metadata->>'request_fingerprint'
		FROM audit_logs
		WHERE user_id = $1 AND action = $2 AND metadata->>'idempotency_key' = $3`, userID, action, key).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, databaseError(err)
	}
	if existing != fingerprint {
		return false, apperror.ErrConflict
	}
	return true, nil
}

func insertPlatformMutationAudit(ctx context.Context, tx pgx.Tx, operatorID, userID, action,
	entityType, entityID, message, key, fingerprint string) error {
	opID, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	metadata := map[string]any{"message": message, "idempotency_key": key, "request_fingerprint": fingerprint}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (operator_id,user_id,action,entity_type,entity_id,metadata)
		VALUES ($1,$2,$3,$4,$5,$6)`, opID, userID, action, entityType, entityID, metadata)
	return databaseError(err)
}

// EqualJSON is used only for defensive tests and diagnostics where JSONB may
// have normalized whitespace. Keeping semantic comparison here avoids callers
// accidentally treating formatting as a different request.
func equalJSON(left, right []byte) bool {
	var a, b any
	return json.Unmarshal(left, &a) == nil && json.Unmarshal(right, &b) == nil && reflect.DeepEqual(a, b)
}
