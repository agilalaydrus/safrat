// Command storefront-backfill adopts storefront objects that predate the asset
// registry (migration 084) into it, so their bytes count toward each operator's
// quota and the cleanup worker can manage them.
//
// It is a one-time, re-runnable maintenance job. It only ever reads the bucket
// and inserts registry rows — it never uploads, moves, or deletes an object,
// and it never modifies a row that already exists.
//
// It defaults to a dry run. The reported counts come from the real statement
// executed inside a transaction that is rolled back, so they are exact rather
// than estimated.
//
//	go run ./cmd/storefront-backfill              # report only
//	go run ./cmd/storefront-backfill -apply       # write the registry rows
//
// Adopting an object brings it under the cleanup worker's mark-and-sweep. An
// adopted object that is referenced by neither the draft nor the published
// storefront snapshot is marked unused and deleted after the seven-day
// recovery window. Run the dry run first and confirm the counts.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	apply := flag.Bool("apply", false, "write the registry rows; without this the run only reports what it would do")
	batchSize := flag.Int("batch", 500, "objects fetched and adopted per batch (1-1000)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *apply, int32(*batchSize)); err != nil {
		fmt.Fprintf(os.Stderr, "storefront-backfill: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, apply bool, batchSize int32) error {
	objectStorage, err := storage.New(ctx, storage.ConfigFromEnv())
	if err != nil {
		return fmt.Errorf("init object storage: %w", err)
	}
	if objectStorage == nil {
		return fmt.Errorf("object storage is not configured; set the S3_* variables")
	}
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	assets := repository.NewStorefrontAssetRepository(pool)

	if apply {
		fmt.Println("mode: APPLY — registry rows will be written")
	} else {
		fmt.Println("mode: DRY RUN — no rows will be written (pass -apply to commit)")
	}

	var total domain.StorefrontBackfillReport
	var scanned int
	var unparsable []string
	token := ""
	for {
		objects, skipped, nextToken, listErr := objectStorage.ListStorefrontObjects(ctx, token, batchSize)
		if listErr != nil {
			return listErr
		}
		scanned += len(objects) + len(skipped)
		unparsable = append(unparsable, skipped...)

		imports := make([]domain.StorefrontAssetImport, 0, len(objects))
		for _, object := range objects {
			imports = append(imports, domain.StorefrontAssetImport{
				ReservationKey: object.ReservationKey(),
				ObjectKey:      object.ObjectKey,
				OperatorID:     object.OperatorID,
				Kind:           object.Kind,
				PublicURL:      object.PublicURL,
				SizeBytes:      object.SizeBytes,
			})
		}
		report, backfillErr := assets.BackfillLive(ctx, imports, apply)
		if backfillErr != nil {
			return fmt.Errorf("adopt batch: %w", backfillErr)
		}
		total.Add(report)

		if nextToken == "" {
			break
		}
		token = nextToken
	}

	verb := "would adopt"
	if apply {
		verb = "adopted"
	}
	fmt.Printf("\nobjects scanned:      %d\n", scanned)
	fmt.Printf("%-21s %d\n", verb+":", total.Inserted)
	fmt.Printf("already registered:   %d\n", total.AlreadyRegistered)
	fmt.Printf("unknown operator:     %d\n", total.UnknownOperator)
	fmt.Printf("size out of bounds:   %d\n", total.InvalidSize)
	fmt.Printf("unparsable keys:      %d\n", len(unparsable))
	for _, key := range unparsable {
		fmt.Printf("  skipped: %s\n", key)
	}
	if !apply && total.Inserted > 0 {
		fmt.Printf("\nRe-run with -apply to write these %d rows.\n", total.Inserted)
		fmt.Println("Adopted objects that no storefront snapshot references will be")
		fmt.Println("deleted by the cleanup worker after its seven-day recovery window.")
	}
	return nil
}
