// Package funnel turns a request into an anonymous, day-scoped visitor token,
// and decides whether a request came from a person at all.
package funnel

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"
	"time"
)

// Hasher builds visitor tokens.
//
// The token is SHA256 over a secret salt, the calendar day, the client address
// and the user agent. Three properties follow, and all three are the reason
// this table is not personal data:
//
//   - It cannot be reversed to an address. The IPv4 space is small enough to
//     enumerate, so without the salt a leaked dump could be turned back into a
//     list of addresses — which is why the salt lives in the environment and
//     never in the database beside the hashes it protects.
//   - It changes at midnight, so nobody can be followed from one day to the
//     next, including by us.
//   - It needs no cookie, so a storefront needs no consent banner — and a
//     consent banner is a conversion cost the agency pays.
type Hasher struct {
	salt []byte
	// now is injectable so the day-boundary behaviour can be tested rather
	// than assumed.
	now func() time.Time
}

func NewHasher(salt string) *Hasher {
	return &Hasher{salt: []byte(salt), now: time.Now}
}

// Configured reports whether a usable salt was supplied. Without one the
// tokens would be reversible, so recording is skipped entirely rather than
// writing something that only looks anonymous.
func (h *Hasher) Configured() bool { return h != nil && len(h.salt) >= 16 }

// Visitor returns the token for one request, or "" when no salt is set.
func (h *Hasher) Visitor(remoteAddr, userAgent string) string {
	if !h.Configured() {
		return ""
	}
	sum := sha256.New()
	sum.Write(h.salt)
	sum.Write([]byte(h.now().UTC().Format("2006-01-02")))
	sum.Write([]byte{0})
	sum.Write([]byte(normaliseAddress(remoteAddr)))
	sum.Write([]byte{0})
	sum.Write([]byte(strings.TrimSpace(userAgent)))
	return hex.EncodeToString(sum.Sum(nil))
}

// normaliseAddress strips a port and squares up IPv6 so the same client
// produces the same token across requests.
func normaliseAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	if ip := net.ParseIP(value); ip != nil {
		return ip.String()
	}
	return value
}

// botMarkers are substrings that appear in agents which announce themselves.
//
// Nothing here catches a crawler that lies, and it is not meant to. The point
// is the honest majority: without any filter a small site's traffic can be
// mostly machines, and a funnel that counts them reports a conversion rate far
// worse than the truth — which is then used to make decisions.
var botMarkers = []string{
	"bot", "crawler", "spider", "slurp", "curl", "wget", "python-requests",
	"headlesschrome", "phantomjs", "lighthouse", "pingdom", "uptime",
	"facebookexternalhit", "whatsapp", "telegrambot", "preview", "monitor",
	"scrapy", "httpclient", "go-http-client", "postman", "insomnia",
}

// IsBot reports whether a user agent identifies itself as an automated client.
// An empty agent counts as one: every real browser sends it.
func IsBot(userAgent string) bool {
	agent := strings.ToLower(strings.TrimSpace(userAgent))
	if agent == "" {
		return true
	}
	for _, marker := range botMarkers {
		if strings.Contains(agent, marker) {
			return true
		}
	}
	return false
}
