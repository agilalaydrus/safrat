package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"

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
// Write records who did what.
//
// An empty operatorID means a platform-level action, which belongs to no
// tenant and is stored with a null operator rather than being attributed to
// whichever travel happened to be involved — that would put platform actions
// in a customer's trail where they do not belong.
//
// entityID is text, not a UUID: granting somebody platform access is an action
// about a Better Auth account id, which is not one. Encoding it into a fake
// UUID to satisfy a column would make the trail unreadable.
func (r *AuditRepository) Write(ctx context.Context, operatorID, userID, action, entityType, entityID, message string) error {
	var opUUID pgtype.UUID
	if operatorID != "" {
		parsed, err := pgUUID(operatorID)
		if err != nil {
			return err
		}
		opUUID = parsed
	}
	return r.queries.CreateAuditLog(ctx, db.CreateAuditLogParams{
		OperatorID: opUUID,
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Column6:    message,
	})
}
