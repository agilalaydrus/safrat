package repository

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PersonalDataReadRepository struct {
	pool *pgxpool.Pool
}

func NewPersonalDataReadRepository(pool *pgxpool.Pool) *PersonalDataReadRepository {
	return &PersonalDataReadRepository{pool: pool}
}

type PersonalDataRead struct {
	ActorUserID     string
	ImpersonationID string
	OperatorID      string
	Procedure       string
}

// Record notes that somebody looked at personal data.
//
// Upsert rather than insert: one row per actor, per procedure, per tenant, per
// day, with a count. A row per request would be tens of thousands of rows
// nobody reads. ON CONFLICT is the whole mechanism — checking for an existing
// row and then updating it would let two concurrent reads both insert.
func (r *PersonalDataReadRepository) Record(ctx context.Context, read PersonalDataRead) error {
	if strings.TrimSpace(read.ActorUserID) == "" || strings.TrimSpace(read.Procedure) == "" {
		return nil
	}
	var impersonation, operator pgtype.UUID
	if strings.TrimSpace(read.ImpersonationID) != "" {
		parsed, err := pgUUID(read.ImpersonationID)
		if err != nil {
			return err
		}
		impersonation = parsed
	}
	if strings.TrimSpace(read.OperatorID) != "" {
		parsed, err := pgUUID(read.OperatorID)
		if err != nil {
			return err
		}
		operator = parsed
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO personal_data_reads (actor_user_id, impersonation_id, operator_id, procedure, day)
		VALUES ($1, $2, $3, $4, (NOW() AT TIME ZONE 'Asia/Jakarta')::date)
		ON CONFLICT (actor_user_id, procedure, day, impersonation_id, operator_id) DO UPDATE
		SET read_count = personal_data_reads.read_count + 1, last_at = NOW()`,
		read.ActorUserID, impersonation, operator, read.Procedure)
	return databaseError(err)
}

type PersonalDataReadRow struct {
	Actor      string
	Procedure  string
	Day        string
	ReadCount  int32
	LastAt     string
	InsideView bool
}

// ListForOperator answers the question a customer would ask: who at TawafiqHub
// has been looking at our people's data, and how much.
func (r *PersonalDataReadRepository) ListForOperator(ctx context.Context, operatorID string, limit int32) ([]PersonalDataReadRow, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(u.email, ''), p.actor_user_id), p.procedure,
		       to_char(p.day, 'YYYY-MM-DD'), p.read_count,
		       to_char(p.last_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD HH24:MI'),
		       p.impersonation_id IS NOT NULL
		FROM personal_data_reads p
		LEFT JOIN "user" u ON u.id = p.actor_user_id
		WHERE p.operator_id = $1
		ORDER BY p.day DESC, p.last_at DESC
		LIMIT $2`, operator, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	result := make([]PersonalDataReadRow, 0)
	for rows.Next() {
		var row PersonalDataReadRow
		if err := rows.Scan(&row.Actor, &row.Procedure, &row.Day, &row.ReadCount, &row.LastAt, &row.InsideView); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
