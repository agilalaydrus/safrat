package service

import "testing"

func TestRupiah(t *testing.T) {
	for _, testCase := range []struct {
		amount int64
		want   string
	}{
		{0, "Rp0"},
		{7, "Rp7"},
		{999, "Rp999"},
		{1_000, "Rp1.000"},
		{4_200_000, "Rp4.200.000"},
		{4_500_000, "Rp4.500.000"},
		{1_234_567_890, "Rp1.234.567.890"},
		// Reversals and clawbacks are stored negative; the sign belongs
		// outside the currency, not between "Rp" and the digits.
		{-1_000, "-Rp1.000"},
		{-4_500_000, "-Rp4.500.000"},
	} {
		if got := rupiah(testCase.amount); got != testCase.want {
			t.Errorf("rupiah(%d) = %q, want %q", testCase.amount, got, testCase.want)
		}
	}
}
