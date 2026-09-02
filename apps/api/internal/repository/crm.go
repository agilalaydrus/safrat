package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CRMRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewCRMRepository(pool *pgxpool.Pool) *CRMRepository {
	return &CRMRepository{pool: pool, queries: db.New(pool)}
}

func (r *CRMRepository) CreateLead(ctx context.Context, operatorID, userID string, draft domain.CRMLeadDraft) (*domain.CRMLead, bool, error) {
	op, err := pgUUID(operatorID)
	if err != nil || strings.TrimSpace(userID) == "" {
		return nil, false, apperror.ErrValidation
	}
	fingerprint := crmFingerprint(draft)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.queries.WithTx(tx)
	scope, err := branchScope(ctx, q, op)
	if err != nil {
		return nil, false, err
	}
	if err := q.LockCRMIdempotencyKey(ctx, db.LockCRMIdempotencyKeyParams{OperatorID: op, IdempotencyKey: draft.IdempotencyKey}); err != nil {
		return nil, false, err
	}
	existing, err := q.GetCRMLeadByIdempotency(ctx, db.GetCRMLeadByIdempotencyParams{OperatorID: op, IdempotencyKey: draft.IdempotencyKey, BranchScope: scope})
	if err == nil {
		if existing.RequestFingerprint != fingerprint {
			return nil, false, apperror.ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		lead, err := r.GetLead(ctx, operatorID, uuidString(existing.ID))
		return lead, false, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, databaseError(err)
	}
	lead, err := q.CreateCRMLead(ctx, db.CreateCRMLeadParams{
		OperatorID: op, BranchScope: scope, FullName: strings.TrimSpace(draft.FullName),
		Phone: strings.TrimSpace(draft.Phone), Email: strings.ToLower(strings.TrimSpace(draft.Email)),
		Source: draft.Source, Campaign: strings.TrimSpace(draft.Campaign),
		SeasonID: pgUUIDOrNull(draft.SeasonID), ProductID: pgUUIDOrNull(draft.ProductID),
		AssigneeUserID: pgText(draft.AssigneeUserID), Pax: draft.Pax,
		EstimatedValueIdr: draft.EstimatedValueIDR, NextAction: strings.TrimSpace(draft.NextAction),
		NextFollowUpAt: optionalTimestamp(draft.NextFollowUpAt), CreatedByUserID: userID,
		IdempotencyKey: draft.IdempotencyKey, RequestFingerprint: fingerprint,
	})
	if err != nil {
		return nil, false, databaseError(err)
	}
	activity, err := q.CreateCRMLeadActivity(ctx, db.CreateCRMLeadActivityParams{
		LeadID: lead.ID, OperatorID: op, BranchID: lead.BranchID, Kind: "CREATED",
		Note: strings.TrimSpace(draft.Note), ActorUserID: userID,
		IdempotencyKey:     "created:" + draft.IdempotencyKey,
		RequestFingerprint: fingerprint, OccurredAt: pgTimestamp(time.Now()),
	})
	if err != nil {
		return nil, false, databaseError(err)
	}
	if err := q.SetCRMLeadInitialActivity(ctx, db.SetCRMLeadInitialActivityParams{
		ActivityID: activity.ID, ID: lead.ID, OperatorID: op, BranchScope: scope,
	}); err != nil {
		return nil, false, databaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, databaseError(err)
	}
	result, err := r.GetLead(ctx, operatorID, uuidString(lead.ID))
	return result, true, err
}

func (r *CRMRepository) ListLeads(ctx context.Context, operatorID string, filter domain.CRMLeadFilter) ([]domain.CRMLead, int64, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, 0, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.queries.ListCRMLeads(ctx, db.ListCRMLeadsParams{
		OperatorID: op, BranchScope: scope, Stage: pgText(filter.Stage), Source: pgText(filter.Source),
		Search: strings.TrimSpace(filter.Search), ResultLimit: limit, ResultOffset: filter.Offset,
	})
	if err != nil {
		return nil, 0, databaseError(err)
	}
	result := make([]domain.CRMLead, 0, len(rows))
	var total int64
	for _, row := range rows {
		result = append(result, crmLeadFromList(row))
		total = row.TotalCount
	}
	return result, total, nil
}

func (r *CRMRepository) GetLead(ctx context.Context, operatorID, leadID string) (*domain.CRMLead, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	id, err := pgUUID(leadID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetCRMLead(ctx, db.GetCRMLeadParams{ID: id, OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	lead := crmLeadFromGet(row)
	return &lead, nil
}

func (r *CRMRepository) GetLeadDetail(ctx context.Context, operatorID, leadID string) (*domain.CRMLeadDetail, error) {
	lead, err := r.GetLead(ctx, operatorID, leadID)
	if err != nil {
		return nil, err
	}
	op, _ := pgUUID(operatorID)
	id, _ := pgUUID(leadID)
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListCRMLeadActivities(ctx, db.ListCRMLeadActivitiesParams{LeadID: id, OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	activities := make([]domain.CRMLeadActivity, 0, len(rows))
	for _, row := range rows {
		activities = append(activities, domain.CRMLeadActivity{
			ID: uuidString(row.ID), LeadID: uuidString(row.LeadID), Kind: row.Kind,
			FromStage: row.FromStage.String, ToStage: row.ToStage.String, Note: row.Note,
			ActorUserID: row.ActorUserID, ActorName: row.ActorName, OccurredAt: row.OccurredAt.Time,
		})
	}
	return &domain.CRMLeadDetail{Lead: *lead, Activities: activities}, nil
}

func (r *CRMRepository) UpdateLead(ctx context.Context, operatorID, userID, leadID, reason, idempotencyKey string, draft domain.CRMLeadDraft) (*domain.CRMLead, bool, error) {
	fingerprint := crmFingerprint(struct {
		LeadID, Reason string
		Draft          domain.CRMLeadDraft
	}{leadID, strings.TrimSpace(reason), draft})
	return r.mutate(ctx, operatorID, userID, leadID, idempotencyKey, fingerprint, func(q *db.Queries, op, scope pgtype.UUID, current db.GetCRMLeadRow) (db.CrmLead, error) {
		activity, err := q.CreateCRMLeadActivity(ctx, db.CreateCRMLeadActivityParams{
			LeadID: current.ID, OperatorID: op, BranchID: current.BranchID, Kind: "PROFILE_UPDATED",
			Note: strings.TrimSpace(reason), ActorUserID: userID, IdempotencyKey: idempotencyKey,
			RequestFingerprint: fingerprint, OccurredAt: pgTimestamp(time.Now()),
		})
		if err != nil {
			return db.CrmLead{}, err
		}
		return q.UpdateCRMLeadProfile(ctx, db.UpdateCRMLeadProfileParams{
			FullName: strings.TrimSpace(draft.FullName), Phone: strings.TrimSpace(draft.Phone),
			Email: strings.ToLower(strings.TrimSpace(draft.Email)), Source: draft.Source,
			Campaign: strings.TrimSpace(draft.Campaign), SeasonID: pgUUIDOrNull(draft.SeasonID),
			ProductID: pgUUIDOrNull(draft.ProductID), AssigneeUserID: pgText(draft.AssigneeUserID),
			Pax: draft.Pax, EstimatedValueIdr: draft.EstimatedValueIDR,
			NextAction: strings.TrimSpace(draft.NextAction), NextFollowUpAt: optionalTimestamp(draft.NextFollowUpAt),
			ActivityID: activity.ID, ID: current.ID, OperatorID: op, BranchScope: scope,
		})
	})
}

func (r *CRMRepository) MoveStage(ctx context.Context, operatorID, userID, leadID, stage, reason, idempotencyKey string) (*domain.CRMLead, bool, error) {
	fingerprint := crmFingerprint([]string{leadID, stage, strings.TrimSpace(reason)})
	return r.mutate(ctx, operatorID, userID, leadID, idempotencyKey, fingerprint, func(q *db.Queries, op, scope pgtype.UUID, current db.GetCRMLeadRow) (db.CrmLead, error) {
		if current.Stage == stage {
			return db.CrmLead{}, apperror.ErrConflict
		}
		activity, err := q.CreateCRMLeadActivity(ctx, db.CreateCRMLeadActivityParams{
			LeadID: current.ID, OperatorID: op, BranchID: current.BranchID, Kind: "STAGE_CHANGED",
			FromStage: pgText(current.Stage), ToStage: pgText(stage), Note: strings.TrimSpace(reason),
			ActorUserID: userID, IdempotencyKey: idempotencyKey,
			RequestFingerprint: fingerprint, OccurredAt: pgTimestamp(time.Now()),
		})
		if err != nil {
			return db.CrmLead{}, err
		}
		return q.MoveCRMLeadStage(ctx, db.MoveCRMLeadStageParams{
			Stage: stage, ActivityID: activity.ID, ID: current.ID, OperatorID: op, BranchScope: scope,
		})
	})
}

func (r *CRMRepository) AddActivity(ctx context.Context, operatorID, userID, leadID, kind, note, nextAction, idempotencyKey string, occurredAt time.Time, nextFollowUpAt *time.Time) (*domain.CRMLeadActivity, bool, error) {
	fingerprint := crmFingerprint(struct {
		LeadID, Kind, Note, NextAction string
		OccurredAt                     time.Time
		NextFollowUpAt                 *time.Time
	}{leadID, kind, strings.TrimSpace(note), strings.TrimSpace(nextAction), occurredAt, nextFollowUpAt})
	op, err := pgUUID(operatorID)
	if err != nil || strings.TrimSpace(userID) == "" {
		return nil, false, apperror.ErrValidation
	}
	id, err := pgUUID(leadID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.queries.WithTx(tx)
	scope, err := branchScope(ctx, q, op)
	if err != nil {
		return nil, false, err
	}
	if err := q.LockCRMIdempotencyKey(ctx, db.LockCRMIdempotencyKeyParams{OperatorID: op, IdempotencyKey: idempotencyKey}); err != nil {
		return nil, false, err
	}
	existing, existingErr := q.GetCRMActivityByIdempotency(ctx, db.GetCRMActivityByIdempotencyParams{OperatorID: op, IdempotencyKey: idempotencyKey, BranchScope: scope})
	if existingErr == nil {
		if existing.RequestFingerprint != fingerprint || existing.LeadID != id {
			return nil, false, apperror.ErrConflict
		}
		return crmActivity(existing), false, tx.Commit(ctx)
	}
	if !errors.Is(existingErr, pgx.ErrNoRows) {
		return nil, false, databaseError(existingErr)
	}
	current, err := q.GetCRMLead(ctx, db.GetCRMLeadParams{ID: id, OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, false, databaseError(err)
	}
	activity, err := q.CreateCRMLeadActivity(ctx, db.CreateCRMLeadActivityParams{
		LeadID: id, OperatorID: op, BranchID: current.BranchID, Kind: kind, Note: strings.TrimSpace(note),
		ActorUserID: userID, IdempotencyKey: idempotencyKey, RequestFingerprint: fingerprint,
		OccurredAt: pgTimestamp(occurredAt),
	})
	if err != nil {
		return nil, false, databaseError(err)
	}
	if _, err := q.ApplyCRMLeadActivity(ctx, db.ApplyCRMLeadActivityParams{
		Kind: kind, OccurredAt: pgTimestamp(occurredAt), NextAction: strings.TrimSpace(nextAction),
		NextFollowUpAt: optionalTimestamp(nextFollowUpAt), ActivityID: activity.ID,
		ID: id, OperatorID: op, BranchScope: scope,
	}); err != nil {
		return nil, false, databaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, databaseError(err)
	}
	return crmActivity(activity), true, nil
}

type crmMutation func(*db.Queries, pgtype.UUID, pgtype.UUID, db.GetCRMLeadRow) (db.CrmLead, error)

func (r *CRMRepository) mutate(ctx context.Context, operatorID, userID, leadID, idempotencyKey, fingerprint string, apply crmMutation) (*domain.CRMLead, bool, error) {
	op, err := pgUUID(operatorID)
	if err != nil || strings.TrimSpace(userID) == "" {
		return nil, false, apperror.ErrValidation
	}
	id, err := pgUUID(leadID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.queries.WithTx(tx)
	scope, err := branchScope(ctx, q, op)
	if err != nil {
		return nil, false, err
	}
	if err := q.LockCRMIdempotencyKey(ctx, db.LockCRMIdempotencyKeyParams{OperatorID: op, IdempotencyKey: idempotencyKey}); err != nil {
		return nil, false, err
	}
	existing, existingErr := q.GetCRMActivityByIdempotency(ctx, db.GetCRMActivityByIdempotencyParams{OperatorID: op, IdempotencyKey: idempotencyKey, BranchScope: scope})
	if existingErr == nil {
		if existing.RequestFingerprint != fingerprint || existing.LeadID != id {
			return nil, false, apperror.ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		lead, err := r.GetLead(ctx, operatorID, leadID)
		return lead, false, err
	}
	if !errors.Is(existingErr, pgx.ErrNoRows) {
		return nil, false, databaseError(existingErr)
	}
	current, err := q.GetCRMLead(ctx, db.GetCRMLeadParams{ID: id, OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, false, databaseError(err)
	}
	if _, err := apply(q, op, scope, current); err != nil {
		return nil, false, databaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, databaseError(err)
	}
	lead, err := r.GetLead(ctx, operatorID, leadID)
	return lead, true, err
}

func (r *CRMRepository) Dashboard(ctx context.Context, operatorID string) (*domain.CRMDashboard, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, err
	}
	summary, err := r.queries.GetCRMPipelineSummary(ctx, db.GetCRMPipelineSummaryParams{OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	stages, err := r.queries.ListCRMStageMetrics(ctx, db.ListCRMStageMetricsParams{OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	sources, err := r.queries.ListCRMSourceMetrics(ctx, db.ListCRMSourceMetricsParams{OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	assignees, err := r.queries.ListCRMAssigneeMetrics(ctx, db.ListCRMAssigneeMetricsParams{OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	attention, err := r.queries.ListCRMAttentionLeads(ctx, db.ListCRMAttentionLeadsParams{OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	result := &domain.CRMDashboard{
		ActiveCount: summary.ActiveCount, PipelineValueIDR: summary.PipelineValueIdr,
		OverdueFollowUpCount: summary.OverdueFollowUpCount, SourceCount: summary.SourceCount,
		UpdatedAt: time.Now(),
	}
	if summary.MonthlyCreatedCount > 0 {
		result.MonthlyConversionBPS = int32(summary.MonthlyClosingCount * 10000 / summary.MonthlyCreatedCount)
	}
	for _, row := range stages {
		result.Stages = append(result.Stages, domain.CRMStageMetric{Stage: row.Stage, Count: row.LeadCount, ValueIDR: row.ValueIdr})
	}
	for _, row := range sources {
		result.Sources = append(result.Sources, domain.CRMSourceMetric{Source: row.Source, Count: row.LeadCount, ValueIDR: row.ValueIdr})
	}
	for _, row := range assignees {
		result.Assignees = append(result.Assignees, domain.CRMAssigneeMetric{UserID: row.UserID, Name: row.Name, ActiveCount: row.ActiveCount, ClosingCount: row.ClosingCount, ValueIDR: row.ValueIdr})
	}
	for _, row := range attention {
		result.AttentionLeads = append(result.AttentionLeads, crmLeadFromAttention(row))
	}
	return result, nil
}

func (r *CRMRepository) ListAssignees(ctx context.Context, operatorID string) ([]domain.CRMAssignee, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListCRMAssignees(ctx, db.ListCRMAssigneesParams{OperatorID: op, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]domain.CRMAssignee, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.CRMAssignee{UserID: row.UserID, Name: row.Name, Email: row.Email})
	}
	return result, nil
}

func crmFingerprint(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func optionalTimestamp(value *time.Time) pgtype.Timestamptz {
	if value == nil || value.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgTimestamp(value.UTC())
}

func timestampPointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func crmLeadValues(id, operatorID, branchID pgtype.UUID, fullName, phone, email, source, campaign, stage string,
	seasonID, productID pgtype.UUID, assigneeID pgtype.Text, pax int32, value int64, nextAction string,
	nextFollowUp, lastContact, closed, createdAt, updatedAt pgtype.Timestamptz,
	seasonName, productName, assigneeName string) domain.CRMLead {
	return domain.CRMLead{
		ID: uuidString(id), OperatorID: uuidString(operatorID), BranchID: nullableUUIDString(branchID),
		FullName: fullName, Phone: phone, Email: email, Source: source, Campaign: campaign, Stage: stage,
		SeasonID: nullableUUIDString(seasonID), SeasonName: seasonName,
		ProductID: nullableUUIDString(productID), ProductName: productName,
		AssigneeUserID: assigneeID.String, AssigneeName: assigneeName, Pax: pax,
		EstimatedValueIDR: value, NextAction: nextAction, NextFollowUpAt: timestampPointer(nextFollowUp),
		LastContactAt: timestampPointer(lastContact), ClosedAt: timestampPointer(closed),
		CreatedAt: createdAt.Time, UpdatedAt: updatedAt.Time,
	}
}

func crmLeadFromGet(row db.GetCRMLeadRow) domain.CRMLead {
	return crmLeadValues(row.ID, row.OperatorID, row.BranchID, row.FullName, row.Phone, row.Email,
		row.Source, row.Campaign, row.Stage, row.SeasonID, row.ProductID, row.AssigneeUserID,
		row.Pax, row.EstimatedValueIdr, row.NextAction, row.NextFollowUpAt, row.LastContactAt,
		row.ClosedAt, row.CreatedAt, row.UpdatedAt, row.SeasonName, row.ProductName, row.AssigneeName)
}

func crmLeadFromList(row db.ListCRMLeadsRow) domain.CRMLead {
	return crmLeadValues(row.ID, row.OperatorID, row.BranchID, row.FullName, row.Phone, row.Email,
		row.Source, row.Campaign, row.Stage, row.SeasonID, row.ProductID, row.AssigneeUserID,
		row.Pax, row.EstimatedValueIdr, row.NextAction, row.NextFollowUpAt, row.LastContactAt,
		row.ClosedAt, row.CreatedAt, row.UpdatedAt, row.SeasonName, row.ProductName, row.AssigneeName)
}

func crmLeadFromAttention(row db.ListCRMAttentionLeadsRow) domain.CRMLead {
	return crmLeadValues(row.ID, row.OperatorID, row.BranchID, row.FullName, row.Phone, row.Email,
		row.Source, row.Campaign, row.Stage, row.SeasonID, row.ProductID, row.AssigneeUserID,
		row.Pax, row.EstimatedValueIdr, row.NextAction, row.NextFollowUpAt, row.LastContactAt,
		row.ClosedAt, row.CreatedAt, row.UpdatedAt, row.SeasonName, row.ProductName, row.AssigneeName)
}

func crmActivity(row db.CrmLeadActivity) *domain.CRMLeadActivity {
	return &domain.CRMLeadActivity{
		ID: uuidString(row.ID), LeadID: uuidString(row.LeadID), Kind: row.Kind,
		FromStage: row.FromStage.String, ToStage: row.ToStage.String, Note: row.Note,
		ActorUserID: row.ActorUserID, OccurredAt: row.OccurredAt.Time,
	}
}
