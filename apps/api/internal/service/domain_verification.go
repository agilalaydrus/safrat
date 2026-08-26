package service

import (
	"context"
	"crypto/subtle"
	"net"
	"strings"
)

// DomainVerificationPrefix is the DNS label an operator publishes the token
// under. A dedicated subdomain keeps the record from colliding with SPF, DKIM,
// or anything else already at the apex.
const DomainVerificationPrefix = "_tawafiqhub-verification"

// DNSTXTLookup is the DNS dependency, kept as an interface so verification can
// be tested without touching the network.
type DNSTXTLookup func(ctx context.Context, name string) ([]string, error)

// LookupTXT is the production lookup.
func LookupTXT(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}

// verifyDomainToken reports whether the expected token is published under the
// verification record for hostname.
//
// The comparison is constant time and exact. Ownership of a domain is what
// authorises us to serve it, put it in a CORS allowlist, and obtain a
// certificate for it, so a loose match here would be the weakest link in all
// three.
func verifyDomainToken(ctx context.Context, lookup DNSTXTLookup, hostname, expected string) (bool, error) {
	if lookup == nil || hostname == "" || expected == "" {
		return false, nil
	}
	records, err := lookup(ctx, DomainVerificationPrefix+"."+hostname)
	if err != nil {
		return false, err
	}
	for _, record := range records {
		// Resolvers may split long TXT values into chunks and some providers
		// wrap the value in quotes; join and trim before comparing.
		candidate := strings.TrimSpace(strings.Trim(strings.TrimSpace(record), `"`))
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1 {
			return true, nil
		}
	}
	return false, nil
}
