package storage

import (
	"os"
	"strings"
)

// ConfigFromEnv resolves the storefront object storage settings from the
// environment. It is the single source of truth for that resolution: the API
// server, the cleanup worker, and the one-time backfill command all read the
// same variables, including the legacy R2_* fallbacks kept for deployments
// that predate the move to self-hosted MinIO.
func ConfigFromEnv() Config {
	return Config{
		Endpoint:        firstValue("S3_ENDPOINT", r2Endpoint()),
		Region:          firstValue("S3_REGION", "auto"),
		Bucket:          firstValue("S3_BUCKET", environmentValue("R2_BUCKET_NAME")),
		AccessKeyID:     firstValue("S3_ACCESS_KEY_ID", environmentValue("R2_ACCESS_KEY_ID")),
		SecretAccessKey: firstValue("S3_SECRET_ACCESS_KEY", environmentValue("R2_SECRET_ACCESS_KEY")),
		PublicBaseURL:   strings.TrimRight(firstValue("S3_PUBLIC_BASE_URL", environmentValue("R2_PUBLIC_BASE_URL")), "/"),
		ForcePathStyle:  strings.EqualFold(firstValue("S3_FORCE_PATH_STYLE", "true"), "true"),
	}
}

func environmentValue(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func firstValue(key, fallback string) string {
	if current := environmentValue(key); current != "" {
		return current
	}
	return fallback
}

func r2Endpoint() string {
	accountID := environmentValue("R2_ACCOUNT_ID")
	if accountID == "" {
		return ""
	}
	return "https://" + accountID + ".r2.cloudflarestorage.com"
}
