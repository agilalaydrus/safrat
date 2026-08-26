package config

import "testing"

func validEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("BETTER_AUTH_SECRET", "test-secret")
	t.Setenv("CORS_ALLOWED_ORIGIN", "http://localhost:3131")
}

func TestLoadDefaultsStorefrontStorageQuota(t *testing.T) {
	validEnvironment(t)
	t.Setenv("STOREFRONT_STORAGE_QUOTA_MB", "")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := int64(250 * 1024 * 1024); config.StorefrontStorageQuotaBytes != want {
		t.Fatalf("quota = %d, want %d", config.StorefrontStorageQuotaBytes, want)
	}
}

func TestLoadRejectsUnsafeStorefrontStorageQuota(t *testing.T) {
	validEnvironment(t)
	for _, value := range []string{"invalid", "24", "10241"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("STOREFRONT_STORAGE_QUOTA_MB", value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted quota %q", value)
			}
		})
	}
}
