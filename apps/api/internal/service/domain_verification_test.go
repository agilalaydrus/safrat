package service

import (
	"context"
	"errors"
	"testing"
)

func TestVerifyDomainToken(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef"
	ctx := context.Background()

	cases := map[string]struct {
		records []string
		want    bool
	}{
		"exact match":             {[]string{token}, true},
		"quoted by the provider":  {[]string{`"` + token + `"`}, true},
		"surrounded by spaces":    {[]string{"  " + token + "  "}, true},
		"alongside other records": {[]string{"v=spf1 -all", token}, true},
		"no records":              {nil, false},
		"different token":         {[]string{"ffffffffffffffffffffffffffffffff"}, false},
		"prefix only":             {[]string{token[:16]}, false},
		"token with suffix":       {[]string{token + "extra"}, false},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			lookup := func(context.Context, string) ([]string, error) { return testCase.records, nil }
			got, err := verifyDomainToken(ctx, lookup, "umrohvacana.com", token)
			if err != nil {
				t.Fatalf("verifyDomainToken: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("verified = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestVerifyDomainTokenQueriesTheDedicatedRecord(t *testing.T) {
	var asked string
	lookup := func(_ context.Context, name string) ([]string, error) {
		asked = name
		return nil, nil
	}
	if _, err := verifyDomainToken(context.Background(), lookup, "umrohvacana.com", "token"); err != nil {
		t.Fatalf("verifyDomainToken: %v", err)
	}
	if want := DomainVerificationPrefix + ".umrohvacana.com"; asked != want {
		t.Fatalf("looked up %q, want %q", asked, want)
	}
}

func TestVerifyDomainTokenPropagatesLookupFailure(t *testing.T) {
	want := errors.New("dns unavailable")
	lookup := func(context.Context, string) ([]string, error) { return nil, want }
	// A DNS failure must not read as "not verified" to the caller, which would
	// be indistinguishable from a missing record.
	if _, err := verifyDomainToken(context.Background(), lookup, "umrohvacana.com", "token"); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
