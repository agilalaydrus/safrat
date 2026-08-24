package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/hajj-saas/api/internal/domain"
)

type fakeCascadePusher struct {
	groupID  string
	kloterID string
	body     string
	err      error
}

func (f *fakeCascadePusher) NotifyOperatorStaff(context.Context, string, string, string) error {
	return f.err
}
func (f *fakeCascadePusher) NotifyGroupPilgrims(_ context.Context, _ string, groupID, _, body string) error {
	f.groupID, f.body = groupID, body
	return f.err
}
func (f *fakeCascadePusher) NotifyKloterPilgrims(_ context.Context, _ string, kloterID, _, body string) error {
	f.kloterID, f.body = kloterID, body
	return f.err
}

type fakeJourneyCascader struct {
	groupID, kloterID, status, updatedBy, notes string
}

func (f *fakeJourneyCascader) BulkUpdateForGroupAs(_ context.Context, _, groupID, status, updatedBy, notes string) (int32, error) {
	f.groupID, f.status, f.updatedBy, f.notes = groupID, status, updatedBy, notes
	return 2, nil
}
func (f *fakeJourneyCascader) BulkUpdateForKloterAs(_ context.Context, _, kloterID, status, updatedBy, notes string) (int32, error) {
	f.kloterID, f.status, f.updatedBy, f.notes = kloterID, status, updatedBy, notes
	return 3, nil
}

func testOutboxHandler(push OperatorPusher, journeys JourneyCascader) *OutboxHandler {
	return &OutboxHandler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), push: push, journeys: journeys}
}

func TestDispatchGroupCityRunsJourneyBeforePush(t *testing.T) {
	push := &fakeCascadePusher{}
	journeys := &fakeJourneyCascader{}
	payload, _ := json.Marshal(domain.GroupCityUpdatedPayload{
		GroupID: "group-1", JourneyStatus: "IN_MAKKAH", UpdatedBy: "user-1", Notes: "arrived", NotificationBody: "Grup Anda kini di Makkah",
	})
	err := testOutboxHandler(push, journeys).dispatch(context.Background(), domain.CascadeEvent{OperatorID: "operator-1", EventType: domain.EventGroupCityUpdated, Payload: payload})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if journeys.groupID != "group-1" || journeys.status != "IN_MAKKAH" || journeys.updatedBy != "user-1" {
		t.Fatalf("unexpected journey cascade: %+v", journeys)
	}
	if push.groupID != "group-1" || push.body != "Grup Anda kini di Makkah" {
		t.Fatalf("unexpected push: %+v", push)
	}
}

func TestDispatchReturnsPushFailureForRetry(t *testing.T) {
	want := errors.New("firebase unavailable")
	push := &fakeCascadePusher{err: want}
	payload, _ := json.Marshal(domain.RitualBulkCompletedPayload{GroupID: "group-1", NotificationBody: "done"})
	err := testOutboxHandler(push, &fakeJourneyCascader{}).dispatch(context.Background(), domain.CascadeEvent{OperatorID: "operator-1", EventType: domain.EventRitualBulkCompleted, Payload: payload})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

func TestDispatchRejectsUnknownEvent(t *testing.T) {
	if err := testOutboxHandler(nil, nil).dispatch(context.Background(), domain.CascadeEvent{EventType: "unknown"}); err == nil {
		t.Fatal("expected unsupported event error")
	}
}
