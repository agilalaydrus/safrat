package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port             string
	DatabaseURL      string
	BetterAuthSecret string
	AllowedOrigin    string
}

func Load() (Config, error) {
	config := Config{
		Port:             value("PORT", "8080"),
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		BetterAuthSecret: strings.TrimSpace(os.Getenv("BETTER_AUTH_SECRET")),
		AllowedOrigin:    strings.TrimRight(strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGIN")), "/"),
	}
	if config.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
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
