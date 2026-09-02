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
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The recording endpoint, over real HTTP.
//
// Four behaviours carry the whole design and each is checked rather than
// assumed: a real visitor is counted once, a self-declared bot is not counted
// at all, an invented tenant cannot put rows anywhere, and without a salt
// nothing is written — because a token that can be reversed to an address
// would quietly turn this table into personal data.
func TestFunnelRecordingCountsPeopleAndIgnoresBotsIntegration(t *testing.T) {
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

	operatorID := uuid.NewString()
	slug := "funnel-" + uuid.NewString()[:8]
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Funnel Uji','ID',$3,$4)`, operatorID, "fn-"+slug, slug+"@example.test", slug); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID)
	})

	serve := func(salt string) hajjv1connect.FunnelServiceClient {
		t.Helper()
		funnelService := service.NewFunnelService(repository.NewFunnelRepository(pool), funnel.NewHasher(salt))
		path, serviceHandler := hajjv1connect.NewFunnelServiceHandler(handler.NewFunnelHandler(funnelService))
		mux := http.NewServeMux()
		mux.Handle(path, serviceHandler)
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		return hajjv1connect.NewFunnelServiceClient(server.Client(), server.URL)
	}
	send := func(client hajjv1connect.FunnelServiceClient, operatorSlug, step, agent, ip string) {
		t.Helper()
		request := connect.NewRequest(&hajjv1.RecordFunnelEventRequest{OperatorSlug: operatorSlug, Step: step, Path: "/"})
		request.Header().Set("User-Agent", agent)
		request.Header().Set("X-Real-IP", ip)
		if _, err := client.RecordEvent(ctx, request); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	count := func() int {
		t.Helper()
		var rows int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM funnel_events WHERE operator_id = $1`, operatorID).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		return rows
	}

	const browser = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1"
	client := serve(strings.Repeat("s", 32))

	send(client, slug, "LANDING", browser, "103.150.60.10")
	if count() != 1 {
		t.Fatalf("kunjungan sungguhan tidak tercatat: %d baris", count())
	}

	send(client, slug, "LANDING", "Mozilla/5.0 (compatible; Googlebot/2.1)", "66.249.66.1")
	send(client, slug, "LANDING", "curl/8.4.0", "1.2.3.4")
	if count() != 1 {
		t.Fatalf("bot ikut terhitung: %d baris — angka konversi akan terlihat jauh lebih buruk dari kenyataan", count())
	}

	// The same person, same day: one visitor, and the token proves it.
	send(client, slug, "KATALOG", browser, "103.150.60.10")
	var visitors int
	if err := pool.QueryRow(ctx, `SELECT COUNT(DISTINCT visitor_hash) FROM funnel_events WHERE operator_id = $1`, operatorID).Scan(&visitors); err != nil {
		t.Fatal(err)
	}
	if visitors != 1 {
		t.Fatalf("%d pengunjung untuk satu orang di satu hari", visitors)
	}

	// An invented tenant has no owner, and a row with no owner would be counted
	// as the platform's own traffic.
	before := count()
	var platformBefore int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM funnel_events WHERE operator_id IS NULL`).Scan(&platformBefore); err != nil {
		t.Fatal(err)
	}
	send(client, "tidak-ada-travel-ini", "LANDING", browser, "103.150.60.11")
	if count() != before {
		t.Fatal("slug yang tidak ada menulis baris atas nama travel lain")
	}
	// The row must not land as platform traffic either. Without this check the
	// guard could be removed and nothing would notice: an unowned row simply
	// becomes TawafiqHub's own visit, inflating the one funnel nobody is
	// double-checking.
	var platformAfter int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM funnel_events WHERE operator_id IS NULL`).Scan(&platformAfter); err != nil {
		t.Fatal(err)
	}
	if platformAfter != platformBefore {
		t.Fatalf("slug asing terhitung sebagai lalu lintas platform: %d → %d", platformBefore, platformAfter)
	}

	// No salt, no rows — and no error either, so a page still renders.
	unsalted := serve("")
	send(unsalted, slug, "LANDING", browser, "103.150.60.12")
	if count() != before {
		t.Fatalf("mencatat tanpa garam: %d baris — token yang bisa dibalik", count())
	}
}
