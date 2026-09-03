package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The roomlist, and the one thing it can see that the allocation rules cannot.
//
// Allocation already refuses to put a man in a room designated female. A room
// designated "family" accepts anyone by design, so a couple sharing one is
// ordinary and two strangers of different genders sharing one is not — and only
// the finished list can tell them apart.
func TestKloterRoomlistGroupsByHotelAndFlagsUnpairedFamilyRoomsIntegration(t *testing.T) {
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
	hotelID, suffix := uuid.NewString(), uuid.NewString()[:8]
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Uji Roomlist','ID',$3,$4)`, operatorID, "rl-"+suffix, "rl-"+suffix+"@example.test", "rl-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
		VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',60)`, seasonID, operatorID)
	exec(`INSERT INTO kloters (id,operator_id,season_id,code,capacity)
		VALUES ($1,$2,$3,'KLT-RL',40)`, kloterID, operatorID, seasonID)
	exec(`INSERT INTO hotels (id,operator_id,season_id,name,city)
		VALUES ($1,$2,$3,'Hotel Madinah','Madinah')`, hotelID, operatorID, seasonID)

	newRoom := func(number, gender string, capacity int32) string {
		id := uuid.NewString()
		exec(`INSERT INTO rooms (id,hotel_id,operator_id,room_number,room_type,capacity,gender)
			VALUES ($1,$2,$3,$4,'STANDARD',$5,$6)`, id, hotelID, operatorID, number, capacity, gender)
		return id
	}
	newPilgrim := func(name, gender string, mahram *string) string {
		id := uuid.NewString()
		exec(`INSERT INTO pilgrims (id,season_id,operator_id,kloter_id,full_name,passport_number,nationality,
			date_of_birth,gender,mahram_id)
			VALUES ($1,$2,$3,$4,$5,$6,'ID','1980-01-01'::timestamptz,$7,$8)`,
			id, seasonID, operatorID, kloterID, name, "P-"+uuid.NewString()[:8], gender, mahram)
		return id
	}
	allocate := func(roomID, pilgrimID string) {
		exec(`INSERT INTO room_allocations (room_id,pilgrim_id,operator_id,hotel_id)
			VALUES ($1,$2,$3,$4)`, roomID, pilgrimID, operatorID, hotelID)
	}

	// A married couple in a family room: ordinary, and must not be flagged.
	coupleRoom := newRoom("101", "family", 2)
	husband := newPilgrim("Budi Suami", "MALE", nil)
	wife := newPilgrim("Sari Istri", "FEMALE", &husband)
	allocate(coupleRoom, husband)
	allocate(coupleRoom, wife)

	// Two unrelated people of different genders in a family room: the case the
	// allocation rules cannot catch.
	strangerRoom := newRoom("102", "family", 3)
	strangerA := newPilgrim("Anwar Sendiri", "MALE", nil)
	strangerB := newPilgrim("Dewi Sendiri", "FEMALE", nil)
	allocate(strangerRoom, strangerA)
	allocate(strangerRoom, strangerB)

	// A single-gender room is never flagged, whatever the mahram links.
	menRoom := newRoom("103", "male", 4)
	allocate(menRoom, newPilgrim("Eko Jamaah", "MALE", nil))

	// Somebody with no bed at all.
	newPilgrim("Fajar Belum Dapat Kamar", "MALE", nil)

	repo := NewKloterRepository(db.New(pool), pool)
	list, err := repo.Roomlist(ctx, operatorID, kloterID)
	if err != nil {
		t.Fatalf("roomlist: %v", err)
	}
	if list.KloterCode != "KLT-RL" {
		t.Fatalf("kode kloter = %q", list.KloterCode)
	}
	if len(list.Hotels) != 1 || list.Hotels[0].City != "Madinah" {
		t.Fatalf("pengelompokan hotel salah: %+v", list.Hotels)
	}
	if len(list.Hotels[0].Rooms) != 3 {
		t.Fatalf("%d kamar, mau 3", len(list.Hotels[0].Rooms))
	}
	if list.Total != 6 {
		t.Fatalf("total jamaah = %d, mau 6", list.Total)
	}

	// The pilgrim with no bed is the most urgent line, and a join from
	// allocations could never produce their row.
	if len(list.Unassigned) != 1 || list.Unassigned[0].FullName != "Fajar Belum Dapat Kamar" {
		t.Fatalf("jamaah tanpa kamar tidak muncul: %+v", list.Unassigned)
	}

	byNumber := map[string]RoomlistRoom{}
	for _, room := range list.Hotels[0].Rooms {
		byNumber[room.RoomNumber] = room
	}
	if byNumber["101"].MixedWithoutMahram() {
		t.Fatal("pasangan suami istri di kamar keluarga ikut ditandai — mahram tercatat di satu sisi saja")
	}
	if !byNumber["102"].MixedWithoutMahram() {
		t.Fatal("dua orang tak berhubungan beda jenis kelamin di kamar keluarga tidak ditandai")
	}
	if byNumber["103"].MixedWithoutMahram() {
		t.Fatal("kamar satu jenis kelamin ditandai")
	}
	if len(byNumber["102"].Occupants) != 2 || byNumber["102"].Capacity != 3 {
		t.Fatalf("isi kamar 102 salah: %+v", byNumber["102"])
	}

	// Another operator sees nothing of this flight.
	other := uuid.NewString()
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Lain','ID',$3,$4)`, other, "rllain-"+suffix, "rllain-"+suffix+"@example.test", "rllain-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, other) })
	if _, err := repo.Roomlist(ctx, other, kloterID); err == nil {
		t.Fatal("travel lain bisa membaca roomlist kloter ini")
	}
}
