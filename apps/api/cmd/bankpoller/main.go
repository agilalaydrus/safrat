// Command bankpoller reads a bank statement and delivers the credits it finds
// to the API's bank feed.
//
// Run it against a statement downloaded from internet banking:
//
//	BANK_FEED_SECRET=… bankpoller \
//	  -file mutasi-agustus.csv \
//	  -endpoint https://api.tawafiqhub.id/webhooks/bank-feed \
//	  -date-column Tanggal -amount-column Nominal \
//	  -description-column Keterangan -reference-column Referensi
//
// Safe to run repeatedly on the same file. The endpoint keys credits by their
// external id, so a re-import records nothing new and settles nothing twice —
// which is what makes "run it again if you are unsure" the right advice.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/hajj-saas/api/internal/bankfeed"
)

func main() {
	var (
		file       = flag.String("file", "", "berkas CSV mutasi rekening")
		endpoint   = flag.String("endpoint", "http://127.0.0.1:8131/webhooks/bank-feed", "alamat endpoint feed")
		dateCol    = flag.String("date-column", "Tanggal", "nama kolom tanggal")
		amountCol  = flag.String("amount-column", "Nominal", "nama kolom nominal")
		descCol    = flag.String("description-column", "Keterangan", "nama kolom keterangan")
		refCol     = flag.String("reference-column", "", "nama kolom nomor referensi bank, kalau ada")
		dateLayout = flag.String("date-layout", "02/01/2006", "format tanggal di kolomnya")
		source     = flag.String("source", "SCRAPER", "API atau SCRAPER")
		dryRun     = flag.Bool("dry-run", false, "baca dan tampilkan saja, jangan kirim")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if *file == "" {
		logger.Error("-file wajib diisi")
		os.Exit(2)
	}
	if *source != "API" && *source != "SCRAPER" {
		logger.Error("-source harus API atau SCRAPER")
		os.Exit(2)
	}

	handle, err := os.Open(*file)
	if err != nil {
		logger.Error("buka berkas", "error", err)
		os.Exit(1)
	}
	defer func() { _ = handle.Close() }()

	reader := &bankfeed.CSVSource{
		Reader: handle, DateColumn: *dateCol, AmountColumn: *amountCol,
		DescriptionColumn: *descCol, ReferenceColumn: *refCol,
		DateLayout: *dateLayout, SourceName: *source,
	}
	mutations, err := reader.Fetch()
	if err != nil {
		// A parse failure stops the run rather than sending a partial batch.
		// Half a statement delivered looks exactly like half the customers not
		// having paid.
		logger.Error("baca mutasi", "error", err)
		os.Exit(1)
	}

	if *refCol == "" {
		logger.Warn("tanpa kolom referensi, id diturunkan dari isi baris",
			"akibat", "dua transfer identik di hari yang sama dianggap satu; pakai ekspor yang memuat nomor referensi bila ada")
	}

	if *dryRun {
		for _, m := range mutations {
			fmt.Printf("%s  %12d  %s  %s\n", m.OccurredAt.Format("2006-01-02"), m.AmountIDR, m.ExternalID, m.Description)
		}
		logger.Info("dry run selesai", "mutasi", len(mutations))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := bankfeed.Deliver(ctx, *endpoint, os.Getenv("BANK_FEED_SECRET"), *source, mutations, nil)
	if err != nil {
		logger.Error("kirim ke feed", "error", err)
		os.Exit(1)
	}

	// Counted against what was read, not against what was newly recorded. On a
	// re-import nothing is new but credits are still matched, and subtracting
	// one from the other produced a negative "unmatched" — nonsense in a log
	// somebody is meant to act on.
	logger.Info("selesai", "dibaca", len(mutations),
		"dicatat_baru", result.Recorded, "cocok", result.Matched,
		"belum_ada_tagihannya", len(mutations)-result.Matched)
}
