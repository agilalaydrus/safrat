package repository

import (
	"context"
	"errors"
	"time"

	"fmt"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// KYCRepository is the one place an identity lives.
//
// Identity used to be columns on whichever role record happened to hold it,
// which made "whose identity is this" a question with no single answer. Here it
// is a record of its own, naming the account it belongs to, so it can be
// produced on request rather than reconstructed.
type KYCRepository struct {
	pool *pgxpool.Pool
}

func NewKYCRepository(pool *pgxpool.Pool) *KYCRepository {
	return &KYCRepository{pool: pool}
}

// KYCRecord is one person's identity as this system holds it.
//
// NIK and NPWP are plaintext in this struct and encrypted in the database. The
// boundary is the repository: nothing above it handles ciphertext, and nothing
// below it sees the numbers.
type KYCRecord struct {
	ID              string
	OperatorID      string
	OperatorName    string
	UserID          string
	SubjectType     string
	SubjectID       string
	FullName        string
	NIK             string
	NPWP            string
	Address         string
	PlaceOfBirth    string
	DateOfBirth     *time.Time
	Status          string
	Source          string
	VerifiedBy      string
	VerifiedAt      *time.Time
	RejectionReason string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Save records an identity, replacing whatever was there for the same subject.
//
// Submitting again resets the record to pending review and clears any previous
// verification, because a verification applies to what was checked — not to
// whatever the person later replaced it with.
func (r *KYCRepository) Save(ctx context.Context, record KYCRecord) (string, error) {
	operatorID, err := pgUUID(record.OperatorID)
	if err != nil {
		return "", apperror.ErrValidation
	}
	subjectID, err := pgUUID(record.SubjectID)
	if err != nil {
		return "", apperror.ErrValidation
	}
	sealedNIK, err := sealKYC(record.NIK)
	if err != nil {
		return "", err
	}
	sealedNPWP, err := sealKYC(record.NPWP)
	if err != nil {
		return "", err
	}
	var userID any
	if record.UserID != "" {
		userID = record.UserID
	}
	source := record.Source
	if source == "" {
		source = "SELF"
	}

	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO kyc_records
			(operator_id, user_id, subject_type, subject_id, full_name,
			 nik_encrypted, npwp_encrypted, address, place_of_birth, date_of_birth,
			 status, source, key_fingerprint)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'PENDING_REVIEW', $11, $12)
		ON CONFLICT (subject_type, subject_id) DO UPDATE SET
			user_id = COALESCE(EXCLUDED.user_id, kyc_records.user_id),
			full_name = EXCLUDED.full_name,
			nik_encrypted = EXCLUDED.nik_encrypted,
			npwp_encrypted = EXCLUDED.npwp_encrypted,
			address = EXCLUDED.address,
			place_of_birth = EXCLUDED.place_of_birth,
			date_of_birth = EXCLUDED.date_of_birth,
			source = EXCLUDED.source,
			key_fingerprint = EXCLUDED.key_fingerprint,
			status = 'PENDING_REVIEW',
			verified_by = '', verified_at = NULL, rejection_reason = '',
			updated_at = NOW()
		RETURNING id::text`,
		operatorID, userID, record.SubjectType, subjectID, record.FullName,
		sealedNIK, sealedNPWP, record.Address, record.PlaceOfBirth, pgDate(record.DateOfBirth),
		source, KYCKeyFingerprint()).Scan(&id)
	return id, err
}

// ForSubject reads the identity collected against one role record.
func (r *KYCRepository) ForSubject(ctx context.Context, subjectType, subjectID string) (*KYCRecord, error) {
	id, err := pgUUID(subjectID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	return r.scanOne(r.pool.QueryRow(ctx, kycSelect+`
		WHERE k.subject_type = $1 AND k.subject_id = $2`, subjectType, id))
}

// ForUser reads every identity attached to one account.
//
// A person can hold more than one — an agent who is also a registered jamaah —
// and collapsing them would hide exactly the connection somebody asking is
// trying to see.
func (r *KYCRepository) ForUser(ctx context.Context, userID string) ([]*KYCRecord, error) {
	if userID == "" {
		return nil, apperror.ErrValidation
	}
	rows, err := r.pool.Query(ctx, kycSelect+` WHERE k.user_id = $1 ORDER BY k.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// List returns identity records for review, newest first.
func (r *KYCRepository) List(ctx context.Context, status string, limit int32) ([]*KYCRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, kycSelect+`
		WHERE $1 = '' OR k.status = $1
		ORDER BY k.created_at DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// SetStatus records a verification decision and who made it.
func (r *KYCRepository) SetStatus(ctx context.Context, recordID, status, decidedBy, reason string) error {
	id, err := pgUUID(recordID)
	if err != nil {
		return apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE kyc_records
		SET status = $2, verified_by = $3,
		    verified_at = CASE WHEN $2 = 'VERIFIED' THEN NOW() ELSE NULL END,
		    rejection_reason = CASE WHEN $2 = 'REJECTED' THEN $4 ELSE '' END,
		    updated_at = NOW()
		WHERE id = $1`, id, status, decidedBy, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

const kycSelect = `
	SELECT k.id::text, k.operator_id::text, o.name, COALESCE(k.user_id, ''),
	       k.subject_type, k.subject_id::text, k.full_name,
	       k.nik_encrypted, k.npwp_encrypted, k.address, k.place_of_birth, k.date_of_birth,
	       k.status, k.source, k.verified_by, k.verified_at, k.rejection_reason,
	       k.created_at, k.updated_at
	FROM kyc_records k
	JOIN operators o ON o.id = k.operator_id`

func (r *KYCRepository) scanOne(row pgx.Row) (*KYCRecord, error) {
	var record KYCRecord
	var sealedNIK, sealedNPWP string
	err := row.Scan(&record.ID, &record.OperatorID, &record.OperatorName, &record.UserID,
		&record.SubjectType, &record.SubjectID, &record.FullName,
		&sealedNIK, &sealedNPWP, &record.Address, &record.PlaceOfBirth, &record.DateOfBirth,
		&record.Status, &record.Source, &record.VerifiedBy, &record.VerifiedAt,
		&record.RejectionReason, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Decrypted here and nowhere else: the ciphertext never leaves this file.
	record.NIK = openKYC(sealedNIK)
	record.NPWP = openKYC(sealedNPWP)
	return &record, nil
}

func (r *KYCRepository) scanMany(rows pgx.Rows) ([]*KYCRecord, error) {
	records := make([]*KYCRecord, 0)
	for rows.Next() {
		record, err := r.scanOne(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// MigrateLegacyIdentities moves identity numbers out of the old columns and
// into encrypted records, and reports how many it moved.
//
// One transaction per record: the number is written to its new home and cleared
// from its old one together, so a row is never left holding it twice or not at
// all. Safe to run repeatedly — it only ever looks at rows that still have a
// legacy value.
func (r *KYCRepository) MigrateLegacyIdentities(ctx context.Context, limit int32) (int, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	type legacy struct {
		subjectType, subjectID, operatorID, userID, fullName, nik, npwp, address string
	}
	rows, err := r.pool.Query(ctx, `
		SELECT 'AGENT', a.id::text, a.operator_id::text, COALESCE(a.linked_user_id, ''),
		       a.name, a.nik, a.npwp, a.address
		FROM agents a WHERE a.nik <> '' OR a.npwp <> ''
		UNION ALL
		SELECT 'PILGRIM', p.id::text, p.operator_id::text, COALESCE(p.linked_user_id, ''),
		       p.full_name, p.nik, '', p.address
		FROM pilgrims p WHERE p.nik <> ''
		LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	pending := make([]legacy, 0)
	for rows.Next() {
		var item legacy
		if err := rows.Scan(&item.subjectType, &item.subjectID, &item.operatorID, &item.userID,
			&item.fullName, &item.nik, &item.npwp, &item.address); err != nil {
			rows.Close()
			return 0, err
		}
		pending = append(pending, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	moved := 0
	for _, item := range pending {
		// Encrypted before the transaction opens: a sealing failure must not
		// leave a transaction hanging, and without a key nothing should move at
		// all rather than moving in the clear.
		sealedNIK, err := sealKYC(item.nik)
		if err != nil {
			return moved, err
		}
		sealedNPWP, err := sealKYC(item.npwp)
		if err != nil {
			return moved, err
		}
		operatorID, err := pgUUID(item.operatorID)
		if err != nil {
			continue
		}
		subjectID, err := pgUUID(item.subjectID)
		if err != nil {
			continue
		}
		var userID any
		if item.userID != "" {
			userID = item.userID
		}

		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return moved, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO kyc_records
				(operator_id, user_id, subject_type, subject_id, full_name,
				 nik_encrypted, npwp_encrypted, address, source, key_fingerprint)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'STAFF', $9)
			ON CONFLICT (subject_type, subject_id) DO UPDATE SET
				nik_encrypted = CASE WHEN kyc_records.nik_encrypted = '' THEN EXCLUDED.nik_encrypted ELSE kyc_records.nik_encrypted END,
				npwp_encrypted = CASE WHEN kyc_records.npwp_encrypted = '' THEN EXCLUDED.npwp_encrypted ELSE kyc_records.npwp_encrypted END,
				updated_at = NOW()`,
			operatorID, userID, item.subjectType, subjectID, item.fullName,
			sealedNIK, sealedNPWP, item.address, KYCKeyFingerprint()); err != nil {
			_ = tx.Rollback(ctx)
			return moved, err
		}
		clear := `UPDATE agents SET nik = '', npwp = '' WHERE id = $1`
		if item.subjectType == "PILGRIM" {
			clear = `UPDATE pilgrims SET nik = '' WHERE id = $1`
		}
		if _, err := tx.Exec(ctx, clear, subjectID); err != nil {
			_ = tx.Rollback(ctx)
			return moved, err
		}
		if err := tx.Commit(ctx); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

// KeyFingerprintsInUse reports which keys sealed the records currently stored,
// and how many each accounts for.
//
// The question somebody asks after losing a key: "what am I looking for". A
// fingerprint is not the key and cannot become one, but it identifies the key
// that would open these rows — so a candidate found in a password manager can
// be checked against it before being deployed, rather than after.
//
// It also makes a rotation legible: while one is in progress two fingerprints
// appear, and the count of the old one falling to zero is what "finished"
// means.
func (r *KYCRepository) KeyFingerprintsInUse(ctx context.Context) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT key_fingerprint, COUNT(*)
		FROM kyc_records
		WHERE nik_encrypted <> '' OR npwp_encrypted <> ''
		GROUP BY key_fingerprint ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]int)
	for rows.Next() {
		var fingerprint string
		var count int
		if err := rows.Scan(&fingerprint, &count); err != nil {
			return nil, err
		}
		counts[fingerprint] = count
	}
	return counts, rows.Err()
}

// RotateKey re-seals records from one key to another, a batch at a time, and
// reports how many it moved.
//
// Resumable by construction: it only ever selects rows still stamped with the
// old key, so stopping and restarting continues where it left off, and running
// it after it has finished does nothing. That matters because a rotation over
// a real dataset is not one command — it is a command run repeatedly until the
// old fingerprint's count reaches zero.
//
// Each record is re-sealed and re-stamped in a single UPDATE. A row can
// therefore never carry a fingerprint that does not match the value beside it,
// which is the property everything else — the startup check, the mismatch
// warning, the progress count — relies on being true.
//
// A value the old key cannot open stops the batch rather than being skipped.
// Skipping it would leave a row nothing can read and nothing is looking for.
func (r *KYCRepository) RotateKey(ctx context.Context, rotator *crypto.Rotator, limit int32) (int, error) {
	if rotator == nil {
		return 0, apperror.ErrValidation
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, nik_encrypted, npwp_encrypted
		FROM kyc_records
		WHERE key_fingerprint = $1 AND (nik_encrypted <> '' OR npwp_encrypted <> '')
		ORDER BY created_at ASC
		LIMIT $2`, rotator.FromFingerprint(), limit)
	if err != nil {
		return 0, err
	}
	type pending struct{ id, nik, npwp string }
	batch := make([]pending, 0)
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.id, &item.nik, &item.npwp); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	rotated := 0
	for _, item := range batch {
		nik, err := rotator.Reseal(item.nik)
		if err != nil {
			return rotated, fmt.Errorf("record %s: %w", item.id, err)
		}
		npwp, err := rotator.Reseal(item.npwp)
		if err != nil {
			return rotated, fmt.Errorf("record %s: %w", item.id, err)
		}
		id, err := pgUUID(item.id)
		if err != nil {
			return rotated, err
		}
		// The value and the stamp move together, or neither does.
		if _, err := r.pool.Exec(ctx, `
			UPDATE kyc_records
			SET nik_encrypted = $2, npwp_encrypted = $3, key_fingerprint = $4, updated_at = NOW()
			WHERE id = $1 AND key_fingerprint = $5`,
			id, nik, npwp, rotator.ToFingerprint(), rotator.FromFingerprint()); err != nil {
			return rotated, err
		}
		rotated++
	}
	return rotated, nil
}
