package query

import (
	"os"
	"strings"
	"testing"
)

func TestOperatorProfileUpdatePersistsStorefrontBranding(t *testing.T) {
	t.Parallel()

	operatorSQL := readQueryFile(t, "operator.sql")
	updateProfile := namedQuery(t, operatorSQL, "UpdateOperatorProfile")
	for _, column := range []string{
		"brand_color",
		"hero_eyebrow",
		"hero_title",
		"hero_subtitle",
		"hero_image_url",
	} {
		if !strings.Contains(updateProfile, column) {
			t.Errorf("UpdateOperatorProfile must persist %s", column)
		}
	}
}

func TestStorefrontBrandingMigrationProtectsPublicInput(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("../migrations/081_operator_storefront_branding.sql")
	if err != nil {
		t.Fatalf("read storefront migration: %v", err)
	}
	migration := string(contents)
	for _, constraint := range []string{
		"operators_brand_color_hex",
		"operators_hero_eyebrow_length",
		"operators_hero_title_length",
		"operators_hero_subtitle_length",
		"operators_hero_image_url_length",
	} {
		if !strings.Contains(migration, constraint) {
			t.Errorf("storefront migration must define %s", constraint)
		}
	}
}
