package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
)

// ManifestRow is one line of the list that leaves the office.
type ManifestRow struct {
	PilgrimID           string
	FullName            string
	PassportNumber      string
	Gender              string
	DateOfBirth         *time.Time
	GroupName           string
	RoomLabel           string
	Phone               string
	HasPassport         bool
	HasVaccine          bool
	HasPhoto            bool
	HasKTP              bool
	HasKK               bool
	HasMahramProof      bool
	MahramProofRequired bool
}

// requiredDocuments is the ladder the screen and the counts both read, so the
// list of what is required exists once.
//
// Marriage or kinship proof is deliberately conditional: it is asked only of
// somebody travelling under a mahram. Requiring it of everybody would mark most
// of a manifest incomplete for a document they were never asked to bring, and a
// readiness figure that is wrong for half the roster is one nobody trusts.
func (row ManifestRow) MissingDocuments() []string {
	missing := make([]string, 0, 6)
	if !row.HasPassport {
		missing = append(missing, "PASPOR")
	}
	if !row.HasVaccine {
		missing = append(missing, "VAKSIN")
	}
	if !row.HasPhoto {
		missing = append(missing, "FOTO")
	}
	if !row.HasKTP {
		missing = append(missing, "KTP")
	}
	if !row.HasKK {
		missing = append(missing, "KK")
	}
	if row.MahramProofRequired && !row.HasMahramProof {
		missing = append(missing, "BUKU_NIKAH")
	}
	return missing
}

type Manifest struct {
	KloterID      string
	Code          string
	FlightNumber  string
	Embarkation   string
	DepartureDate *time.Time
	Capacity      int32
	Rows          []ManifestRow
}

// Manifest returns every pilgrim on one kloter with what each is still missing.
//
// Substituted pilgrims are excluded. They are not travelling, and counting them
// would make a full flight look short and a ready manifest look incomplete —
// the same rule the entitlement counts already use.
func (r *KloterRepository) Manifest(ctx context.Context, operatorID, kloterID string) (Manifest, error) {
	manifest := Manifest{}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return manifest, apperror.ErrValidation
	}
	kloter, err := pgUUID(kloterID)
	if err != nil {
		return manifest, apperror.ErrValidation
	}

	err = r.pool.QueryRow(ctx, `
		SELECT id::text, code, COALESCE(flight_number, ''), COALESCE(embarkation, ''),
		       departure_date, COALESCE(capacity, 0)
		FROM kloters WHERE id = $1 AND operator_id = $2`, kloter, operator).
		Scan(&manifest.KloterID, &manifest.Code, &manifest.FlightNumber,
			&manifest.Embarkation, &manifest.DepartureDate, &manifest.Capacity)
	if errors.Is(err, pgx.ErrNoRows) {
		return manifest, apperror.ErrNotFound
	}
	if err != nil {
		return manifest, databaseError(err)
	}

	// The room is joined through the allocation rather than stored on the
	// pilgrim, so a substitution that moves somebody between rooms shows the
	// room they are actually in. A pilgrim with no allocation yet is an
	// ordinary state, not a missing row — hence the left joins.
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.full_name, COALESCE(p.passport_number, ''), COALESCE(p.gender, ''),
		       p.date_of_birth, COALESCE(g.name, ''), COALESCE(p.phone, ''),
		       COALESCE(h.name || ' ' || rm.room_number, ''),
		       p.documents_passport, p.documents_vaccine, p.documents_photo,
		       p.documents_ktp, p.documents_kk, p.documents_mahram_proof,
		       p.mahram_id IS NOT NULL
		FROM pilgrims p
		LEFT JOIN groups g ON g.id = p.group_id
		LEFT JOIN room_allocations ra ON ra.pilgrim_id = p.id
		LEFT JOIN rooms rm ON rm.id = ra.room_id
		LEFT JOIN hotels h ON h.id = rm.hotel_id
		WHERE p.kloter_id = $1 AND p.operator_id = $2 AND p.is_substituted = false
		ORDER BY g.name NULLS LAST, p.full_name`, kloter, operator)
	if err != nil {
		return manifest, databaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var row ManifestRow
		if err := rows.Scan(&row.PilgrimID, &row.FullName, &row.PassportNumber, &row.Gender,
			&row.DateOfBirth, &row.GroupName, &row.Phone, &row.RoomLabel,
			&row.HasPassport, &row.HasVaccine, &row.HasPhoto,
			&row.HasKTP, &row.HasKK, &row.HasMahramProof, &row.MahramProofRequired); err != nil {
			return manifest, err
		}
		manifest.Rows = append(manifest.Rows, row)
	}
	return manifest, rows.Err()
}
