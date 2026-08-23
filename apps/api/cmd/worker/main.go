package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/notification"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/hajj-saas/api/internal/worker"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// DATABASE_URL is optional — pgxpool.New with an empty string resolves
	// from PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE instead (no URL
	// parsing, so a password with special characters can't break it).
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		logger.Error("invalid configuration", "error", "REDIS_URL is required")
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := db.New(pool)
	operatorRepository := repository.NewOperatorRepository(queries)
	agentRepository := repository.NewAgentRepository(queries)
	pilgrimRepository := repository.NewPilgrimRepository(queries)
	sosRepository := repository.NewSOSRepository(queries)
	waitlistRepository := repository.NewWaitlistRepository(queries)
	notificationRepository := repository.NewNotificationRepository(queries)
	auditRepository := repository.NewAuditRepository(queries)
	outboxRepository := repository.NewOutboxRepository(queries)
	agentService := service.NewAgentService(operatorRepository, agentRepository, auditRepository, pool)
	tierHandler := worker.NewTierHandler(logger, operatorRepository, agentService)

	firebasePusher, err := notification.NewFirebasePusher(context.Background(), logger, strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")), notificationRepository)
	if err != nil {
		logger.Error("init firebase", "error", err)
	}
	// eventBus is nil here — the worker is a separate process from the API
	// server, so it has no monitoring-dashboard subscribers to publish to
	// (those live in the server's in-process events.Bus). Bus.Publish is a
	// documented nil-safe no-op for exactly this case.
	sosService := service.NewSOSService(operatorRepository, pilgrimRepository, sosRepository, auditRepository, firebasePusher, nil)
	sosHandler := worker.NewSOSHandler(logger, sosService)
	waitlistHandler := worker.NewWaitlistHandler(logger, waitlistRepository)
	cashFlowHandler := worker.NewCashFlowHandler(logger, queries)
	outboxHandler := worker.NewOutboxHandler(logger, outboxRepository, firebasePusher)

	redisOpt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		logger.Error("parse REDIS_URL", "error", err)
		os.Exit(1)
	}

	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{Logger: slogAdapter{logger}})
	if _, err := scheduler.Register("@every 5m", worker.NewTierRecalculateAllTask()); err != nil {
		logger.Error("register tier recalculation schedule", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.Register("@every 1m", worker.NewSOSEscalateTask()); err != nil {
		logger.Error("register SOS escalation schedule", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.Register("@every 5m", worker.NewWaitlistExpireTask()); err != nil {
		logger.Error("register waitlist expiration schedule", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.Register("@every 1h", worker.NewMarkOverdueVendorPaymentsTask()); err != nil {
		logger.Error("register vendor payment overdue schedule", "error", err)
		os.Exit(1)
	}
	// Drains the cascade_events outbox frequently — this is what makes
	// producer-side side-effects (e.g. health BERAT push) actually fire.
	if _, err := scheduler.Register("@every 10s", worker.NewCascadeDispatchTask()); err != nil {
		logger.Error("register cascade dispatch schedule", "error", err)
		os.Exit(1)
	}
	go func() {
		if err := scheduler.Run(); err != nil {
			logger.Error("scheduler stopped", "error", err)
			os.Exit(1)
		}
	}()

	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TaskTierRecalculateAll, tierHandler.HandleRecalculateAll)
	mux.HandleFunc(worker.TaskSOSEscalate, sosHandler.HandleEscalate)
	mux.HandleFunc(worker.TaskWaitlistExpire, waitlistHandler.HandleExpire)
	mux.HandleFunc(worker.TaskMarkOverdueVendorPayments, cashFlowHandler.HandleMarkOverdue)
	mux.HandleFunc(worker.TaskCascadeDispatch, outboxHandler.HandleDispatch)

	server := asynq.NewServer(redisOpt, asynq.Config{Concurrency: 5, Logger: slogAdapter{logger}})
	logger.Info("worker listening", "redis", redisURL)
	if err := server.Run(mux); err != nil {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

// slogAdapter satisfies asynq's minimal logger interface with our existing slog.Logger.
type slogAdapter struct{ logger *slog.Logger }

func (a slogAdapter) Debug(args ...interface{}) { a.logger.Debug(fmt.Sprint(args...)) }
func (a slogAdapter) Info(args ...interface{})  { a.logger.Info(fmt.Sprint(args...)) }
func (a slogAdapter) Warn(args ...interface{})  { a.logger.Warn(fmt.Sprint(args...)) }
func (a slogAdapter) Error(args ...interface{}) { a.logger.Error(fmt.Sprint(args...)) }
func (a slogAdapter) Fatal(args ...interface{}) { a.logger.Error(fmt.Sprint(args...)); os.Exit(1) }
