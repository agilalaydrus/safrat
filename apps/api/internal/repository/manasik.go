package repository

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type ManasikRepository struct {
	queries *db.Queries
}

func NewManasikRepository(queries *db.Queries) *ManasikRepository {
	return &ManasikRepository{queries: queries}
}

func toManasikCurriculum(row db.ManasikCurricula) *domain.ManasikCurriculum {
	return &domain.ManasikCurriculum{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
		Title: row.Title, Description: row.Description, SortOrder: row.SortOrder, CreatedAt: row.CreatedAt.Time,
	}
}

func (r *ManasikRepository) CreateCurriculum(ctx context.Context, operatorID, seasonID, title, description string, sortOrder int32) (*domain.ManasikCurriculum, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateManasikCurriculum(ctx, db.CreateManasikCurriculumParams{
		OperatorID: op, SeasonID: season, Title: title, Description: description, SortOrder: sortOrder,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toManasikCurriculum(row), nil
}

func (r *ManasikRepository) UpdateCurriculum(ctx context.Context, operatorID, id, title, description string, sortOrder int32) (*domain.ManasikCurriculum, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	curriculumID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpdateManasikCurriculum(ctx, db.UpdateManasikCurriculumParams{
		ID: curriculumID, OperatorID: op, Title: title, Description: description, SortOrder: sortOrder,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toManasikCurriculum(row), nil
}

func (r *ManasikRepository) DeleteCurriculum(ctx context.Context, operatorID, id string) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	curriculumID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	if err := r.queries.DeleteManasikCurriculum(ctx, db.DeleteManasikCurriculumParams{ID: curriculumID, OperatorID: op}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *ManasikRepository) ListCurricula(ctx context.Context, operatorID, seasonID string) ([]*domain.ManasikCurriculum, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListManasikCurricula(ctx, db.ListManasikCurriculaParams{OperatorID: op, SeasonID: season})
	if err != nil {
		return nil, databaseError(err)
	}
	curricula := make([]*domain.ManasikCurriculum, 0, len(rows))
	for _, row := range rows {
		curricula = append(curricula, toManasikCurriculum(row))
	}
	return curricula, nil
}

func toManasikSession(row db.ListManasikSessionsRow) *domain.ManasikSession {
	return &domain.ManasikSession{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
		CurriculumID: nullableUUIDString(row.CurriculumID), CurriculumTitle: row.CurriculumTitle,
		KloterID: nullableUUIDString(row.KloterID), KloterCode: row.KloterCode,
		Title: row.Title, Location: row.Location, InstructorName: row.InstructorName,
		ScheduledAt: row.ScheduledAt.Time, DurationMinutes: row.DurationMinutes, Capacity: row.Capacity,
		Notes: row.Notes, Status: row.Status, CreatedAt: row.CreatedAt.Time,
	}
}

func toManasikSessionSingle(row db.ManasikSession) *domain.ManasikSession {
	return &domain.ManasikSession{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
		CurriculumID: nullableUUIDString(row.CurriculumID), KloterID: nullableUUIDString(row.KloterID),
		Title: row.Title, Location: row.Location, InstructorName: row.InstructorName,
		ScheduledAt: row.ScheduledAt.Time, DurationMinutes: row.DurationMinutes, Capacity: row.Capacity,
		Notes: row.Notes, Status: row.Status, CreatedAt: row.CreatedAt.Time,
	}
}

func (r *ManasikRepository) CreateSession(ctx context.Context, operatorID, seasonID, curriculumID, kloterID, title, location, instructorName string, scheduledAt time.Time, durationMinutes, capacity int32, notes string) (*domain.ManasikSession, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateManasikSession(ctx, db.CreateManasikSessionParams{
		OperatorID: op, SeasonID: season, CurriculumID: pgUUIDOrNull(curriculumID), KloterID: pgUUIDOrNull(kloterID),
		Title: title, Location: location, InstructorName: instructorName,
		ScheduledAt: pgtype.Timestamptz{Time: scheduledAt, Valid: true}, DurationMinutes: durationMinutes,
		Capacity: capacity, Notes: notes,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toManasikSessionSingle(row), nil
}

func (r *ManasikRepository) UpdateSession(ctx context.Context, operatorID, id, curriculumID, kloterID, title, location, instructorName string, scheduledAt time.Time, durationMinutes, capacity int32, notes string) (*domain.ManasikSession, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	sessionID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpdateManasikSession(ctx, db.UpdateManasikSessionParams{
		ID: sessionID, OperatorID: op, CurriculumID: pgUUIDOrNull(curriculumID), KloterID: pgUUIDOrNull(kloterID),
		Title: title, Location: location, InstructorName: instructorName,
		ScheduledAt: pgtype.Timestamptz{Time: scheduledAt, Valid: true}, DurationMinutes: durationMinutes,
		Capacity: capacity, Notes: notes,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toManasikSessionSingle(row), nil
}

func (r *ManasikRepository) ListSessions(ctx context.Context, operatorID, seasonID string) ([]*domain.ManasikSession, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListManasikSessions(ctx, db.ListManasikSessionsParams{OperatorID: op, SeasonID: season})
	if err != nil {
		return nil, databaseError(err)
	}
	sessions := make([]*domain.ManasikSession, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, toManasikSession(row))
	}
	return sessions, nil
}

func (r *ManasikRepository) DeleteSession(ctx context.Context, operatorID, id string) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	sessionID, err := pgUUID(id)
	if err != nil {
		return apperror.ErrValidation
	}
	if err := r.queries.DeleteManasikSession(ctx, db.DeleteManasikSessionParams{ID: sessionID, OperatorID: op}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *ManasikRepository) UpdateSessionStatus(ctx context.Context, operatorID, id, status string) (*domain.ManasikSession, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	sessionID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpdateManasikSessionStatus(ctx, db.UpdateManasikSessionStatusParams{ID: sessionID, OperatorID: op, Status: status})
	if err != nil {
		return nil, databaseError(err)
	}
	return toManasikSessionSingle(row), nil
}

func toManasikAttendance(row db.ListManasikAttendanceRow) *domain.ManasikAttendance {
	return &domain.ManasikAttendance{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SessionID: uuidString(row.SessionID),
		PilgrimID: uuidString(row.PilgrimID), PilgrimName: row.PilgrimName, PassportNumber: row.PassportNumber,
		Status: row.Status, Notes: row.Notes, RecordedAt: row.RecordedAt.Time,
	}
}

// RecordAttendance upserts one pilgrim's mark for one session — a roll-call
// is taken one name at a time from a list, so each call is one name, but a
// re-submission (correcting a mis-mark) settles on the same row rather than
// duplicating it.
func (r *ManasikRepository) RecordAttendance(ctx context.Context, operatorID, sessionID, pilgrimID, status, notes string) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	session, err := pgUUID(sessionID)
	if err != nil {
		return apperror.ErrValidation
	}
	pilgrim, err := pgUUID(pilgrimID)
	if err != nil {
		return apperror.ErrValidation
	}
	if _, err := r.queries.UpsertManasikAttendance(ctx, db.UpsertManasikAttendanceParams{
		OperatorID: op, SessionID: session, PilgrimID: pilgrim, Status: status, Notes: notes,
	}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *ManasikRepository) ListAttendance(ctx context.Context, operatorID, sessionID string) ([]*domain.ManasikAttendance, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	session, err := pgUUID(sessionID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListManasikAttendance(ctx, db.ListManasikAttendanceParams{OperatorID: op, SessionID: session})
	if err != nil {
		return nil, databaseError(err)
	}
	attendance := make([]*domain.ManasikAttendance, 0, len(rows))
	for _, row := range rows {
		attendance = append(attendance, toManasikAttendance(row))
	}
	return attendance, nil
}

func (r *ManasikRepository) AttendanceSummary(ctx context.Context, operatorID, sessionID string) (*domain.ManasikAttendanceSummary, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	session, err := pgUUID(sessionID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.ManasikAttendanceSummary(ctx, db.ManasikAttendanceSummaryParams{OperatorID: op, SessionID: session})
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.ManasikAttendanceSummary{PresentCount: row.PresentCount, AbsentCount: row.AbsentCount, ExcusedCount: row.ExcusedCount}, nil
}
