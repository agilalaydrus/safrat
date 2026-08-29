// Command rotatekyc re-seals stored identity numbers and refund payout
// destinations from one encryption key to another.
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
// key, so it resumes where it stopped and does nothing once finished. The API
// and worker must be stopped while it runs: a process holding the old key
// cannot read a row immediately after that row moves to the new key. The count
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
	payouts := repository.NewRefundPayoutRepository(pool)
	logger.Info("rotating stored encryption key",
		"from", rotator.FromFingerprint(), "to", rotator.ToFingerprint())

	kycTotal := 0
	for {
		rotated, err := kyc.RotateKey(ctx, rotator, 100)
		if err != nil {
			// Stops rather than skipping. A record the old key cannot open must
			// not be left stamped with a key that cannot read it, and whatever
			// caused that needs a person before more rows move.
			logger.Error("KYC rotation stopped", "error", err, "rotated_so_far", kycTotal)
			os.Exit(1)
		}
		if rotated == 0 {
			break
		}
		kycTotal += rotated
		logger.Info("KYC batch rotated", "count", rotated, "total", kycTotal)
		// Gentle on the database even during the maintenance window.
		time.Sleep(200 * time.Millisecond)
	}

	payoutTotal := 0
	for {
		rotated, err := payouts.RotateDestinationKey(ctx, rotator, 100)
		if err != nil {
			logger.Error("refund payout rotation stopped", "error", err, "rotated_so_far", payoutTotal)
			os.Exit(1)
		}
		if rotated == 0 {
			break
		}
		payoutTotal += rotated
		logger.Info("refund payout batch rotated", "count", rotated, "total", payoutTotal)
		time.Sleep(200 * time.Millisecond)
	}

	kycRemaining, err := kyc.KeyFingerprintsInUse(ctx)
	if err != nil {
		logger.Error("read KYC progress", "error", err)
		os.Exit(1)
	}
	payoutRemaining, err := payouts.DestinationKeyFingerprintsInUse(ctx)
	if err != nil {
		logger.Error("read refund payout progress", "error", err)
		os.Exit(1)
	}
	stillOld := kycRemaining[rotator.FromFingerprint()] + payoutRemaining[rotator.FromFingerprint()]
	legacyUnstamped := payoutRemaining[""]
	logger.Info("rotation finished",
		"kyc_rotated", kycTotal, "refund_payouts_rotated", payoutTotal,
		"still_on_old_key", stillOld, "legacy_unstamped_payouts", legacyUnstamped,
		"kyc_records_by_key", kycRemaining, "refund_payouts_by_key", payoutRemaining)
	if stillOld > 0 {
		logger.Error("records still hold the old key; do not destroy it yet", "count", stillOld)
		os.Exit(1)
	}
	if legacyUnstamped > 0 {
		logger.Error("encrypted payout destinations still have no key fingerprint; do not destroy the old key yet", "count", legacyUnstamped)
		os.Exit(1)
	}
	logger.Info("the old key is no longer needed by any stored record")
}
