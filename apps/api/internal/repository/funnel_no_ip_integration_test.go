package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The funnel's whole claim to being aggregate data rests on one fact: no IP
// address is ever written down. That claim is a promise made in
// INSIDEN-DATA-PRIBADI.md, so it is checked by a test rather than by memory —
// a later migration adding a convenient `ip` column would otherwise change the
// legal character of the table with nobody noticing.
func TestFunnelTablesHoldNoAddressesIntegration(t *testing.T) {
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

	rows, err := pool.Query(ctx, `
		SELECT table_name, column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name IN ('funnel_events', 'funnel_daily')`)
	if err != nil {
		t.Fatalf("skema: %v", err)
	}
	defer rows.Close()

	// Names that would mean somebody stored an address, however they spelled it.
	suspect := []string{"ip", "addr", "address", "remote", "client_host"}
	seen := 0
	for rows.Next() {
		var table, column, dataType string
		if err := rows.Scan(&table, &column, &dataType); err != nil {
			t.Fatal(err)
		}
		seen++
		// inet and cidr exist for exactly one purpose.
		if dataType == "inet" || dataType == "cidr" {
			t.Fatalf("%s.%s bertipe %s — kolom ini hanya ada untuk menyimpan alamat", table, column, dataType)
		}
		lowered := strings.ToLower(column)
		for _, needle := range suspect {
			if lowered == needle || strings.HasPrefix(lowered, needle+"_") || strings.HasSuffix(lowered, "_"+needle) {
				t.Fatalf("%s.%s namanya menyiratkan alamat. Kalau memang bukan, ganti namanya; kalau memang iya, klaim di INSIDEN-DATA-PRIBADI.md tidak lagi benar", table, column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	// A schema that matched nothing would pass every assertion above without
	// checking anything at all.
	if seen == 0 {
		t.Fatal("tidak ada kolom yang diperiksa — tabel corong tidak ada di database uji?")
	}

	// The visitor token must be a hash, and only a hash. A column typed loosely
	// enough to hold an address eventually holds one.
	var check string
	if err := pool.QueryRow(ctx, `
		SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid = 'funnel_events'::regclass AND conname = 'funnel_events_visitor_hash_check'`).Scan(&check); err != nil {
		t.Fatalf("batasan hash pengunjung hilang: %v", err)
	}
	if !strings.Contains(check, "64") {
		t.Fatalf("batasan panjang hash tidak lagi 64 karakter: %s", check)
	}
}
