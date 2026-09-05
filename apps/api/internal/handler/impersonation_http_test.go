package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	db "github.com/hajj-saas/api/internal/gen/db"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Impersonation end to end: the session sees the tenant's own data, and cannot
// change any of it.
//
// The write attempt is the assertion that matters. Everything else here is
// setup for it — a read-only claim that is only enforced by the handlers
// avoiding writes is not enforcement, so this drives a real create RPC through
// the real interceptor and requires it to be refused.
func TestImpersonationReadsTenantDataAndCannotWriteIntegration(t *testing.T) {
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

	// The admin, with their own operator, and a customer they will look at.
	fixture := newHTTPFixture(t, pool)
	var adminUserID string
	if err := pool.QueryRow(ctx, `SELECT "userId" FROM session WHERE token = $1`, fixture.sessionToken).Scan(&adminUserID); err != nil {
		t.Fatalf("read session user: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM platform_admins`); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_admins (user_id) VALUES ($1)`, adminUserID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, adminUserID); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM platform_admins`) })

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	customer, customerOrg, suffix := uuid.NewString(), "pelanggan-org-"+uuid.NewString(), uuid.NewString()[:8]
	exec(`INSERT INTO organization (id, name, slug, "createdAt") VALUES ($1,'Pelanggan',$2,NOW())`, customerOrg, "pel-"+suffix)
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
	      VALUES ($1,$2,'Travel Pelanggan','ID',$3,$4)`, customer, customerOrg, "pel-"+suffix+"@example.test", "pel-"+suffix)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM operators WHERE id = $1`, customer)
		_, _ = pool.Exec(bg, `DELETE FROM organization WHERE id = $1`, customerOrg)
	})
	customerSeason := uuid.NewString()
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity)
	      VALUES ($1,$2,'Musim Pelanggan','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',25)`, customerSeason, customer)

	queries := db.New(pool)
	platform := service.NewPlatformService(repository.NewPlatformRepository(pool),
		repository.NewSupplierCostRepository(pool), repository.NewSupplierRepository(pool),
		repository.NewProductRepository(queries, pool), repository.NewSubscriptionRepository(pool),
		repository.NewKYCRepository(pool), repository.NewAuditRepository(queries),
		repository.NewFunnelRepository(pool), repository.NewImpersonationRepository(pool), repository.NewPersonalDataReadRepository(pool), nil)

	interceptors := connect.WithInterceptors(middleware.NewAuthInterceptorWithImpersonation(pool,
		repository.NewIdentityRepository(queries, repository.NewAgentRepository(queries)),
		repository.NewSubscriptionRepository(pool),
		repository.NewImpersonationRepository(pool),
		repository.NewPersonalDataReadRepository(pool)))

	mux := http.NewServeMux()
	platformPath, platformHandler := hajjv1connect.NewPlatformServiceHandler(handler.NewPlatformHandler(platform), interceptors)
	mux.Handle(platformPath, platformHandler)
	seasonService := service.NewSeasonService(repository.NewOperatorRepository(queries),
		repository.NewSeasonRepository(queries), repository.NewAuditRepository(queries),
		repository.NewAnalyticsRepository(queries), repository.NewMonitoringRepository(queries))
	seasonPath, seasonHandler := hajjv1connect.NewSeasonServiceHandler(handler.NewSeasonHandler(seasonService), interceptors)
	mux.Handle(seasonPath, seasonHandler)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	platformClient := hajjv1connect.NewPlatformServiceClient(server.Client(), server.URL)
	seasonClient := hajjv1connect.NewSeasonServiceClient(server.Client(), server.URL)

	authorise := func(r interface{ Header() http.Header }, impersonation string) {
		r.Header().Set("Authorization", "Bearer "+fixture.sessionToken)
		if impersonation != "" {
			r.Header().Set(middleware.ImpersonationHeader, impersonation)
		}
	}

	// A reason under ten characters is refused: this column is the only thing
	// that will explain the session later.
	shallow := connect.NewRequest(&hajjv1.StartImpersonationRequest{
		OperatorId: customer, Reason: "cek", Minutes: 15, IdempotencyKey: "imp-" + uuid.NewString(),
	})
	authorise(shallow, "")
	if _, err := platformClient.StartImpersonation(ctx, shallow); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("alasan pendek = %v, mau invalid_argument", connect.CodeOf(err))
	}

	startReq := connect.NewRequest(&hajjv1.StartImpersonationRequest{
		OperatorId: customer, Reason: "pelanggan melaporkan daftar musim kosong",
		Minutes: 15, IdempotencyKey: "imp-" + uuid.NewString(),
	})
	authorise(startReq, "")
	started, err := platformClient.StartImpersonation(ctx, startReq)
	if err != nil {
		t.Fatalf("start impersonation: %v", err)
	}
	token := started.Msg.GetToken()
	if len(token) != 64 {
		t.Fatalf("token panjangnya %d", len(token))
	}
	// The token must never be readable back out of the database.
	var stored int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM impersonation_sessions WHERE token_hash = $1`, token).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatal("token tersimpan apa adanya di database")
	}

	// The session sees the customer's data, not the admin's own operator.
	listReq := connect.NewRequest(&hajjv1.ListSeasonsRequest{})
	authorise(listReq, token)
	seasons, err := seasonClient.ListSeasons(ctx, listReq)
	if err != nil {
		t.Fatalf("list seasons while impersonating: %v", err)
	}
	sawCustomerSeason := false
	for _, season := range seasons.Msg.GetSeasons() {
		if season.GetId() == customerSeason {
			sawCustomerSeason = true
		}
		if season.GetName() == "Musim HTTP" {
			t.Fatal("musim milik operator admin sendiri terbaca — impersonasi tidak mengganti tenant")
		}
	}
	if !sawCustomerSeason {
		t.Fatalf("musim pelanggan tidak terbaca (%d musim)", len(seasons.Msg.GetSeasons()))
	}

	// The assertion this whole feature rests on.
	//
	// The request is deliberately valid in every other respect — dates and all
	// — so that removing the guard really would insert the row. A request that
	// validation would reject anyway proves nothing about the guard.
	createReq := connect.NewRequest(&hajjv1.CreateSeasonRequest{
		Name: "Musim Dibuat Saat Impersonasi", Type: hajjv1.SeasonType_SEASON_TYPE_UMRAH_REGULER,
		StartDate: timestamppb.New(time.Now()),
		EndDate:   timestamppb.New(time.Now().Add(30 * 24 * time.Hour)),
		Capacity:  10,
	})
	authorise(createReq, token)
	if _, err := seasonClient.CreateSeason(ctx, createReq); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("menulis saat impersonasi = %v, mau permission_denied", connect.CodeOf(err))
	}
	var written int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM seasons WHERE name = 'Musim Dibuat Saat Impersonasi'`).Scan(&written); err != nil {
		t.Fatal(err)
	}
	if written != 0 {
		t.Fatal("baris tertulis walau panggilannya ditolak")
	}

	// The platform's own surface is closed while impersonating, even though
	// this same admin may call it with their own session.
	closedReq := connect.NewRequest(&hajjv1.ListUsageRequest{})
	authorise(closedReq, token)
	if _, err := platformClient.ListUsage(ctx, closedReq); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("panel platform saat impersonasi = %v, mau permission_denied", connect.CodeOf(err))
	}

	// A made-up token is not a way in.
	forgedReq := connect.NewRequest(&hajjv1.ListSeasonsRequest{})
	authorise(forgedReq, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if _, err := seasonClient.ListSeasons(ctx, forgedReq); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("token palsu = %v, mau unauthenticated", connect.CodeOf(err))
	}

	// Expiry is enforced by the lookup, not by a timer somebody has to remember
	// to run.
	// Both timestamps move, because the table refuses a session that expires
	// before it began — the fixture has to age the session, not corrupt it.
	if _, err := pool.Exec(ctx, `UPDATE impersonation_sessions
		SET started_at = NOW() - INTERVAL '2 hours', expires_at = NOW() - INTERVAL '1 minute'
		WHERE id = $1`, started.Msg.GetSessionId()); err != nil {
		t.Fatal(err)
	}
	expiredReq := connect.NewRequest(&hajjv1.ListSeasonsRequest{})
	authorise(expiredReq, token)
	if _, err := seasonClient.ListSeasons(ctx, expiredReq); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("sesi kedaluwarsa = %v, mau unauthenticated", connect.CodeOf(err))
	}

	// Closing early works on a live session, and the row survives it.
	secondReq := connect.NewRequest(&hajjv1.StartImpersonationRequest{
		OperatorId: customer, Reason: "memeriksa penutupan sesi lebih awal",
		Minutes: 15, IdempotencyKey: "imp-" + uuid.NewString(),
	})
	authorise(secondReq, "")
	second, err := platformClient.StartImpersonation(ctx, secondReq)
	if err != nil {
		t.Fatalf("start kedua: %v", err)
	}
	endReq := connect.NewRequest(&hajjv1.EndImpersonationRequest{Token: second.Msg.GetToken()})
	authorise(endReq, "")
	if _, err := platformClient.EndImpersonation(ctx, endReq); err != nil {
		t.Fatalf("end impersonation: %v", err)
	}
	endedReq := connect.NewRequest(&hajjv1.ListSeasonsRequest{})
	authorise(endedReq, second.Msg.GetToken())
	if _, err := seasonClient.ListSeasons(ctx, endedReq); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("sesi yang ditutup masih bisa dipakai: %v", connect.CodeOf(err))
	}

	// Reading a tenant's jamaah while impersonating leaves a record. Changes
	// were always audited; reads were not, and this is the surface that made
	// unrecorded reads possible at scale.
	var readCount int32
	var insideView bool
	if err := pool.QueryRow(ctx, `SELECT read_count, impersonation_id IS NOT NULL
		FROM personal_data_reads
		WHERE operator_id = $1 AND procedure = '/hajj.v1.SeasonService/ListSeasons'`, customer).
		Scan(&readCount, &insideView); err == nil {
		t.Fatal("ListSeasons tercatat sebagai pembacaan data pribadi — musim bukan orang, dan mencatat semuanya mengubur yang penting")
	}

	pilgrimService := service.NewPilgrimService(repository.NewOperatorRepository(queries),
		repository.NewPilgrimRepository(queries), repository.NewAccommodationRepository(queries),
		repository.NewTransportRepository(queries, pool), repository.NewAuditRepository(queries), pool)
	pilgrimPath, pilgrimHandler := hajjv1connect.NewPilgrimServiceHandler(handler.NewPilgrimHandler(pilgrimService), interceptors)
	mux.Handle(pilgrimPath, pilgrimHandler)
	pilgrimClient := hajjv1connect.NewPilgrimServiceClient(server.Client(), server.URL)

	third := connect.NewRequest(&hajjv1.StartImpersonationRequest{
		OperatorId: customer, Reason: "memeriksa pencatatan pembacaan data pribadi",
		Minutes: 15, IdempotencyKey: "imp-" + uuid.NewString(),
	})
	authorise(third, "")
	live, err := platformClient.StartImpersonation(ctx, third)
	if err != nil {
		t.Fatalf("start ketiga: %v", err)
	}
	for range 3 {
		pilgrimReq := connect.NewRequest(&hajjv1.ListPilgrimsRequest{SeasonId: customerSeason})
		authorise(pilgrimReq, live.Msg.GetToken())
		if _, err := pilgrimClient.ListPilgrims(ctx, pilgrimReq); err != nil {
			t.Fatalf("list pilgrims while impersonating: %v", err)
		}
	}
	if err := pool.QueryRow(ctx, `SELECT read_count, impersonation_id IS NOT NULL
		FROM personal_data_reads
		WHERE operator_id = $1 AND procedure = '/hajj.v1.PilgrimService/ListPilgrims'`, customer).
		Scan(&readCount, &insideView); err != nil {
		t.Fatalf("pembacaan data pribadi tidak tercatat: %v", err)
	}
	// One row with a count, not three rows: a row per request would be tens of
	// thousands nobody reads.
	if readCount != 3 {
		t.Fatalf("read_count = %d, mau 3", readCount)
	}
	if !insideView {
		t.Fatal("pembacaan tidak dikaitkan dengan sesi impersonasinya")
	}

	readsReq := connect.NewRequest(&hajjv1.ListPersonalDataReadsRequest{OperatorId: customer})
	authorise(readsReq, "")
	reads, err := platformClient.ListPersonalDataReads(ctx, readsReq)
	if err != nil {
		t.Fatalf("list personal data reads: %v", err)
	}
	if len(reads.Msg.GetReads()) == 0 {
		t.Fatal("catatan pembacaan tidak terbaca kembali")
	}

	// Every session is on the customer's own record, closed or not.
	historyReq := connect.NewRequest(&hajjv1.ListImpersonationsRequest{OperatorId: customer})
	authorise(historyReq, "")
	history, err := platformClient.ListImpersonations(ctx, historyReq)
	if err != nil {
		t.Fatalf("list impersonations: %v", err)
	}
	if len(history.Msg.GetSessions()) < 2 {
		t.Fatalf("%d sesi tercatat, mau 2", len(history.Msg.GetSessions()))
	}
	for _, row := range history.Msg.GetSessions() {
		if row.GetReason() == "" {
			t.Fatal("sesi tercatat tanpa alasan")
		}
	}
	var audited int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs
		WHERE operator_id = $1 AND action IN ('impersonation_started','impersonation_ended')`, customer).Scan(&audited); err != nil {
		t.Fatal(err)
	}
	if audited < 3 {
		t.Fatalf("%d entri audit, mau minimal 3 (dua mulai, satu tutup)", audited)
	}
}
