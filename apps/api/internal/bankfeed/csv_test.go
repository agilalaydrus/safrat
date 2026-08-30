package bankfeed

import (
	"strings"
	"testing"
)

// Statement formatting is where a silent error would live: a misread separator
// turns Rp1.500.000 into Rp1.500, and the credit then matches no invoice and
// looks like a customer who never paid.
func TestAmountsAreReadTheWayIndonesianStatementsWriteThem(t *testing.T) {
	cases := map[string]int64{
		"1.500.000":      1_500_000,
		"1,500,000":      1_500_000,
		"1.500.000,00":   1_500_000,
		"1,500,000.00":   1_500_000,
		"Rp 2.750.123":   2_750_123,
		"750000":         750_000,
		"1.234.567,89":   1_234_567,
	}
	for raw, want := range cases {
		got, err := parseAmount(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if got != want {
			t.Fatalf("%q terbaca %d, mau %d", raw, got, want)
		}
	}

	for _, bad := range []string{"", "abc", "Rp"} {
		if _, err := parseAmount(bad); err == nil {
			t.Fatalf("%q diterima sebagai nominal", bad)
		}
	}
}

// A debit reaching the feed would be recorded as money arriving, which is the
// one mistake this whole path exists to avoid.
func TestOnlyCreditsAreReported(t *testing.T) {
	statement := `Tanggal,Keterangan,Nominal,Referensi
01/08/2026,TRANSFER MASUK ANDI,1.500.000,REF-1
02/08/2026,BIAYA ADMIN,-15.000,REF-2
03/08/2026,TRANSFER MASUK BUDI,2.000.000,REF-3`

	source := &CSVSource{
		Reader: strings.NewReader(statement), DateColumn: "Tanggal",
		AmountColumn: "Nominal", DescriptionColumn: "Keterangan",
		ReferenceColumn: "Referensi", DateLayout: "02/01/2006", SourceName: "SCRAPER",
	}
	mutations, err := source.Fetch()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(mutations) != 2 {
		t.Fatalf("%d mutasi, mau 2 — debit ikut terbawa", len(mutations))
	}
	if mutations[0].ExternalID != "REF-1" || mutations[0].AmountIDR != 1_500_000 {
		t.Fatalf("mutasi pertama = %+v", mutations[0])
	}
}

// Re-importing the same statement must not create new rows. Without the
// bank's own reference the id is derived from the content, so the same line
// derives the same id every time.
func TestReimportDerivesTheSameIdentity(t *testing.T) {
	statement := `Tanggal,Keterangan,Nominal
01/08/2026,TRANSFER MASUK ANDI,1.500.000`

	read := func() []Mutation {
		source := &CSVSource{
			Reader: strings.NewReader(statement), DateColumn: "Tanggal",
			AmountColumn: "Nominal", DescriptionColumn: "Keterangan",
			DateLayout: "02/01/2006", SourceName: "SCRAPER",
		}
		mutations, err := source.Fetch()
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}
		return mutations
	}

	first, second := read(), read()
	if first[0].ExternalID != second[0].ExternalID {
		t.Fatalf("impor ulang menghasilkan id berbeda: %s vs %s", first[0].ExternalID, second[0].ExternalID)
	}
	if !strings.HasPrefix(first[0].ExternalID, "derived-") {
		t.Fatalf("id turunan tidak ditandai sebagai turunan: %s", first[0].ExternalID)
	}
}

// A column that is not there must stop the import rather than silently produce
// rows with zero amounts.
func TestMissingColumnStopsTheImport(t *testing.T) {
	source := &CSVSource{
		Reader:       strings.NewReader("Tanggal,Keterangan\n01/08/2026,X"),
		DateColumn:   "Tanggal",
		AmountColumn: "Nominal",
		DateLayout:   "02/01/2006", SourceName: "SCRAPER",
	}
	if _, err := source.Fetch(); err == nil {
		t.Fatal("kolom nominal yang hilang tidak menghentikan impor")
	}
}
