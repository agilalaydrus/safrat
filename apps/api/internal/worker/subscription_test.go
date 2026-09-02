package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hajj-saas/api/internal/repository"

	"github.com/hibiken/asynq"
)

type fakeSubscriptionSweeper struct {
	expired, lapsed         int64
	expireErr, lapseErr     error
	expireCalls, lapseCalls int

	due        []repository.RenewalDue
	dueErr     error
	issued     []string
	issueErr   error
	staleCount int
	staleTotal int64
}

func (f *fakeSubscriptionSweeper) ExpireOverdueInvoices(context.Context) (int64, error) {
	f.expireCalls++
	return f.expired, f.expireErr
}

func (f *fakeSubscriptionSweeper) MarkLapsed(context.Context) (int64, error) {
	f.lapseCalls++
	return f.lapsed, f.lapseErr
}

func (f *fakeSubscriptionSweeper) ListDueForRenewal(context.Context, time.Duration) ([]repository.RenewalDue, error) {
	return f.due, f.dueErr
}

func (f *fakeSubscriptionSweeper) IssueBillingPeriod(_ context.Context, operatorID, plan string, _ time.Time, _ int64, _ string) (repository.Invoice, string, bool, error) {
	if f.issueErr != nil {
		return repository.Invoice{}, "", false, f.issueErr
	}
	f.issued = append(f.issued, operatorID+"/"+plan)
	return repository.Invoice{Amount: 1_500_007}, "Travel", true, nil
}

func (f *fakeSubscriptionSweeper) CountStaleUnmatched(context.Context, time.Duration) (int, int64, error) {
	return f.staleCount, f.staleTotal, nil
}

// Nothing billed for the next period: the sweep expired invoices and marked
// subscriptions lapsed, and never asked anybody to pay. A subscription simply
// stopped and revenue ended without anybody deciding it should.
func TestSweepIssuesRenewalInvoices(t *testing.T) {
	sweeper := &fakeSubscriptionSweeper{due: []repository.RenewalDue{
		{OperatorID: "op-1", Plan: "STARTER", PeriodStart: time.Now(), BaseAmount: 589000},
		{OperatorID: "op-2", Plan: "GROWTH", PeriodStart: time.Now(), BaseAmount: 789000},
	}}
	handler := &SubscriptionHandler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sweeper: sweeper}

	if err := handler.HandleSweep(context.Background(), asynq.NewTask(TaskSubscriptionSweep, nil)); err != nil {
		t.Fatalf("HandleSweep: %v", err)
	}
	if len(sweeper.issued) != 2 {
		t.Fatalf("%d tagihan diterbitkan, mau 2: %v", len(sweeper.issued), sweeper.issued)
	}
}

// One operator failing must not stop the rest — every one skipped is one
// nobody is billing.
func TestOneFailedRenewalDoesNotStopTheSweep(t *testing.T) {
	sweeper := &fakeSubscriptionSweeper{
		due:      []repository.RenewalDue{{OperatorID: "op-1", Plan: "STARTER", PeriodStart: time.Now(), BaseAmount: 589000}},
		issueErr: errors.New("gagal"),
		expired:  1,
	}
	handler := &SubscriptionHandler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sweeper: sweeper}

	if err := handler.HandleSweep(context.Background(), asynq.NewTask(TaskSubscriptionSweep, nil)); err != nil {
		t.Fatalf("penerbitan yang gagal menghentikan sweep: %v", err)
	}
	if sweeper.lapseCalls != 1 {
		t.Fatal("sisa sweep tidak berjalan setelah satu penerbitan gagal")
	}
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
