package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The manifest, and the two judgements that make it usable rather than merely
// correct: a substituted pilgrim is not on the flight, and marriage proof is
// only asked of somebody travelling under a mahram.
func TestKloterManifestCountsOnlyWhoIsTravellingIntegration(t *testing.T) {
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

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	operatorID, seasonID, kloterID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	suffix := uuid.NewString()[:8]
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Uji Manifes','ID',$3,$4)`, operatorID, "man-"+suffix, "man-"+suffix+"@example.test", "man-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
		VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',60)`, seasonID, operatorID)
	exec(`INSERT INTO kloters (id,operator_id,season_id,code,embarkation,flight_number,departure_date,capacity)
		VALUES ($1,$2,$3,'KLT-01','Jakarta','GA-900',NOW()+INTERVAL '20 days',45)`, kloterID, operatorID, seasonID)

	newPilgrim := func(name string, mahram *string, substituted bool, passport, vaccine, photo, ktp, kk, mahramProof bool) string {
		id := uuid.NewString()
		exec(`INSERT INTO pilgrims (id,season_id,operator_id,kloter_id,full_name,passport_number,nationality,
			date_of_birth,gender,mahram_id,is_substituted,
			documents_passport,documents_vaccine,documents_photo,documents_ktp,documents_kk,documents_mahram_proof)
			VALUES ($1,$2,$3,$4,$5,$6,'ID','1985-05-05'::timestamptz,'MALE',$7,$8,$9,$10,$11,$12,$13,$14)`,
			id, seasonID, operatorID, kloterID, name, "P-"+uuid.NewString()[:8], mahram, substituted,
			passport, vaccine, photo, ktp, kk, mahramProof)
		return id
	}

	// Complete, and travelling alone: marriage proof is not asked of him.
	newPilgrim("Ahmad Lengkap", nil, false, true, true, true, true, true, false)
	// Travelling under a mahram, and missing exactly that proof.
	husband := newPilgrim("Budi Mahram", nil, false, true, true, true, true, true, false)
	newPilgrim("Siti Istri", &husband, false, true, true, true, true, true, false)
	// Missing two ordinary documents.
	newPilgrim("Candra Kurang", nil, false, false, true, false, true, true, false)
	// Substituted: not on this flight at all.
	newPilgrim("Dedi Diganti", nil, true, false, false, false, false, false, false)

	repo := NewKloterRepository(db.New(pool), pool)
	manifest, err := repo.Manifest(ctx, operatorID, kloterID)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if manifest.Code != "KLT-01" || manifest.FlightNumber != "GA-900" {
		t.Fatalf("kepala manifes salah: %+v", manifest)
	}
	if len(manifest.Rows) != 4 {
		t.Fatalf("%d baris, mau 4 — jamaah yang sudah digantikan ikut terhitung?", len(manifest.Rows))
	}
	for _, row := range manifest.Rows {
		if row.FullName == "Dedi Diganti" {
			t.Fatal("jamaah yang sudah digantikan muncul di manifes")
		}
	}

	byName := map[string][]string{}
	required := map[string]bool{}
	for _, row := range manifest.Rows {
		byName[row.FullName] = row.MissingDocuments()
		required[row.FullName] = row.MahramProofRequired
	}

	if len(byName["Ahmad Lengkap"]) != 0 {
		t.Fatalf("jamaah lengkap dianggap kurang: %v — buku nikah diminta dari yang tidak bermahram?", byName["Ahmad Lengkap"])
	}
	if required["Ahmad Lengkap"] {
		t.Fatal("buku nikah diminta dari jamaah tanpa mahram")
	}
	if !required["Siti Istri"] {
		t.Fatal("buku nikah tidak diminta dari jamaah yang bermahram")
	}
	if len(byName["Siti Istri"]) != 1 || byName["Siti Istri"][0] != "BUKU_NIKAH" {
		t.Fatalf("kekurangan jamaah bermahram salah: %v", byName["Siti Istri"])
	}
	// Named and in order, so a screen can say what to chase.
	if len(byName["Candra Kurang"]) != 2 ||
		byName["Candra Kurang"][0] != "PASPOR" || byName["Candra Kurang"][1] != "FOTO" {
		t.Fatalf("kekurangan tidak disebut berurutan: %v", byName["Candra Kurang"])
	}

	// Another operator sees nothing of this flight.
	other := uuid.NewString()
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Lain','ID',$3,$4)`, other, "manlain-"+suffix, "manlain-"+suffix+"@example.test", "manlain-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, other) })
	if _, err := repo.Manifest(ctx, other, kloterID); err == nil {
		t.Fatal("travel lain bisa membaca manifes kloter ini")
	}
}
