// Command rotatekyc re-seals stored identity numbers from one encryption key
// to another.
//
// A separate command rather than a scheduled task, deliberately. Rotation needs
// two keys in the same process at once, and a long-running server holding both
// would be one configuration mistake away from writing new data with the old
// key. This exists for as long as the rotation takes, then stops.
//
//	KYC_ROTATE_FROM=<old base64 key> KYC_ROTATE_TO=<new base64 key> \
//	  DATABASE_URL=... go run ./cmd/rotatekyc
//
// Safe to run repeatedly: it only touches records still stamped with the old
// key, so it resumes where it stopped and does nothing once finished. The count
// still holding the old fingerprint is what says whether the old key can be
// destroyed.
package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/crypto"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	rotator, err := crypto.NewRotator(
		strings.TrimSpace(os.Getenv("KYC_ROTATE_FROM")),
		strings.TrimSpace(os.Getenv("KYC_ROTATE_TO")),
	)
	if err != nil {
		logger.Error("prepare rotation", "error", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	kyc := repository.NewKYCRepository(pool)
	logger.Info("rotating KYC encryption key",
		"from", rotator.FromFingerprint(), "to", rotator.ToFingerprint())

	total := 0
	for {
		rotated, err := kyc.RotateKey(ctx, rotator, 100)
		if err != nil {
			// Stops rather than skipping. A record the old key cannot open must
			// not be left stamped with a key that cannot read it, and whatever
			// caused that needs a person before more rows move.
			logger.Error("rotation stopped", "error", err, "rotated_so_far", total)
			os.Exit(1)
		}
		if rotated == 0 {
			break
		}
		total += rotated
		logger.Info("batch rotated", "count", rotated, "total", total)
		// Gentle on a live database: this runs while people are using it, and
		// there is no deadline.
		time.Sleep(200 * time.Millisecond)
	}

	remaining, err := kyc.KeyFingerprintsInUse(ctx)
	if err != nil {
		logger.Error("read progress", "error", err)
		os.Exit(1)
	}
	stillOld := remaining[rotator.FromFingerprint()]
	logger.Info("rotation finished", "rotated", total,
		"still_on_old_key", stillOld, "records_by_key", remaining)
	if stillOld > 0 {
		logger.Error("records still hold the old key; do not destroy it yet", "count", stillOld)
		os.Exit(1)
	}
	logger.Info("the old key is no longer needed by any stored record")
}
