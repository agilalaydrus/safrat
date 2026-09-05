package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/hajj-saas/api/internal/storage"
)

type Config struct {
	Port                        string
	DatabaseURL                 string
	BetterAuthSecret            string
	AllowedOrigin               string
	SentryDSN                   string
	FirebaseServiceAccountJSON  string
	XenditSecretKey             string
	XenditWebhookToken          string
	StorefrontStorage           storage.Config
	StorefrontStorageQuotaBytes int64
	GeoIPDBPath                 string
	AuditExportSigningKey       string
}

func Load() (Config, error) {
	config := Config{
		Port:             value("PORT", "8080"),
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		BetterAuthSecret: strings.TrimSpace(os.Getenv("BETTER_AUTH_SECRET")),
		AllowedOrigin:    strings.TrimRight(strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGIN")), "/"),
		// SentryDSN is optional — unset means sentry.Init is a no-op (see main.go).
		SentryDSN: strings.TrimSpace(os.Getenv("SENTRY_DSN")),
		// FirebaseServiceAccountJSON is optional — unset means SOS push notifications
		// are a no-op (see internal/notification.NewFirebasePusher).
		FirebaseServiceAccountJSON: strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")),
		// XenditSecretKey is optional at startup but not at checkout time —
		// unlike Firebase/Sentry's silent no-op, OrderService.CreateOrder
		// returns a clear FailedPrecondition instead of pretending a
		// payment was created (see internal/payment/xendit.go).
		XenditSecretKey:    strings.TrimSpace(os.Getenv("XENDIT_SECRET_KEY")),
		XenditWebhookToken: strings.TrimSpace(os.Getenv("XENDIT_WEBHOOK_TOKEN")),
		// Resolved by the storage package so the server, the cleanup worker,
		// and the backfill command all read these the same way.
		StorefrontStorage: storage.ConfigFromEnv(),
		// GeoIPDBPath is optional — unset means visitor city/province stay
		// empty (see internal/geoip.Open). The file itself is downloaded and
		// kept current by cmd/worker (internal/worker/geoip_refresh.go), not
		// shipped in the repo or fetched inline on the request path.
		GeoIPDBPath: value("GEOIP_DB_PATH", ""),
		// AuditExportSigningKey is optional — unset means
		// PlatformService.ExportAuditTrail refuses with a clear error instead
		// of exporting something unsigned (see internal/crypto.NewSigner).
		AuditExportSigningKey: strings.TrimSpace(os.Getenv("AUDIT_EXPORT_SIGNING_KEY")),
	}
	quotaMB, err := strconv.ParseInt(value("STOREFRONT_STORAGE_QUOTA_MB", "250"), 10, 64)
	if err != nil || quotaMB < 25 || quotaMB > 10240 {
		return Config{}, errors.New("STOREFRONT_STORAGE_QUOTA_MB must be between 25 and 10240")
	}
	config.StorefrontStorageQuotaBytes = quotaMB * 1024 * 1024
	// DatabaseURL is optional — when unset, pgxpool.New in main.go is called
	// with an empty string, which pgx resolves from PGHOST/PGPORT/PGUSER/
	// PGPASSWORD/PGDATABASE directly (no URL parsing at all). Kept as a
	// convenience for local dev, where a simple password never breaks URL
	// parsing; production sets PG* vars instead — see docker-compose.prod.yml.
	if config.BetterAuthSecret == "" {
		return Config{}, errors.New("BETTER_AUTH_SECRET is required")
	}
	if config.AllowedOrigin == "" {
		return Config{}, errors.New("CORS_ALLOWED_ORIGIN is required")
	}
	if _, err := url.ParseRequestURI(config.AllowedOrigin); err != nil {
		return Config{}, errors.New("CORS_ALLOWED_ORIGIN must be a valid URL")
	}
	return config, nil
}

func value(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
