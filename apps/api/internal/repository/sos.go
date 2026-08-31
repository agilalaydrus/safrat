package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SOSRepository struct{ queries *db.Queries }

func NewSOSRepository(queries *db.Queries) *SOSRepository {
	return &SOSRepository{queries: queries}
}

// Create inserts an SOS alert. When idempotencyKey is non-empty and one already
// exists for this pilgrim+key (an offline replay), it returns the existing alert
// with created=false — the service uses that to skip re-notifying.
func (r *SOSRepository) Create(ctx context.Context, operatorID, pilgrimID string, lat, lng *float64, idempotencyKey string) (alert *domain.SOSAlert, created bool, err error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, false, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, false, err
	}
	row, err := r.queries.CreateSOSAlert(ctx, db.CreateSOSAlertParams{OperatorID: opUUID, PilgrimID: pilgrimUUID, Lat: float8Value(lat), Lng: float8Value(lng), IdempotencyKey: idempotencyKey})
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT DO NOTHING returned nothing — a duplicate key. Return
		// the already-recorded alert.
		existing, getErr := r.queries.GetSOSAlertByIdempotencyKey(ctx, db.GetSOSAlertByIdempotencyKeyParams{PilgrimID: pilgrimUUID, IdempotencyKey: idempotencyKey})
		if getErr != nil {
			return nil, false, getErr
		}
		return toSOSAlert(existing, ""), false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return toSOSAlert(row, ""), true, nil
}

func float8Value(value *float64) pgtype.Float8 {
	if value == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *value, Valid: true}
}

func (r *SOSRepository) ListActive(ctx context.Context, operatorID string) ([]*domain.SOSAlert, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListActiveSOSAlerts(ctx, db.ListActiveSOSAlertsParams{OperatorID: opUUID, BranchScope: scope})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.SOSAlert, 0, len(rows))
	for _, row := range rows {
		alert := db.SosAlert{ID: row.ID, OperatorID: row.OperatorID, PilgrimID: row.PilgrimID, Status: row.Status, AcknowledgedBy: row.AcknowledgedBy, AcknowledgedAt: row.AcknowledgedAt, ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt, Notes: row.Notes, CreatedAt: row.CreatedAt, Lat: row.Lat, Lng: row.Lng}
		result = append(result, toSOSAlert(alert, row.PilgrimName))
	}
	return result, nil
}

func (r *SOSRepository) ListActiveForLeader(ctx context.Context, operatorID, leaderUserID string) ([]*domain.SOSAlert, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListActiveSOSAlertsForLeader(ctx, db.ListActiveSOSAlertsForLeaderParams{OperatorID: opUUID, LeaderID: pgText(leaderUserID), BranchScope: scope})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.SOSAlert, 0, len(rows))
	for _, row := range rows {
		alert := db.SosAlert{ID: row.ID, OperatorID: row.OperatorID, PilgrimID: row.PilgrimID, Status: row.Status, AcknowledgedBy: row.AcknowledgedBy, AcknowledgedAt: row.AcknowledgedAt, ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt, Notes: row.Notes, CreatedAt: row.CreatedAt, Lat: row.Lat, Lng: row.Lng}
		result = append(result, toSOSAlert(alert, row.PilgrimName))
	}
	return result, nil
}

// EscalateStale flips every ACTIVE alert older than 10 minutes to ESCALATED,
// across all operators — the worker's periodic sweep, not scoped to one operator.
func (r *SOSRepository) EscalateStale(ctx context.Context) ([]*domain.SOSAlert, error) {
	rows, err := r.queries.EscalateStaleSOSAlerts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.SOSAlert, 0, len(rows))
	for _, row := range rows {
		alert := db.SosAlert{ID: row.ID, OperatorID: row.OperatorID, PilgrimID: row.PilgrimID, Status: row.Status, AcknowledgedBy: row.AcknowledgedBy, AcknowledgedAt: row.AcknowledgedAt, ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt, Notes: row.Notes, CreatedAt: row.CreatedAt, Lat: row.Lat, Lng: row.Lng}
		result = append(result, toSOSAlert(alert, row.PilgrimName))
	}
	return result, nil
}

// ListPilgrimHistory returns the pilgrim's own alerts, most recent first —
// used by the pilgrim app to poll for a status change (e.g. a leader
// acknowledging) rather than the coordinator/leader-side "active alerts"
// views above. pilgrimName isn't stored per-row here since the caller
// already knows which pilgrim this is (resolved from app_access_code).
func (r *SOSRepository) ListPilgrimHistory(ctx context.Context, operatorID, pilgrimID, pilgrimName string) ([]*domain.SOSAlert, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListSOSPilgrimHistory(ctx, db.ListSOSPilgrimHistoryParams{PilgrimID: pilgrimUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.SOSAlert, 0, len(rows))
	for _, row := range rows {
		result = append(result, toSOSAlert(row, pilgrimName))
	}
	return result, nil
}

func (r *SOSRepository) Acknowledge(ctx context.Context, operatorID, alertID, userID string) (*domain.SOSAlert, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	alertUUID, err := pgUUID(alertID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	alert, err := r.queries.AcknowledgeSOSAlert(ctx, db.AcknowledgeSOSAlertParams{ID: alertUUID, OperatorID: opUUID, AcknowledgedBy: pgText(userID), BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	return toSOSAlert(alert, ""), nil
}

func (r *SOSRepository) Resolve(ctx context.Context, operatorID, alertID, userID, notes string) (*domain.SOSAlert, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	alertUUID, err := pgUUID(alertID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	alert, err := r.queries.ResolveSOSAlert(ctx, db.ResolveSOSAlertParams{ID: alertUUID, OperatorID: opUUID, ResolvedBy: pgText(userID), Notes: notes, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	return toSOSAlert(alert, ""), nil
}

func toSOSAlert(alert db.SosAlert, pilgrimName string) *domain.SOSAlert {
	result := &domain.SOSAlert{ID: uuid.UUID(alert.ID.Bytes).String(), OperatorID: uuid.UUID(alert.OperatorID.Bytes).String(), PilgrimID: uuid.UUID(alert.PilgrimID.Bytes).String(), PilgrimName: pilgrimName, Status: alert.Status, Notes: alert.Notes, CreatedAt: alert.CreatedAt.Time}
	if alert.AcknowledgedBy.Valid {
		result.AcknowledgedBy = alert.AcknowledgedBy.String
	}
	if alert.AcknowledgedAt.Valid {
		t := alert.AcknowledgedAt.Time
		result.AcknowledgedAt = &t
	}
	if alert.ResolvedBy.Valid {
		result.ResolvedBy = alert.ResolvedBy.String
	}
	if alert.ResolvedAt.Valid {
		t := alert.ResolvedAt.Time
		result.ResolvedAt = &t
	}
	result.Lat = float8Ptr(alert.Lat)
	result.Lng = float8Ptr(alert.Lng)
	return result
}
