package main

import "testing"

func TestOriginMatcherCanonicalRootAndTenantSubdomains(t *testing.T) {
	matcher := newOriginMatcher("https://tawafiqhub.id")

	allowed := []string{
		"https://tawafiqhub.id",
		"https://vacana.tawafiqhub.id",
		"https://app.tawafiqhub.id",
	}
	for _, origin := range allowed {
		if !matcher.allows(origin) {
			t.Errorf("expected %q to be allowed", origin)
		}
	}

	rejected := []string{
		"http://tawafiqhub.id",
		"https://evil.example",
		"https://nested.vacana.tawafiqhub.id",
	}
	for _, origin := range rejected {
		if matcher.allows(origin) {
			t.Errorf("expected %q to be rejected", origin)
		}
	}
}
