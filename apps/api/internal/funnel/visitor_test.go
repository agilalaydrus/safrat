package funnel

import (
	"strings"
	"testing"
	"time"
)

func TestVisitorTokenIsStableWithinADayAndChangesAcrossOne(t *testing.T) {
	hasher := NewHasher(strings.Repeat("s", 32))
	day := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	hasher.now = func() time.Time { return day }

	first := hasher.Visitor("103.150.60.10:44120", "Mozilla/5.0")
	// The same person later the same day must count once, not twice.
	hasher.now = func() time.Time { return day.Add(9 * time.Hour) }
	if same := hasher.Visitor("103.150.60.10", "Mozilla/5.0"); same != first {
		t.Fatal("token berubah dalam satu hari — satu orang akan terhitung dua kali")
	}
	// And tomorrow they must be unrecognisable, including to us. That is what
	// keeps this from being a way to follow somebody.
	hasher.now = func() time.Time { return day.Add(24 * time.Hour) }
	if next := hasher.Visitor("103.150.60.10", "Mozilla/5.0"); next == first {
		t.Fatal("token bertahan lintas hari — orang bisa dilacak")
	}
	if len(first) != 64 {
		t.Fatalf("panjang token %d, mau 64", len(first))
	}
	// A different salt must not produce the same token, or the salt is not
	// doing the one job it has.
	other := NewHasher(strings.Repeat("x", 32))
	other.now = func() time.Time { return day }
	if other.Visitor("103.150.60.10", "Mozilla/5.0") == first {
		t.Fatal("garam tidak memengaruhi token")
	}
}

func TestRecordingIsSkippedWithoutAUsableSalt(t *testing.T) {
	// Refusing to record is the safe failure. A short or missing salt makes the
	// hash reversible from a list of addresses, and a table that only looks
	// anonymous is worse than no table.
	for _, salt := range []string{"", "pendek"} {
		hasher := NewHasher(salt)
		if hasher.Configured() {
			t.Fatalf("garam %q dianggap layak", salt)
		}
		if token := hasher.Visitor("103.150.60.10", "Mozilla/5.0"); token != "" {
			t.Fatalf("token dibuat tanpa garam layak: %q", token)
		}
	}
}

func TestBotsAreRecognisedAndBrowsersAreNot(t *testing.T) {
	bots := []string{
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"curl/8.4.0", "python-requests/2.31", "facebookexternalhit/1.1",
		"Mozilla/5.0 HeadlessChrome/120", "",
	}
	for _, agent := range bots {
		if !IsBot(agent) {
			t.Fatalf("tidak dikenali sebagai bot: %q", agent)
		}
	}
	people := []string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36",
		"Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 Chrome/119.0 Mobile Safari/537.36",
	}
	for _, agent := range people {
		if IsBot(agent) {
			t.Fatalf("pengunjung sungguhan dibuang sebagai bot: %q", agent)
		}
	}
}
