package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
)

type RoomlistOccupant struct {
	PilgrimID string
	FullName  string
	Gender    string
	MahramID  string
}

type RoomlistRoom struct {
	RoomID           string
	RoomNumber       string
	RoomType         string
	DesignatedGender string
	Capacity         int32
	Occupants        []RoomlistOccupant
}

type RoomlistHotel struct {
	HotelID      string
	Name         string
	City         string
	CheckInDate  *time.Time
	CheckOutDate *time.Time
	Rooms        []RoomlistRoom
}

type Roomlist struct {
	KloterCode string
	Hotels     []RoomlistHotel
	Unassigned []RoomlistOccupant
	Total      int32
}

// Roomlist is the sheet a hotel receives, grouped the way a hotel reads it:
// city, then property, then room.
//
// Built from two queries rather than one join with grouping done in SQL. The
// second reads pilgrims with no bed at all — a left join from allocations
// cannot produce those rows, and they are the most urgent line on the screen.
func (r *KloterRepository) Roomlist(ctx context.Context, operatorID, kloterID string) (Roomlist, error) {
	list := Roomlist{}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return list, apperror.ErrValidation
	}
	kloter, err := pgUUID(kloterID)
	if err != nil {
		return list, apperror.ErrValidation
	}

	err = r.pool.QueryRow(ctx, `SELECT code FROM kloters WHERE id = $1 AND operator_id = $2`,
		kloter, operator).Scan(&list.KloterCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return list, apperror.ErrNotFound
	}
	if err != nil {
		return list, databaseError(err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT h.id::text, h.name, COALESCE(h.city, ''), h.check_in_date, h.check_out_date,
		       rm.id::text, rm.room_number, COALESCE(rm.room_type, ''), COALESCE(rm.gender, ''), rm.capacity,
		       p.id::text, p.full_name, COALESCE(p.gender, ''), COALESCE(p.mahram_id::text, '')
		FROM room_allocations ra
		JOIN rooms rm ON rm.id = ra.room_id
		JOIN hotels h ON h.id = rm.hotel_id
		JOIN pilgrims p ON p.id = ra.pilgrim_id
		WHERE ra.operator_id = $1 AND p.kloter_id = $2 AND p.is_substituted = false
		ORDER BY h.city, h.name, rm.room_number, p.full_name`, operator, kloter)
	if err != nil {
		return list, databaseError(err)
	}
	defer rows.Close()

	// Grouped in Go rather than with array_agg. The nesting is three deep and
	// the alternative is a query nobody can change safely later.
	hotelIndex := map[string]int{}
	roomIndex := map[string]int{}
	for rows.Next() {
		var hotelID, hotelName, city, roomID, roomNumber, roomType, roomGender string
		var checkIn, checkOut *time.Time
		var capacity int32
		var occupant RoomlistOccupant
		if err := rows.Scan(&hotelID, &hotelName, &city, &checkIn, &checkOut,
			&roomID, &roomNumber, &roomType, &roomGender, &capacity,
			&occupant.PilgrimID, &occupant.FullName, &occupant.Gender, &occupant.MahramID); err != nil {
			return list, err
		}
		hotelAt, ok := hotelIndex[hotelID]
		if !ok {
			list.Hotels = append(list.Hotels, RoomlistHotel{
				HotelID: hotelID, Name: hotelName, City: city,
				CheckInDate: checkIn, CheckOutDate: checkOut,
			})
			hotelAt = len(list.Hotels) - 1
			hotelIndex[hotelID] = hotelAt
		}
		roomAt, ok := roomIndex[roomID]
		if !ok {
			list.Hotels[hotelAt].Rooms = append(list.Hotels[hotelAt].Rooms, RoomlistRoom{
				RoomID: roomID, RoomNumber: roomNumber, RoomType: roomType,
				DesignatedGender: roomGender, Capacity: capacity,
			})
			roomAt = len(list.Hotels[hotelAt].Rooms) - 1
			roomIndex[roomID] = roomAt
		}
		list.Hotels[hotelAt].Rooms[roomAt].Occupants = append(
			list.Hotels[hotelAt].Rooms[roomAt].Occupants, occupant)
	}
	if err := rows.Err(); err != nil {
		return list, err
	}

	unassigned, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.full_name, COALESCE(p.gender, ''), COALESCE(p.mahram_id::text, '')
		FROM pilgrims p
		WHERE p.kloter_id = $1 AND p.operator_id = $2 AND p.is_substituted = false
		  AND NOT EXISTS (SELECT 1 FROM room_allocations ra WHERE ra.pilgrim_id = p.id)
		ORDER BY p.full_name`, kloter, operator)
	if err != nil {
		return list, databaseError(err)
	}
	defer unassigned.Close()
	for unassigned.Next() {
		var occupant RoomlistOccupant
		if err := unassigned.Scan(&occupant.PilgrimID, &occupant.FullName,
			&occupant.Gender, &occupant.MahramID); err != nil {
			return list, err
		}
		list.Unassigned = append(list.Unassigned, occupant)
	}
	if err := unassigned.Err(); err != nil {
		return list, err
	}

	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM pilgrims
		WHERE kloter_id = $1 AND operator_id = $2 AND is_substituted = false`,
		kloter, operator).Scan(&list.Total); err != nil {
		return list, databaseError(err)
	}
	return list, nil
}

// MixedWithoutMahram reports a family room holding both men and women where at
// least one of them has no mahram in the room with them.
//
// Only family rooms can reach this state: the allocation rules already refuse
// to put a man in a room designated female, and vice versa. A family room
// accepts anyone by design, which is what makes it the case worth checking —
// and it is a question to look at, not always an error, so it is surfaced
// rather than blocked.
func (room RoomlistRoom) MixedWithoutMahram() bool {
	if room.DesignatedGender != "family" || len(room.Occupants) < 2 {
		return false
	}
	present := map[string]bool{}
	genders := map[string]bool{}
	for _, occupant := range room.Occupants {
		present[occupant.PilgrimID] = true
		if occupant.Gender != "" {
			genders[occupant.Gender] = true
		}
	}
	if len(genders) < 2 {
		return false
	}
	for _, occupant := range room.Occupants {
		// Either this person's own mahram is here, or somebody here has named
		// them as theirs. The link is recorded on one side only.
		if occupant.MahramID != "" && present[occupant.MahramID] {
			continue
		}
		named := false
		for _, other := range room.Occupants {
			if other.MahramID == occupant.PilgrimID {
				named = true
				break
			}
		}
		if !named {
			return true
		}
	}
	return false
}
