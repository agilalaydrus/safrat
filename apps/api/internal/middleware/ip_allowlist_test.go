package middleware

import "testing"

func TestIPAllowed(t *testing.T) {
	cases := []struct {
		name  string
		ip    string
		cidrs []string
		want  bool
	}{
		{"inside range", "198.51.100.42", []string{"198.51.100.0/24"}, true},
		{"outside range", "203.0.113.5", []string{"198.51.100.0/24"}, false},
		{"empty list never matches", "198.51.100.42", []string{}, false},
		{"malformed CIDR skipped, not treated as match-all", "198.51.100.42", []string{"not-a-cidr"}, false},
		{"malformed CIDR alongside a real one still matches the real one", "198.51.100.42", []string{"not-a-cidr", "198.51.100.0/24"}, true},
		{"unparseable caller IP never matches", "not-an-ip", []string{"0.0.0.0/0"}, false},
		{"IPv6 range", "2001:db8::1", []string{"2001:db8::/32"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ipAllowed(tc.ip, tc.cidrs); got != tc.want {
				t.Fatalf("ipAllowed(%q, %v) = %v, mau %v", tc.ip, tc.cidrs, got, tc.want)
			}
		})
	}
}
