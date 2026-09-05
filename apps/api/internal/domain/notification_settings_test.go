package domain

import "testing"

func TestInQuietHours(t *testing.T) {
	cases := []struct {
		name    string
		s       NotificationSettings
		minutes int32
		want    bool
	}{
		{"disabled never mutes", NotificationSettings{QuietHoursEnabled: false, QuietHoursStartMinutes: 0, QuietHoursEndMinutes: 6 * 60}, 3 * 60, false},
		{"ordinary same-day window, inside", NotificationSettings{QuietHoursEnabled: true, QuietHoursStartMinutes: 9 * 60, QuietHoursEndMinutes: 17 * 60}, 12 * 60, true},
		{"ordinary same-day window, outside", NotificationSettings{QuietHoursEnabled: true, QuietHoursStartMinutes: 9 * 60, QuietHoursEndMinutes: 17 * 60}, 18 * 60, false},
		{"wraps midnight, inside late evening", NotificationSettings{QuietHoursEnabled: true, QuietHoursStartMinutes: 22 * 60, QuietHoursEndMinutes: 6 * 60}, 23 * 60, true},
		{"wraps midnight, inside early morning", NotificationSettings{QuietHoursEnabled: true, QuietHoursStartMinutes: 22 * 60, QuietHoursEndMinutes: 6 * 60}, 3 * 60, true},
		{"wraps midnight, outside at midday", NotificationSettings{QuietHoursEnabled: true, QuietHoursStartMinutes: 22 * 60, QuietHoursEndMinutes: 6 * 60}, 12 * 60, false},
		{"wraps midnight, exactly at end is outside", NotificationSettings{QuietHoursEnabled: true, QuietHoursStartMinutes: 22 * 60, QuietHoursEndMinutes: 6 * 60}, 6 * 60, false},
		{"wraps midnight, exactly at start is inside", NotificationSettings{QuietHoursEnabled: true, QuietHoursStartMinutes: 22 * 60, QuietHoursEndMinutes: 6 * 60}, 22 * 60, true},
		{"zero-width window mutes nothing", NotificationSettings{QuietHoursEnabled: true, QuietHoursStartMinutes: 9 * 60, QuietHoursEndMinutes: 9 * 60}, 9 * 60, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.InQuietHours(tc.minutes); got != tc.want {
				t.Fatalf("InQuietHours(%d) = %v, mau %v (window %d-%d)", tc.minutes, got, tc.want, tc.s.QuietHoursStartMinutes, tc.s.QuietHoursEndMinutes)
			}
		})
	}
}
