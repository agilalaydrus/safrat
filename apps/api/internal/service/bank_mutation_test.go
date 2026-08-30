package service

import (
	"context"
	"testing"
)

type recordingNotifier struct {
	calls []string
}

func (r *recordingNotifier) NotifyOperatorStaff(_ context.Context, operatorID, title, body string) error {
	r.calls = append(r.calls, operatorID+"|"+title+"|"+body)
	return nil
}

// Settlement must not depend on a notification succeeding. The money has
// already moved and the access is already granted; undoing that over a message
// nobody received would be the wrong trade.
func TestNotificationIsOptionalAndNeverBlocks(t *testing.T) {
	// No notifier at all: the common local and self-hosted case.
	silent := &BankMutationService{}
	silent.tellOperator(context.Background(), "op-1", 1_500_000)

	// An empty operator says nothing rather than pushing to nobody.
	notifier := &recordingNotifier{}
	service := &BankMutationService{notifier: notifier}
	service.tellOperator(context.Background(), "", 1_500_000)
	if len(notifier.calls) != 0 {
		t.Fatalf("dikirim tanpa operator: %v", notifier.calls)
	}

	service.tellOperator(context.Background(), "op-1", 1_500_000)
	if len(notifier.calls) != 1 {
		t.Fatalf("%d panggilan, mau 1", len(notifier.calls))
	}
	// The amount is the whole point: "pembayaran diterima" without a figure
	// leaves the reader checking anyway.
	if !contains(notifier.calls[0], "1.500.000") {
		t.Fatalf("pesan tidak menyebut nominal: %s", notifier.calls[0])
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
