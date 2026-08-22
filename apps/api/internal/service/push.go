package service

import "context"

// PushNotifier decouples cascade-triggering services (Kloter/Group/Ritual/
// HealthReport) from the Firebase-specific implementation
// (internal/notification) — nil is a valid no-op notifier for local dev
// without FIREBASE_SERVICE_ACCOUNT_JSON set, same pattern as SOSNotifier.
type PushNotifier interface {
	NotifyGroupPilgrims(ctx context.Context, operatorID, groupID, title, body string)
	NotifyKloterPilgrims(ctx context.Context, operatorID, kloterID, title, body string)
	NotifyOperatorStaff(ctx context.Context, operatorID, title, body string)
}
