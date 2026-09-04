package repository

import (
	"context"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
)

// ItinerarySegment is one leg of Rangkaian — Transport or Hotel — with just
// enough of the referenced movement/hotel joined in for the screen to show
// it without a second round trip.
type ItinerarySegment struct {
	ID                  string
	Position            int32
	SegmentType         string
	MovementID          string
	MovementName        string
	MovementMode        string
	MovementScheduledAt *time.Time
	HotelID             string
	HotelName           string
	HotelCity           string
	Notes               string
}

// ItinerarySegmentInput is what the client sends when replacing the whole
// Rangkaian in one call.
type ItinerarySegmentInput struct {
	SegmentType string
	MovementID  string
	HotelID     string
	Notes       string
}

func validSegmentType(t string) bool { return t == "TRANSPORT" || t == "HOTEL" }

// ListItinerarySegments returns Rangkaian in order for one kloter, scoped to
// the operator that owns it.
func (r *KloterRepository) ListItinerarySegments(ctx context.Context, operatorID, kloterID string) ([]ItinerarySegment, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	kloter, err := pgUUID(kloterID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.position, s.segment_type, s.notes,
		       COALESCE(s.movement_id::text, ''), COALESCE(m.name, ''), COALESCE(m.mode, ''), m.scheduled_at,
		       COALESCE(s.hotel_id::text, ''), COALESCE(h.name, ''), COALESCE(h.city, '')
		FROM kloter_itinerary_segments s
		LEFT JOIN movements m ON m.id = s.movement_id
		LEFT JOIN hotels h ON h.id = s.hotel_id
		WHERE s.kloter_id = $1 AND s.operator_id = $2
		ORDER BY s.position`, kloter, operator)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	segments := make([]ItinerarySegment, 0, 8)
	for rows.Next() {
		var seg ItinerarySegment
		if err := rows.Scan(&seg.ID, &seg.Position, &seg.SegmentType, &seg.Notes,
			&seg.MovementID, &seg.MovementName, &seg.MovementMode, &seg.MovementScheduledAt,
			&seg.HotelID, &seg.HotelName, &seg.HotelCity); err != nil {
			return nil, err
		}
		segments = append(segments, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return segments, nil
}

// SetItinerarySegments replaces the whole Rangkaian for one kloter.
//
// Replace rather than edit-one-at-a-time for the same reason room tiers are:
// the sequence is one decision, and saving it leg-by-leg would let a screen
// read a spine that starts or ends on a Hotel while the edit is half-applied.
func (r *KloterRepository) SetItinerarySegments(ctx context.Context, operatorID, kloterID string, segments []ItinerarySegmentInput) ([]ItinerarySegment, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	kloter, err := pgUUID(kloterID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if len(segments) > 0 {
		if segments[0].SegmentType != "TRANSPORT" || segments[len(segments)-1].SegmentType != "TRANSPORT" {
			return nil, apperror.ErrValidation
		}
	}
	for _, seg := range segments {
		if !validSegmentType(seg.SegmentType) {
			return nil, apperror.ErrValidation
		}
		if seg.SegmentType == "TRANSPORT" && seg.MovementID == "" {
			return nil, apperror.ErrValidation
		}
		if seg.SegmentType == "HOTEL" && seg.HotelID == "" {
			return nil, apperror.ErrValidation
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM kloters WHERE id = $1 AND operator_id = $2)`,
		kloter, operator).Scan(&exists); err != nil {
		return nil, databaseError(err)
	}
	if !exists {
		return nil, apperror.ErrNotFound
	}

	// A movement or hotel from another kloter/operator would let one
	// operator's Rangkaian point at another operator's transport, or at a
	// vehicle nobody in this kloter is actually taking.
	for _, seg := range segments {
		if seg.SegmentType == "TRANSPORT" {
			movementID, err := pgUUID(seg.MovementID)
			if err != nil {
				return nil, apperror.ErrValidation
			}
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM movements WHERE id = $1 AND operator_id = $2 AND kloter_id = $3)`,
				movementID, operator, kloter).Scan(&ok); err != nil {
				return nil, databaseError(err)
			}
			if !ok {
				return nil, apperror.ErrValidation
			}
		} else {
			hotelID, err := pgUUID(seg.HotelID)
			if err != nil {
				return nil, apperror.ErrValidation
			}
			var ok bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS(
					SELECT 1 FROM hotels h JOIN kloters k ON k.season_id = h.season_id
					WHERE h.id = $1 AND h.operator_id = $2 AND k.id = $3
				)`, hotelID, operator, kloter).Scan(&ok); err != nil {
				return nil, databaseError(err)
			}
			if !ok {
				return nil, apperror.ErrValidation
			}
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM kloter_itinerary_segments WHERE kloter_id = $1 AND operator_id = $2`,
		kloter, operator); err != nil {
		return nil, databaseError(err)
	}
	for i, seg := range segments {
		if _, err := tx.Exec(ctx, `
			INSERT INTO kloter_itinerary_segments (kloter_id, operator_id, position, segment_type, movement_id, hotel_id, notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			kloter, operator, i, seg.SegmentType, pgUUIDOrNull(seg.MovementID), pgUUIDOrNull(seg.HotelID), seg.Notes); err != nil {
			return nil, databaseError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, databaseError(err)
	}
	return r.ListItinerarySegments(ctx, operatorID, kloterID)
}

