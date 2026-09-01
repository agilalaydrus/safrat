package service

import (
	"testing"
	"time"
)

func TestBuildInstallmentSchedulePreservesEveryRupiahAndClampsMonthEnd(t *testing.T) {
	firstDue := time.Date(2027, time.January, 31, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		scheme  string
		count   int
		payable int64
	}{
		{name: "full", scheme: "FULL", count: 1, payable: 35_000_001},
		{name: "dp", scheme: "DP_50", count: 2, payable: 35_000_001},
		{name: "six", scheme: "INSTALLMENT_6X", count: 6, payable: 35_000_001},
		{name: "twelve", scheme: "INSTALLMENT_12X", count: 12, payable: 35_000_001},
		{name: "cash bonus", scheme: "CASH_BONUS", count: 1, payable: 34_500_000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := buildInstallmentSchedule(test.scheme, test.count, test.payable, firstDue)
			if len(items) != test.count {
				t.Fatalf("jumlah jadwal = %d, mau %d", len(items), test.count)
			}
			var total int64
			for index, item := range items {
				if item.Number != int32(index+1) || item.AmountDueIDR <= 0 {
					t.Fatalf("jadwal %d tidak valid: %#v", index, item)
				}
				total += item.AmountDueIDR
			}
			if total != test.payable {
				t.Fatalf("jumlah rupiah = %d, mau %d", total, test.payable)
			}
		})
	}

	dp := buildInstallmentSchedule("DP_50", 2, 35_000_001, firstDue)
	if dp[0].AmountDueIDR != 17_500_001 || dp[1].AmountDueIDR != 17_500_000 {
		t.Fatalf("pembulatan DP salah: %#v", dp)
	}
	if got := dp[1].DueDate.Format("2006-01-02"); got != "2027-02-28" {
		t.Fatalf("jatuh tempo akhir bulan bergeser ke %s", got)
	}
}
