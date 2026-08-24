package repository

import (
	"strings"
	"testing"
)

func TestSlugBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		operator string
		want     string
	}{
		{name: "drops PT prefix", operator: "PT Vacana Indonesia", want: "vacana-indonesia"},
		{name: "drops punctuated PT prefix", operator: "PT. Safrat Teknologi Nusantara", want: "safrat-teknologi-nusantara"},
		{name: "drops CV prefix", operator: "CV Barokah Tour & Travel", want: "barokah-tour-travel"},
		{name: "drops KBIHU prefix", operator: "KBIHU Al-Hikmah", want: "al-hikmah"},
		{name: "uses full brand name", operator: "Vacana Tour", want: "vacana-tour"},
		{name: "keeps prefix when it is the whole name", operator: "PT", want: "pt"},
		{name: "empty punctuation", operator: "!!!", want: ""},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := slugBase(test.operator); got != test.want {
				t.Fatalf("slugBase(%q) = %q, want %q", test.operator, got, test.want)
			}
		})
	}
}

func TestSlugBaseFitsDNSLabelWithSuffix(t *testing.T) {
	t.Parallel()

	got := slugBase(strings.Repeat("operator ", 20))
	if len(got) > operatorSlugBaseMaxLength {
		t.Fatalf("slug length = %d, want <= %d", len(got), operatorSlugBaseMaxLength)
	}
	if strings.HasSuffix(got, "-") {
		t.Fatalf("slug %q must not end in a hyphen", got)
	}
}

func TestIsValidOperatorSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		slug string
		want bool
	}{
		{slug: "sinar", want: true},
		{slug: "sinar-bukit-shofa", want: true},
		{slug: "ab", want: false},
		{slug: "-sinar", want: false},
		{slug: "sinar-", want: false},
		{slug: "Sinar", want: false},
		{slug: "sinar_bukit", want: false},
		{slug: strings.Repeat("a", 64), want: false},
	}

	for _, test := range tests {
		if got := IsValidOperatorSlug(test.slug); got != test.want {
			t.Errorf("IsValidOperatorSlug(%q) = %v, want %v", test.slug, got, test.want)
		}
	}
}

func TestReservedOperatorSlugsCannotBeUsed(t *testing.T) {
	t.Parallel()

	reserved := []string{
		"admin", "api", "app", "auth", "dashboard",
		"docs", "help", "status", "support", "www",
	}
	for _, slug := range reserved {
		if !IsValidOperatorSlug(slug) {
			t.Errorf("reserved slug %q should remain syntactically valid", slug)
		}
		if !IsReservedOperatorSlug(slug) {
			t.Errorf("expected %q to be reserved", slug)
		}
		if IsUsableOperatorSlug(slug) {
			t.Errorf("reserved slug %q must not be usable", slug)
		}
	}

	if !IsUsableOperatorSlug("vacana-indonesia") {
		t.Error("ordinary DNS-safe operator slug should be usable")
	}
	if IsReservedOperatorSlug("vacana-indonesia") {
		t.Error("ordinary operator slug must not be reserved")
	}
}
