package geoip

import (
	"os"
	"testing"
)

func TestResolverUnconfiguredNeverFails(t *testing.T) {
	r := Open("")
	if city, province := r.Lookup("202.152.162.1"); city != "" || province != "" {
		t.Fatalf("resolver tanpa database seharusnya diam, dapat city=%q province=%q", city, province)
	}
}

func TestResolverMissingFileNeverFails(t *testing.T) {
	r := Open("/nonexistent/path/does-not-exist.mmdb")
	if city, province := r.Lookup("202.152.162.1"); city != "" || province != "" {
		t.Fatalf("resolver dengan file hilang seharusnya diam, dapat city=%q province=%q", city, province)
	}
}

func TestResolverUnparseableIPReturnsEmpty(t *testing.T) {
	r := Open("")
	if city, province := r.Lookup("not-an-ip"); city != "" || province != "" {
		t.Fatalf("IP tidak valid seharusnya menghasilkan kosong, dapat city=%q province=%q", city, province)
	}
}

// TestResolverRealDatabaseIntegration proves the actual DB-IP City Lite
// schema decodes correctly — a wrong struct tag would silently return
// empty strings forever rather than fail loudly, so this is checked
// against a real downloaded file rather than trusted from documentation.
func TestResolverRealDatabaseIntegration(t *testing.T) {
	path := os.Getenv("GEOIP_TEST_DB_PATH")
	if path == "" {
		t.Skip("GEOIP_TEST_DB_PATH is not set")
	}
	r := Open(path)
	city, province := r.Lookup("202.152.162.1") // a Jakarta-area address
	if city == "" || province == "" {
		t.Fatalf("alamat Jakarta seharusnya menghasilkan city/province, dapat city=%q province=%q", city, province)
	}
	t.Logf("resolved: city=%q province=%q", city, province)
}
