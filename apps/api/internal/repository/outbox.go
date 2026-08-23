package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// OutboxRepository is the transactional-outbox store (cascade_events). Producers
// call EnqueueTx inside their authoritative write's transaction; the worker
// relay drains it via Claim/MarkProcessed/MarkFailed.
type OutboxRepository struct {
	queries *db.Queries
}

func NewOutboxRepository(queries *db.Queries) *OutboxRepository {
	return &OutboxRepository{queries: queries}
}

func nullableUUID(value string) (pgtype.UUID, error) {
	if value == "" {
		return pgtype.UUID{}, nil // Valid=false -> SQL NULL
	}
	return pgUUID(value)
}

// EnqueueTx inserts an outbox event using the given transaction, so it commits
// atomically with the producer's authoritative write — the whole point of the
// pattern (no lost events on a crash between commit and a separate publish).
func (r *OutboxRepository) EnqueueTx(ctx context.Context, tx pgx.Tx, operatorID, eventType, entityID string, payload any) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	entUUID, err := nullableUUID(entityID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = r.queries.WithTx(tx).EnqueueCascadeEvent(ctx, db.EnqueueCascadeEventParams{
		OperatorID: opUUID,
		EventType:  eventType,
		EntityID:   entUUID,
		Payload:    raw,
	})
	return err
}

// Claim atomically grabs a batch of unprocessed events (skipping rows locked by
// a concurrent relay) and increments their attempt counter.
func (r *OutboxRepository) Claim(ctx context.Context, maxAttempts, limit int32) ([]domain.CascadeEvent, error) {
	rows, err := r.queries.ClaimCascadeEvents(ctx, db.ClaimCascadeEventsParams{Attempts: maxAttempts, Limit: limit})
	if err != nil {
		return nil, err
	}
	events := make([]domain.CascadeEvent, 0, len(rows))
	for _, row := range rows {
		entityID := ""
		if row.EntityID.Valid {
			entityID = uuid.UUID(row.EntityID.Bytes).String()
		}
		events = append(events, domain.CascadeEvent{
			ID:         row.ID,
			OperatorID: uuid.UUID(row.OperatorID.Bytes).String(),
			EventType:  row.EventType,
			EntityID:   entityID,
			Payload:    row.Payload,
			Attempts:   row.Attempts,
		})
	}
	return events, nil
}

func (r *OutboxRepository) MarkProcessed(ctx context.Context, id int64) error {
	return r.queries.MarkCascadeEventProcessed(ctx, id)
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	return r.queries.MarkCascadeEventFailed(ctx, db.MarkCascadeEventFailedParams{ID: id, LastError: errMsg})
}
