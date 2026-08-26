package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/hibiken/asynq"
)

type fakeSubscriptionSweeper struct {
	expired, lapsed         int64
	expireErr, lapseErr     error
	expireCalls, lapseCalls int
}

func (f *fakeSubscriptionSweeper) ExpireOverdueInvoices(context.Context) (int64, error) {
	f.expireCalls++
	return f.expired, f.expireErr
}

func (f *fakeSubscriptionSweeper) MarkLapsed(context.Context) (int64, error) {
	f.lapseCalls++
	return f.lapsed, f.lapseErr
}

func TestSubscriptionSweepReleasesAmountsAndMarksLapsed(t *testing.T) {
	sweeper := &fakeSubscriptionSweeper{expired: 3, lapsed: 2}
	handler := &SubscriptionHandler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sweeper: sweeper}

	if err := handler.HandleSweep(context.Background(), asynq.NewTask(TaskSubscriptionSweep, nil)); err != nil {
		t.Fatalf("HandleSweep: %v", err)
	}
	if sweeper.expireCalls != 1 || sweeper.lapseCalls != 1 {
		t.Fatalf("expire=%d lapse=%d, want one each", sweeper.expireCalls, sweeper.lapseCalls)
	}
}

// A failure must not be returned: asynq would retry, and the next hourly tick
// does the same work anyway.
func TestSubscriptionSweepSwallowsFailures(t *testing.T) {
	sweeper := &fakeSubscriptionSweeper{expireErr: errors.New("database unavailable")}
	handler := &SubscriptionHandler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sweeper: sweeper}
	if err := handler.HandleSweep(context.Background(), asynq.NewTask(TaskSubscriptionSweep, nil)); err != nil {
		t.Fatalf("HandleSweep returned %v, want nil", err)
	}
	// Expiring failed, so marking lapsed must not run on a half-swept state.
	if sweeper.lapseCalls != 0 {
		t.Fatal("continued after the expiry sweep failed")
	}
}
