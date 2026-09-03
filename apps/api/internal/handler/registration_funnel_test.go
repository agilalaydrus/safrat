package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/funnel"
	"github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A registration attempt records two steps, and the distance between them is
// the point.
//
// KIRIM is written before the attempt and SELESAI only when a row is created,
// so the gap counts people who tried to register and were refused by our own
// validation. That number is the most actionable one in the funnel and it
// vanishes entirely if the two are recorded together — which is why this test
// drives a rejected attempt as well as an accepted one.
func TestRegistrationRecordsAttemptAndCompletionSeparatelyIntegration(t *testing.T) {
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

	operatorID, seasonID := uuid.NewString(), uuid.NewString()
	suffix := uuid.NewString()[:8]
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
	      VALUES ($1,$2,'Corong Daftar','ID',$3,$4)`,
		operatorID, "rf-"+suffix, "rf-"+suffix+"@example.test", "rf-"+suffix)
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
	      VALUES ($1,$2,'Musim Corong','UMRAH_REGULER',NOW(),NOW()+INTERVAL '60 days',30)`, seasonID, operatorID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID)
	})

	queries := db.New(pool)
	funnelService := service.NewFunnelService(repository.NewFunnelRepository(pool), funnel.NewHasher(strings.Repeat("s", 32)))
	registrationService := service.NewRegistrationService(
		repository.NewOperatorRepository(queries), repository.NewRegistrationRepository(queries),
		repository.NewAuditRepository(queries), repository.NewAgentRepository(queries))
	path, serviceHandler := hajjv1connect.NewRegistrationServiceHandler(
		handler.NewRegistrationHandler(registrationService, funnelService))
	mux := http.NewServeMux()
	mux.Handle(path, serviceHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := hajjv1connect.NewRegistrationServiceClient(server.Client(), server.URL)

	const browser = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1"
	submit := func(seasonForRequest, name string) error {
		request := connect.NewRequest(&hajjv1.SubmitRegistrationRequest{
			OperatorId: operatorID, SeasonId: seasonForRequest, FullName: name,
			PassportNumber: "RF-" + uuid.NewString()[:8], Gender: "MALE", Nationality: "ID",
			Phone: "081234567890", Email: "corong@example.test", Address: "Jl. Uji",
			UtmSource: "instagram", UtmCampaign: "ramadhan",
		})
		request.Header().Set("User-Agent", browser)
		request.Header().Set("X-Real-IP", "103.150.60.90")
		_, err := client.SubmitRegistration(ctx, request)
		return err
	}
	steps := func(step string) int {
		t.Helper()
		var rows int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM funnel_events WHERE operator_id = $1 AND step = $2`, operatorID, step).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		return rows
	}

	if err := submit(seasonID, "Pendaftar Sah"); err != nil {
		t.Fatalf("pendaftaran sah ditolak: %v", err)
	}
	if steps("KIRIM") != 1 || steps("SELESAI") != 1 {
		t.Fatalf("pendaftaran berhasil: KIRIM=%d SELESAI=%d, mau 1 dan 1", steps("KIRIM"), steps("SELESAI"))
	}

	// A season that does not belong to this operator is refused by the service.
	// The attempt still counts; the completion must not.
	if err := submit(uuid.NewString(), "Pendaftar Gagal"); err == nil {
		t.Fatal("musim asing seharusnya ditolak")
	}
	if steps("KIRIM") != 2 {
		t.Fatalf("percobaan gagal tidak tercatat: KIRIM=%d — orang yang ditolak sistem kita jadi tak terlihat", steps("KIRIM"))
	}
	if steps("SELESAI") != 1 {
		t.Fatalf("percobaan gagal dihitung sebagai selesai: SELESAI=%d", steps("SELESAI"))
	}

	// Attribution is kept on the registration row itself, because the visitor
	// token resets at midnight and umrah is decided over weeks.
	var source, campaign string
	if err := pool.QueryRow(ctx, `SELECT utm_source, utm_campaign FROM pilgrim_registrations WHERE operator_id = $1`, operatorID).Scan(&source, &campaign); err != nil {
		t.Fatal(err)
	}
	if source != "instagram" || campaign != "ramadhan" {
		t.Fatalf("atribusi tidak tersimpan di baris pendaftaran: %q / %q", source, campaign)
	}

	// And SELESAI carries the registration it produced, so the funnel can be
	// joined to what it actually created.
	var linked int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM funnel_events f
		JOIN pilgrim_registrations r ON r.id = f.entity_id
		WHERE f.operator_id = $1 AND f.step = 'SELESAI'`, operatorID).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if linked != 1 {
		t.Fatalf("SELESAI tidak tertaut ke pendaftarannya: %d", linked)
	}
}
