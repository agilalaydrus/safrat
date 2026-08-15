package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

type SOSRepository struct{ queries *db.Queries }

func NewSOSRepository(queries *db.Queries) *SOSRepository {
	return &SOSRepository{queries: queries}
}

func (r *SOSRepository) Create(ctx context.Context, operatorID, pilgrimID string) (*domain.SOSAlert, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	pilgrimUUID, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, err
	}
	alert, err := r.queries.CreateSOSAlert(ctx, db.CreateSOSAlertParams{OperatorID: opUUID, PilgrimID: pilgrimUUID})
	if err != nil {
		return nil, err
	}
	return toSOSAlert(alert, ""), nil
}

func (r *SOSRepository) ListActive(ctx context.Context, operatorID string) ([]*domain.SOSAlert, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListActiveSOSAlerts(ctx, opUUID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.SOSAlert, 0, len(rows))
	for _, row := range rows {
		alert := db.SosAlert{ID: row.ID, OperatorID: row.OperatorID, PilgrimID: row.PilgrimID, Status: row.Status, AcknowledgedBy: row.AcknowledgedBy, AcknowledgedAt: row.AcknowledgedAt, ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt, Notes: row.Notes, CreatedAt: row.CreatedAt}
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
		alert := db.SosAlert{ID: row.ID, OperatorID: row.OperatorID, PilgrimID: row.PilgrimID, Status: row.Status, AcknowledgedBy: row.AcknowledgedBy, AcknowledgedAt: row.AcknowledgedAt, ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt, Notes: row.Notes, CreatedAt: row.CreatedAt}
		result = append(result, toSOSAlert(alert, row.PilgrimName))
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
	alert, err := r.queries.AcknowledgeSOSAlert(ctx, db.AcknowledgeSOSAlertParams{ID: alertUUID, OperatorID: opUUID, AcknowledgedBy: pgText(userID)})
	if err != nil {
		return nil, err
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
	alert, err := r.queries.ResolveSOSAlert(ctx, db.ResolveSOSAlertParams{ID: alertUUID, OperatorID: opUUID, ResolvedBy: pgText(userID), Notes: notes})
	if err != nil {
		return nil, err
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
	return result
}
