package repository

import (
	"context"

	db "github.com/hajj-saas/api/internal/gen/db"
)

type AuditRepository struct{ queries *db.Queries }

func NewAuditRepository(queries *db.Queries) *AuditRepository {
	return &AuditRepository{queries: queries}
}

// Write is fire-and-forget by convention at call sites — a failed audit
// write must never break the operation it's describing. Not transactional;
// use PilgrimRepository.WriteAuditLogTx when the write needs to be atomic
// with other changes in the same transaction.
func (r *AuditRepository) Write(ctx context.Context, operatorID, userID, action, entityType, entityID, message string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	entityUUID, err := pgUUID(entityID)
	if err != nil {
		return err
	}
	return r.queries.CreateAuditLog(ctx, db.CreateAuditLogParams{
		OperatorID: opUUID,
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityUUID,
		Column6:    message,
	})
}
