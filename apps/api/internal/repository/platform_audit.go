package repository

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
)

// AuditEntry is one line of the global trail.
type AuditEntry struct {
	ID         string
	At         time.Time
	Actor      string
	ActorID    string
	Action     string
	EntityType string
	EntityID   string
	OperatorID string
	Operator   string
	Message    string
}

// Audit categories. Named sets rather than free text, because these are the
// questions actually asked during an incident — "who impersonated a customer"
// and "what irreversible things were done" — and a search box makes somebody
// remember the exact spelling of an action under pressure.
const (
	AuditCategoryAll          = "ALL"
	AuditCategoryPrivileged   = "PRIVILEGED"
	AuditCategoryImpersonate  = "IMPERSONATION"
	AuditCategoryPersonalData = "PERSONAL_DATA"
)

var auditCategoryActions = map[string][]string{
	AuditCategoryPrivileged: {
		"tenant_suspended", "tenant_reinstated", "plan_limit_changed",
		"trial_days_changed", "grace_period_changed", "plan_override_set",
		"plan_override_deleted", "platform_admin_granted", "platform_admin_revoked",
		"sessions_revoked", "subscription_invoice_voided",
	},
	AuditCategoryImpersonate:  {"impersonation_started", "impersonation_ended"},
	AuditCategoryPersonalData: {"kyc_record_read", "kyc_status_set"},
}

type AuditFilter struct {
	OperatorID string
	Actor      string
	Category   string
	Since      *time.Time
	Limit      int32
}

// AuditTrail reads across every tenant. Read-only by construction: there is no
// update or delete anywhere near this file, and migration 125 already removed
// both privileges from the application role — so a delete button in the UI
// would be a button that always fails.
func (r *PlatformRepository) AuditTrail(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	if filter.Limit <= 0 || filter.Limit > 500 {
		filter.Limit = 100
	}
	conditions := []string{"TRUE"}
	args := []any{}
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, strings.ReplaceAll(condition, "$?", "$"+strconv.Itoa(len(args))))
	}

	if strings.TrimSpace(filter.OperatorID) != "" {
		operator, err := pgUUID(filter.OperatorID)
		if err != nil {
			return nil, apperror.ErrValidation
		}
		add("a.operator_id = $?", operator)
	}
	if actor := strings.TrimSpace(filter.Actor); actor != "" {
		// Matched against the email as well as the id, because an incident
		// starts from a person, and nobody remembers a Better Auth user id.
		// One argument used twice: add() replaces every $? in the condition with
		// the same placeholder, which is exactly what is wanted here.
		add("(a.user_id = $? OR u.email ILIKE '%' || $? || '%')", actor)
	}
	if actions, ok := auditCategoryActions[filter.Category]; ok {
		add("a.action = ANY($?)", actions)
	}
	if filter.Since != nil {
		add("a.created_at >= $?", *filter.Since)
	}
	args = append(args, filter.Limit)
	limitArg := "$" + strconv.Itoa(len(args))

	query := `
		SELECT a.id::text, a.created_at,
		       COALESCE(NULLIF(u.email, ''), a.user_id, 'sistem'), COALESCE(a.user_id, ''),
		       a.action, COALESCE(a.entity_type, ''), COALESCE(a.entity_id, ''),
		       COALESCE(a.operator_id::text, ''), COALESCE(o.name, ''),
		       COALESCE(a.metadata->>'message', '')
		FROM audit_logs a
		LEFT JOIN "user" u ON u.id = a.user_id
		LEFT JOIN operators o ON o.id = a.operator_id
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY a.created_at DESC
		LIMIT ` + limitArg

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	entries := make([]AuditEntry, 0)
	for rows.Next() {
		var entry AuditEntry
		if err := rows.Scan(&entry.ID, &entry.At, &entry.Actor, &entry.ActorID, &entry.Action,
			&entry.EntityType, &entry.EntityID, &entry.OperatorID, &entry.Operator, &entry.Message); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// StreamAuditTrail is AuditTrail without the LIMIT, for the auditor export
// (C4): an export exists specifically to answer "everything that matched,"
// so capping it the way the screen caps itself would silently produce an
// incomplete export with nothing on screen saying so. Reads row by row via
// the pool directly rather than buffering the whole trail into a slice —
// same reasoning as ProfitLossRepository.StreamExport, and the same
// requirement here: this trail is exactly the data an export must never
// truncate by running out of memory first.
func (r *PlatformRepository) StreamAuditTrail(ctx context.Context, filter AuditFilter, emit func(AuditEntry) error) error {
	conditions := []string{"TRUE"}
	args := []any{}
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, strings.ReplaceAll(condition, "$?", "$"+strconv.Itoa(len(args))))
	}

	if strings.TrimSpace(filter.OperatorID) != "" {
		operator, err := pgUUID(filter.OperatorID)
		if err != nil {
			return apperror.ErrValidation
		}
		add("a.operator_id = $?", operator)
	}
	if actor := strings.TrimSpace(filter.Actor); actor != "" {
		add("(a.user_id = $? OR u.email ILIKE '%' || $? || '%')", actor)
	}
	if actions, ok := auditCategoryActions[filter.Category]; ok {
		add("a.action = ANY($?)", actions)
	}
	if filter.Since != nil {
		add("a.created_at >= $?", *filter.Since)
	}

	query := `
		SELECT a.id::text, a.created_at,
		       COALESCE(NULLIF(u.email, ''), a.user_id, 'sistem'), COALESCE(a.user_id, ''),
		       a.action, COALESCE(a.entity_type, ''), COALESCE(a.entity_id, ''),
		       COALESCE(a.operator_id::text, ''), COALESCE(o.name, ''),
		       COALESCE(a.metadata->>'message', '')
		FROM audit_logs a
		LEFT JOIN "user" u ON u.id = a.user_id
		LEFT JOIN operators o ON o.id = a.operator_id
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY a.created_at ASC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return databaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry AuditEntry
		if err := rows.Scan(&entry.ID, &entry.At, &entry.Actor, &entry.ActorID, &entry.Action,
			&entry.EntityType, &entry.EntityID, &entry.OperatorID, &entry.Operator, &entry.Message); err != nil {
			return databaseError(err)
		}
		if err := emit(entry); err != nil {
			return err
		}
	}
	return rows.Err()
}
