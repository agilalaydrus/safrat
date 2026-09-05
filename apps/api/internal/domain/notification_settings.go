package domain

// NotificationSettings is one operator's quiet-hours window and per-event
// push toggles. Times are minutes since midnight, the operator's own local
// wall-clock — not UTC.
type NotificationSettings struct {
	OperatorID               string
	QuietHoursEnabled        bool
	QuietHoursStartMinutes   int32
	QuietHoursEndMinutes     int32
	NotifyGroupCityChange    bool
	NotifyKloterStatusChange bool
	NotifyRitualBulkComplete bool
}

// InQuietHours reports whether nowMinutes (minutes since midnight, local
// time) falls inside the window. A window that wraps past midnight (e.g.
// 22:00-06:00, the default) is handled by treating "inside" as "at or after
// start, OR before end" rather than "at or after start AND before end" —
// the ordinary between-two-times test silently means nothing for a window
// that crosses midnight.
func (s NotificationSettings) InQuietHours(nowMinutes int32) bool {
	if !s.QuietHoursEnabled {
		return false
	}
	start, end := s.QuietHoursStartMinutes, s.QuietHoursEndMinutes
	if start == end {
		return false // a zero-width window mutes nothing, not everything
	}
	if start < end {
		return nowMinutes >= start && nowMinutes < end
	}
	return nowMinutes >= start || nowMinutes < end
}
