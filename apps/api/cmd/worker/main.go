package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/hajj-saas/api/internal/crypto"
	"github.com/hajj-saas/api/internal/events"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/mailer"
	"github.com/hajj-saas/api/internal/notification"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/queue"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/hajj-saas/api/internal/storage"
	"github.com/hajj-saas/api/internal/worker"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
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
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("parse REDIS_URL", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOptions)
	defer func() { _ = redisClient.Close() }()
	operatorRepository := repository.NewRedisOperatorRepository(context.Background(), queries, redisClient, logger)
	agentRepository := repository.NewAgentRepository(queries)
	pilgrimRepository := repository.NewPilgrimRepository(queries)
	sosRepository := repository.NewSOSRepository(queries)
	waitlistRepository := repository.NewWaitlistRepository(queries)
	notificationRepository := repository.NewNotificationRepository(queries)
	auditRepository := repository.NewAuditRepository(queries)
	outboxRepository := repository.NewOutboxRepository(queries)
	storefrontAssetRepository := repository.NewStorefrontAssetRepository(pool)
	subscriptionRepository := repository.NewSubscriptionRepository(pool)
	platformRepository := repository.NewPlatformRepository(pool)
	journeyRepository := repository.NewJourneyRepository(queries)
	orderRepository := repository.NewOrderRepository(queries, pool)
	seasonRepository := repository.NewSeasonRepository(queries)
	fulfilmentRepository := repository.NewFulfilmentRepository(pool)
	// The worker migrates legacy identities, so it needs the same key.
	kycSealer, sealerErr := crypto.NewSealer(strings.TrimSpace(os.Getenv("KYC_ENCRYPTION_KEY")))
	if sealerErr != nil {
		logger.Error("read KYC_ENCRYPTION_KEY", "error", sealerErr)
		os.Exit(1)
	}
	repository.SetKYCSealer(kycSealer)
	kycRepository := repository.NewKYCRepository(pool)
	supplierRepository := repository.NewSupplierRepository(pool)
	supplierCostRepository := repository.NewSupplierCostRepository(pool)
	productRepository := repository.NewProductRepository(queries, pool)
	ledgerRepository := repository.NewLedgerRepository(pool)
	refundRepository := repository.NewRefundRepository(pool)
	refundPayoutRepository := repository.NewRefundPayoutRepository(pool, kycSealer)
	if kycSealer != nil {
		payoutsInUse, fingerprintErr := refundPayoutRepository.DestinationKeyFingerprintsInUse(context.Background())
		if fingerprintErr != nil {
			logger.Error("read refund payout key fingerprints", "error", fingerprintErr)
		} else {
			logger.Info("refund payout encryption key loaded",
				"fingerprint", kycSealer.Fingerprint(), "records_by_key", payoutsInUse)
			for fingerprint, count := range payoutsInUse {
				if fingerprint == "" || fingerprint != kycSealer.Fingerprint() {
					logger.Error("refund payout destinations do not match the loaded key",
						"their_fingerprint", fingerprint, "records", count,
						"loaded_fingerprint", kycSealer.Fingerprint(),
						"effect", "those payout destinations cannot be dispatched")
				}
			}
		}
	}
	identityRepository := repository.NewIdentityRepository(queries, agentRepository)
	agentService := service.NewAgentService(operatorRepository, agentRepository, auditRepository, pool)
	journeyService := service.NewJourneyService(operatorRepository, journeyRepository, auditRepository)
	tierHandler := worker.NewTierHandler(logger, operatorRepository, agentService)

	firebasePusher, err := notification.NewFirebasePusher(context.Background(), logger, strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")), notificationRepository)
	if err != nil {
		logger.Error("init firebase", "error", err)
	}
	// Redis pub/sub carries worker-originated escalation/cascade completion
	// events to monitoring subscribers connected to any API replica.
	eventBus := events.NewRedisBus(redisClient)
	sosService := service.NewSOSService(operatorRepository, pilgrimRepository, sosRepository, auditRepository, firebasePusher, eventBus)
	sosHandler := worker.NewSOSHandler(logger, sosService)
	waitlistHandler := worker.NewWaitlistHandler(logger, waitlistRepository)
	cashFlowHandler := worker.NewCashFlowHandler(logger, queries)
	smtpMailer, err := mailer.FromEnv(strings.TrimSpace(os.Getenv("SMTP_HOST")), envOrDefault("SMTP_PORT", "465"), strings.TrimSpace(os.Getenv("SMTP_USER")), os.Getenv("SMTP_PASSWORD"), strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL")))
	if err != nil {
		logger.Error("init SMTP mailer", "error", err)
		os.Exit(1)
	}
	if smtpMailer == nil {
		logger.Warn("finance email delivery disabled", "reason", "SMTP_USER/SMTP_PASSWORD are not configured")
	}
	outboxHandler := worker.NewOutboxHandler(logger, outboxRepository, firebasePusher, journeyService, eventBus, queries, smtpMailer)
	subscriptionHandler := worker.NewSubscriptionHandler(logger, subscriptionRepository)
	planOverrideHandler := worker.NewPlanOverrideHandler(logger, platformRepository)
	// Dry run until the owner has compared one cycle's output against a list
	// made by hand. A demand sent to an agency that already paid costs more
	// than a week of delay.
	usageHandler := worker.NewUsageHandler(logger, subscriptionRepository)
	funnelRollUpHandler := worker.NewFunnelRollUpHandler(logger, repository.NewFunnelRepository(pool))
	dunningHandler := worker.NewDunningHandler(logger, subscriptionRepository, os.Getenv("DUNNING_LIVE") != "true")
	commissionHandler := worker.NewCommissionHandler(logger, ledgerRepository)
	// The poller settles through the same service the webhook does, so there
	// is one definition of settlement and one place the amount is verified.
	orderService := service.NewOrderService(operatorRepository, pilgrimRepository,
		productRepository, orderRepository, auditRepository, ledgerRepository, refundRepository,
		agentRepository, seasonRepository, pool, payment.NewClient(strings.TrimSpace(os.Getenv("XENDIT_SECRET_KEY"))),
		strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGIN")))
	paymentHandler := worker.NewPaymentHandler(logger, orderRepository, orderService)
	refundPayoutService := service.NewRefundPayoutService(operatorRepository, identityRepository, refundPayoutRepository, ledgerRepository, auditRepository, pool, payment.NewClient(strings.TrimSpace(os.Getenv("XENDIT_SECRET_KEY"))))
	refundPayoutHandler := worker.NewRefundPayoutHandler(logger, refundPayoutRepository, refundPayoutService)
	fulfilmentService := service.NewFulfilmentService(fulfilmentRepository, supplierRepository, supplierCostRepository, orderRepository)
	dailySpendHandler := worker.NewDailySpendHandler(logger, orderRepository)
	kycHandler := worker.NewKYCHandler(logger, kycRepository, repository.NewPilgrimRepository(queries))
	fulfilmentHandler := worker.NewFulfilmentHandler(logger, fulfilmentRepository, supplierRepository, fulfilmentService)
	objectStorage, storageErr := storage.New(context.Background(), storage.ConfigFromEnv())
	if storageErr != nil {
		logger.Error("init storefront object storage", "error", storageErr)
		os.Exit(1)
	}
	var storefrontAssetHandler *worker.StorefrontAssetHandler
	if objectStorage != nil {
		storefrontAssetHandler = worker.NewStorefrontAssetHandler(logger, storefrontAssetRepository, objectStorage)
	} else {
		logger.Warn("storefront asset cleanup disabled", "reason", "object storage is not configured")
	}

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
	// Hourly is frequent enough: an invoice is due after days, and a lapsed
	// subscription is already locked out by access_until regardless of status.
	if _, err := scheduler.Register("@every 1h", worker.NewSubscriptionSweepTask()); err != nil {
		logger.Error("register subscription sweep schedule", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.Register("@every 24h", worker.NewFunnelRollUpTask()); err != nil {
		logger.Error("register funnel rollup schedule", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.Register("@every 24h", worker.NewUsageRecomputeTask()); err != nil {
		logger.Error("register usage recompute schedule", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.Register("@every 24h", worker.NewSubscriptionDunningTask()); err != nil {
		logger.Error("register dunning schedule", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.Register("@every 24h", worker.NewPlanOverrideExpireTask()); err != nil {
		logger.Error("register plan override expiration schedule", "error", err)
		os.Exit(1)
	}
	// Every 10 minutes rather than hourly: the gap this closes is an agent not
	// being paid, and it is invisible until someone notices the number is
	// wrong. The sweep is a single indexed statement that does nothing at all
	// when there is nothing missing.
	if _, err := scheduler.Register("@every 10m", worker.NewCommissionReconcileTask()); err != nil {
		logger.Error("register commission reconciliation schedule", "error", err)
		os.Exit(1)
	}
	// Every 2 minutes. A dropped webhook means a jamaah has paid and nobody
	// knows; the cost of checking is one outbound call per order that has been
	// waiting more than the grace period, and usually there are none.
	if _, err := scheduler.Register("@every 2m", worker.NewPaymentPollTask()); err != nil {
		logger.Error("register payment poll schedule", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.Register("@every 30s", worker.NewRefundPayoutDispatchTask()); err != nil {
		logger.Error("register refund payout dispatch schedule", "error", err)
		os.Exit(1)
	}
	// Every 10 minutes. A jamaah waiting on undelivered pulsa is not urgent to
	// the minute, but it must not be discovered a day later either — and
	// nothing else is watching, because holding rather than refunding was a
	// deliberate choice that only works if somebody is told.
	if _, err := scheduler.Register("@every 10m", worker.NewFulfilmentSweepTask()); err != nil {
		logger.Error("register fulfilment sweep schedule", "error", err)
		os.Exit(1)
	}
	// Every minute. A jamaah who has paid for pulsa is waiting from the moment
	// the payment clears, and this is the step between that and it arriving.
	if _, err := scheduler.Register("@every 1m", worker.NewFulfilmentDispatchTask()); err != nil {
		logger.Error("register fulfilment dispatch schedule", "error", err)
		os.Exit(1)
	}
	// Hourly, and normally finds nothing: identity numbers are written
	// encrypted from the start, so this only ever picks up rows that predate
	// that. Scheduled rather than run once because "run it after deploying" is
	// an instruction somebody eventually does not follow.
	if _, err := scheduler.Register("@every 1h", worker.NewKYCMigrateTask()); err != nil {
		logger.Error("register KYC migration schedule", "error", err)
		os.Exit(1)
	}
	// Hourly rather than daily. A daily job that misses its window because the
	// worker was restarting waits a whole day for the next one, and the purge
	// is cheap enough that running it against nothing costs nothing.
	// Daily: the retention window is measured in months, so anything more
	// frequent scans for nothing. Registered separately from the spend purge
	// because they expire different things on different timescales.
	if _, err := scheduler.Register("@every 24h", worker.NewAuditPurgeTask()); err != nil {
		logger.Error("register audit purge schedule", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.Register("@every 1h", worker.NewDailySpendPurgeTask()); err != nil {
		logger.Error("register daily spend purge schedule", "error", err)
		os.Exit(1)
	}
	if storefrontAssetHandler != nil {
		if _, err := scheduler.Register("@every 1h", worker.NewStorefrontAssetGCTask()); err != nil {
			logger.Error("register storefront asset cleanup schedule", "error", err)
			os.Exit(1)
		}
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
	mux.HandleFunc(worker.TaskSubscriptionSweep, subscriptionHandler.HandleSweep)
	mux.HandleFunc(worker.TaskPlanOverrideExpire, planOverrideHandler.HandleExpire)
	mux.HandleFunc(worker.TaskSubscriptionDunning, dunningHandler.HandleRun)
	mux.HandleFunc(worker.TaskUsageRecompute, usageHandler.HandleRecompute)
	mux.HandleFunc(worker.TaskFunnelRollUp, funnelRollUpHandler.HandleRollUp)
	mux.HandleFunc(worker.TaskCommissionReconcile, commissionHandler.HandleReconcile)
	mux.HandleFunc(worker.TaskPaymentPoll, paymentHandler.HandlePoll)
	mux.HandleFunc(worker.TaskRefundPayoutDispatch, refundPayoutHandler.HandleDispatch)
	mux.HandleFunc(worker.TaskFulfilmentSweep, fulfilmentHandler.HandleSweep)
	mux.HandleFunc(worker.TaskFulfilmentDispatch, fulfilmentHandler.HandleDispatch)
	mux.HandleFunc(queue.TaskDispatchOne, fulfilmentHandler.HandleDispatchOne)
	mux.HandleFunc(worker.TaskKYCMigrate, kycHandler.HandleMigrate)
	mux.HandleFunc(worker.TaskDailySpendPurge, dailySpendHandler.HandlePurge)
	mux.HandleFunc(worker.TaskAuditPurge, dailySpendHandler.HandleAuditPurge)
	if storefrontAssetHandler != nil {
		mux.HandleFunc(worker.TaskStorefrontAssetGC, storefrontAssetHandler.HandleGC)
	}

	server := asynq.NewServer(redisOpt, asynq.Config{Concurrency: 5, Logger: slogAdapter{logger}})
	logger.Info("worker listening", "redis", redisURL)
	if err := server.Run(mux); err != nil {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

// slogAdapter satisfies asynq's minimal logger interface with our existing slog.Logger.
type slogAdapter struct{ logger *slog.Logger }

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func (a slogAdapter) Debug(args ...interface{}) { a.logger.Debug(fmt.Sprint(args...)) }
func (a slogAdapter) Info(args ...interface{})  { a.logger.Info(fmt.Sprint(args...)) }
func (a slogAdapter) Warn(args ...interface{})  { a.logger.Warn(fmt.Sprint(args...)) }
func (a slogAdapter) Error(args ...interface{}) { a.logger.Error(fmt.Sprint(args...)) }
func (a slogAdapter) Fatal(args ...interface{}) { a.logger.Error(fmt.Sprint(args...)); os.Exit(1) }
