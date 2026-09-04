package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// A roll-call is taken one name at a time, and correcting a mis-mark must
// settle on the same attendance row rather than creating a second one that
// would double-count the pilgrim in the session's present/absent tally.
// This proves RecordAttendance is a true upsert and the summary reflects
// only the latest mark.
func TestManasikAttendanceUpsertsNotDuplicatesIntegration(t *testing.T) {
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
			t.Fatalf("fixture %q: %v", query, err)
		}
	}

	operatorID, orgID := uuid.NewString(), "manasik-"+uuid.NewString()
	seasonID, pilgrimID := uuid.NewString(), uuid.NewString()

	exec(`INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Manasik Uji','ID',$3,$4)`,
		operatorID, orgID, operatorID[:8]+"@example.test", "man-"+operatorID[:8])
	exec(`INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Uji','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`,
		seasonID, operatorID)
	exec(`INSERT INTO pilgrims (id, season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender) VALUES ($1,$2,$3,'Jamaah Uji','A1234567','ID','1990-01-01'::timestamptz,'MALE')`,
		pilgrimID, seasonID, operatorID)
	t.Cleanup(func() {
		exec(`DELETE FROM manasik_attendance WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM manasik_sessions WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM manasik_curricula WHERE operator_id = $1`, operatorID)
		exec(`DELETE FROM pilgrims WHERE id = $1`, pilgrimID)
		exec(`DELETE FROM seasons WHERE id = $1`, seasonID)
		exec(`DELETE FROM operators WHERE id = $1`, operatorID)
	})

	queries := db.New(pool)
	manasikService := NewManasikService(repository.NewOperatorRepository(queries), repository.NewManasikRepository(queries))

	curriculum, err := manasikService.CreateCurriculum(ctx, orgID, &hajjv1.CreateManasikCurriculumRequest{SeasonId: seasonID, Title: "Tawaf", SortOrder: 1})
	if err != nil {
		t.Fatalf("CreateCurriculum: %v", err)
	}

	session, err := manasikService.CreateSession(ctx, orgID, &hajjv1.CreateManasikSessionRequest{
		SeasonId: seasonID, CurriculumId: curriculum.Id, Title: "Sesi Tawaf 1",
		ScheduledAt: timestamppb.New(time.Now().Add(24 * time.Hour)), Capacity: 50,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sessions, err := manasikService.ListSessions(ctx, orgID, &hajjv1.ListManasikSessionsRequest{SeasonId: seasonID})
	if err != nil || len(sessions.Sessions) != 1 || sessions.Sessions[0].CurriculumTitle != "Tawaf" {
		t.Fatalf("ListSessions curriculum_title tidak ikut ter-join: %v / %+v", err, sessions)
	}

	if err := manasikService.RecordAttendance(ctx, orgID, &hajjv1.RecordManasikAttendanceRequest{SessionId: session.Id, PilgrimId: pilgrimID, Status: "ABSENT"}); err != nil {
		t.Fatalf("RecordAttendance (absent): %v", err)
	}
	// Correction: the coordinator mis-marked, the pilgrim was actually present.
	if err := manasikService.RecordAttendance(ctx, orgID, &hajjv1.RecordManasikAttendanceRequest{SessionId: session.Id, PilgrimId: pilgrimID, Status: "PRESENT"}); err != nil {
		t.Fatalf("RecordAttendance (present): %v", err)
	}

	attendance, err := manasikService.ListAttendance(ctx, orgID, &hajjv1.ListManasikAttendanceRequest{SessionId: session.Id})
	if err != nil {
		t.Fatalf("ListAttendance: %v", err)
	}
	if len(attendance.Rows) != 1 {
		t.Fatalf("baris absensi = %d, mau 1 (koreksi semestinya menimpa, bukan menambah baris)", len(attendance.Rows))
	}
	if attendance.Rows[0].Status != "PRESENT" {
		t.Fatalf("status akhir = %s, mau PRESENT", attendance.Rows[0].Status)
	}
	if attendance.PresentCount != 1 || attendance.AbsentCount != 0 {
		t.Fatalf("ringkasan = present:%d absent:%d, mau present:1 absent:0", attendance.PresentCount, attendance.AbsentCount)
	}
}
