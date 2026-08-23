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
