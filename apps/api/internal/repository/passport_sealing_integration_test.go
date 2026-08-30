package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/crypto"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A passport number is the field in the pilgrims table that enables
// impersonation rather than embarrassment. These are the properties that make
// encrypting it worth the change: a stolen dump gives nothing, and the one
// lookup that needs it still works.
func TestPassportsAreSealedAndStillFindableIntegration(t *testing.T) {
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	sealer, err := crypto.NewSealer("Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4Zm9vYmFyYmE=")
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	previous := kycSealer
	SetKYCSealer(sealer)
	t.Cleanup(func() { SetKYCSealer(previous) })

	queries := db.New(pool)
	pilgrims := NewPilgrimRepository(queries)

	operatorID, seasonID := uuid.NewString(), uuid.NewString()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("fixture %q: %v", q, err)
		}
	}
	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Paspor Uji','ID',$3,$4)`,
		operatorID, "pass-"+uuid.NewString(), operatorID[:8]+"@example.test", "pass-"+operatorID[:8])
	t.Cleanup(func() {
		bg := context.Background()
		tx, err := pool.Begin(bg)
		if err != nil {
			return
		}
		defer func() { _ = tx.Rollback(bg) }()
		if _, err := tx.Exec(bg, `SELECT set_config('app.allow_ledger_purge','on',true)`); err != nil {
			return
		}
		if _, err := tx.Exec(bg, `DELETE FROM operators WHERE id = $1`, operatorID); err != nil {
			return
		}
		_ = tx.Commit(bg)
	})
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',50)`, seasonID, operatorID)

	const passport = "C1234567"
	created, err := pilgrims.Create(ctx, operatorID, domain.PilgrimInput{
		SeasonID: seasonID, FullName: "Jamaah Paspor",
		PassportNumber: passport, Nationality: "ID", Gender: "MALE",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// What the caller sees is the number. What the disk holds is not.
	if created.PassportNumber != passport {
		t.Fatalf("dibaca %q, mau %q — enkripsi tidak boleh terlihat pemanggil", created.PassportNumber, passport)
	}
	var stored, blind, fingerprint string
	if err := pool.QueryRow(ctx,
		`SELECT passport_number, passport_number_blind, passport_key_fingerprint FROM pilgrims WHERE id = $1`,
		created.ID).Scan(&stored, &blind, &fingerprint); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if strings.Contains(stored, passport) {
		t.Fatalf("nomor paspor tersimpan apa adanya: %q", stored)
	}
	if !strings.HasPrefix(stored, "v1.") {
		t.Fatalf("tersimpan tanpa penanda terenkripsi: %q", stored)
	}
	if blind == "" || fingerprint == "" {
		t.Fatal("token pencarian atau sidik jari kunci kosong; barisnya tidak akan bisa ditemukan atau didiagnosis")
	}

	// The one lookup that needs the value back, and the normalisation that
	// makes it usable by a person typing.
	for _, typed := range []string{passport, "c1234567", " C1234567 "} {
		found, err := pilgrims.GetByPassport(ctx, operatorID, seasonID, typed)
		if err != nil {
			t.Fatalf("cari %q: %v", typed, err)
		}
		if found.ID != created.ID {
			t.Fatalf("cari %q menemukan jamaah lain", typed)
		}
	}

	// A row written before encryption must keep working, and the backfill must
	// move it without a stop-the-world migration.
	legacyID := uuid.NewString()
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender)
	      VALUES ($1,$2,$3,'Jamaah Lama','D7654321','ID','1990-01-01'::timestamptz,'MALE')`,
		legacyID, seasonID, operatorID)

	legacyFound, err := pilgrims.GetByPassport(ctx, operatorID, seasonID, "D7654321")
	if err != nil || legacyFound.ID != legacyID {
		t.Fatalf("baris lama tidak dapat dicari sebelum backfill: %v", err)
	}

	before, err := pilgrims.LegacyPassportCount(ctx)
	if err != nil || before == 0 {
		t.Fatalf("hitungan baris lama = %d (%v)", before, err)
	}
	if _, err := pilgrims.MigrateLegacyPassports(ctx, 500); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	// Re-running finds nothing, and must not double-seal what it already sealed.
	again, err := pilgrims.MigrateLegacyPassports(ctx, 500)
	if err != nil {
		t.Fatalf("backfill ulang: %v", err)
	}
	if again != 0 {
		t.Fatalf("backfill ulang memindahkan %d baris; seharusnya nol", again)
	}

	migrated, err := pilgrims.GetByPassport(ctx, operatorID, seasonID, "D7654321")
	if err != nil {
		t.Fatalf("cari setelah backfill: %v", err)
	}
	if migrated.PassportNumber != "D7654321" {
		t.Fatalf("setelah backfill terbaca %q — enkripsi ganda merusaknya", migrated.PassportNumber)
	}
}