// RundownItem is one row of the day-by-day operational schedule.
type RundownItem struct {
	ID        string
	DayNumber int32
	Position  int32
	TimeLabel string
	Title     string
	Location  string
	PIC       string
	Notes     string
}

// ListRundownItems returns the rundown in day/position order for one kloter.
func (r *KloterRepository) ListRundownItems(ctx context.Context, operatorID, kloterID string) ([]RundownItem, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	kloter, err := pgUUID(kloterID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, day_number, position, time_label, title, location, pic, notes
		FROM kloter_rundown_items
		WHERE kloter_id = $1 AND operator_id = $2
		ORDER BY day_number, position`, kloter, operator)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	items := make([]RundownItem, 0, 16)
	for rows.Next() {
		var item RundownItem
		if err := rows.Scan(&item.ID, &item.DayNumber, &item.Position, &item.TimeLabel, &item.Title, &item.Location, &item.PIC, &item.Notes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// SetRundownItems replaces the whole rundown for one kloter. Position within
// a day is assigned from input order, same reasoning as Rangkaian: the
// schedule for a day is one decision, not independent inserts a reader could
// catch half-saved.
func (r *KloterRepository) SetRundownItems(ctx context.Context, operatorID, kloterID string, items []RundownItem) ([]RundownItem, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	kloter, err := pgUUID(kloterID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	for _, item := range items {
		if item.DayNumber < 1 {
			return nil, apperror.ErrValidation
		}
		if item.Title == "" {
			return nil, apperror.ErrValidation
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM kloters WHERE id = $1 AND operator_id = $2)`,
		kloter, operator).Scan(&exists); err != nil {
		return nil, databaseError(err)
	}
	if !exists {
		return nil, apperror.ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM kloter_rundown_items WHERE kloter_id = $1 AND operator_id = $2`,
		kloter, operator); err != nil {
		return nil, databaseError(err)
	}
	positionByDay := map[int32]int32{}
	for _, item := range items {
		position := positionByDay[item.DayNumber]
		positionByDay[item.DayNumber] = position + 1
		if _, err := tx.Exec(ctx, `
			INSERT INTO kloter_rundown_items (kloter_id, operator_id, day_number, position, time_label, title, location, pic, notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			kloter, operator, item.DayNumber, position, item.TimeLabel, item.Title, item.Location, item.PIC, item.Notes); err != nil {
			return nil, databaseError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, databaseError(err)
	}
	return r.ListRundownItems(ctx, operatorID, kloterID)
}
