package query

import (
	"os"
	"strings"
	"testing"
)

func TestPublicRegistrationUsesStorefrontAvailabilityRule(t *testing.T) {
	t.Parallel()

	seasonSQL := readQueryFile(t, "season.sql")
	registrationSQL := readQueryFile(t, "pilgrim_registration.sql")
	publicSeasons := namedQuery(t, seasonSQL, "ListPublicSeasonsByOperator")
	registrationForm := namedQuery(t, registrationSQL, "GetOperatorSeasonForRegistration")

	const availabilityPredicate = "s.end_date >= NOW()"
	if !strings.Contains(publicSeasons, availabilityPredicate) {
		t.Fatalf("public storefront query must contain %q", availabilityPredicate)
	}
	if !strings.Contains(registrationForm, availabilityPredicate) {
		t.Fatalf("registration form query must contain %q", availabilityPredicate)
	}
	if strings.Contains(registrationForm, "s.is_active = true") {
		t.Fatal("registration availability must not be coupled to the operational active season")
	}
}

func readQueryFile(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(contents)
}

func namedQuery(t *testing.T, contents, name string) string {
	t.Helper()
	marker := "-- name: " + name + " "
	start := strings.Index(contents, marker)
	if start < 0 {
		t.Fatalf("query %s not found", name)
	}
	rest := contents[start:]
	if end := strings.Index(rest[len(marker):], "\n-- name: "); end >= 0 {
		return rest[:len(marker)+end]
	}
	return rest
}
