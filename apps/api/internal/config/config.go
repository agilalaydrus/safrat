package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port                       string
	DatabaseURL                string
	BetterAuthSecret           string
	AllowedOrigin              string
	SentryDSN                  string
	FirebaseServiceAccountJSON string
	XenditSecretKey            string
	XenditWebhookToken         string
	S3Endpoint                 string
	S3Region                   string
	S3Bucket                   string
	S3AccessKeyID              string
	S3SecretAccessKey          string
	S3PublicBaseURL            string
	S3ForcePathStyle           bool
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
		S3Endpoint:         firstValue("S3_ENDPOINT", r2Endpoint()),
		S3Region:           value("S3_REGION", "auto"),
		S3Bucket:           firstValue("S3_BUCKET", strings.TrimSpace(os.Getenv("R2_BUCKET_NAME"))),
		S3AccessKeyID:      firstValue("S3_ACCESS_KEY_ID", strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))),
		S3SecretAccessKey:  firstValue("S3_SECRET_ACCESS_KEY", strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))),
		S3PublicBaseURL:    strings.TrimRight(firstValue("S3_PUBLIC_BASE_URL", strings.TrimSpace(os.Getenv("R2_PUBLIC_BASE_URL"))), "/"),
		S3ForcePathStyle:   strings.EqualFold(value("S3_FORCE_PATH_STYLE", "true"), "true"),
	}
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

func firstValue(key, fallback string) string {
	if current := strings.TrimSpace(os.Getenv(key)); current != "" {
		return current
	}
	return fallback
}

func r2Endpoint() string {
	accountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
	if accountID == "" {
		return ""
	}
	return "https://" + accountID + ".r2.cloudflarestorage.com"
}
