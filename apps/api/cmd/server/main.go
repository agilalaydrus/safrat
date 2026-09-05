package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/config"
	"github.com/hajj-saas/api/internal/crypto"
	"github.com/hajj-saas/api/internal/events"
	"github.com/hajj-saas/api/internal/funnel"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/notification"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/queue"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/hajj-saas/api/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// sentry.Init with an empty DSN is a documented no-op — safe to call
	// unconditionally so local dev without SENTRY_DSN set just doesn't report.
	if err := sentry.Init(sentry.ClientOptions{Dsn: config.SentryDSN}); err != nil {
		logger.Error("init sentry", "error", err)
	}
	defer sentry.Flush(2 * time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	var pool *pgxpool.Pool
	// Built inside the block below, but consulted by the CORS handler that is
	// constructed after it — same pattern as pool.
	var tenantOrigins *middleware.TenantOriginAllowlist
	{
		var err error
		pool, err = pgxpool.New(ctx, config.DatabaseURL)
		if err != nil {
			logger.Error("connect database", "error", err)
			sentry.CaptureException(err)
			sentry.Flush(2 * time.Second)
			os.Exit(1)
		}
		defer pool.Close()
		queries := db.New(pool)
		var redisClient *redis.Client
		if redisURL := strings.TrimSpace(os.Getenv("REDIS_URL")); redisURL != "" {
			opt, parseErr := redis.ParseURL(redisURL)
			if parseErr != nil {
				logger.Error("parse REDIS_URL", "error", parseErr)
				os.Exit(1)
			}
			redisClient = redis.NewClient(opt)
			defer func() { _ = redisClient.Close() }()
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			if pingErr := redisClient.Ping(pingCtx).Err(); pingErr != nil {
				logger.Warn("redis unavailable at startup; resilient fallbacks are active", "error", pingErr)
			}
			cancel()
		}
		operatorRepository := repository.NewRedisOperatorRepository(ctx, queries, redisClient, logger)
		pilgrimRepository := repository.NewPilgrimRepository(queries)
		seasonRepository := repository.NewSeasonRepository(queries)
		accommodationRepository := repository.NewAccommodationRepository(queries)
		transportRepository := repository.NewTransportRepository(queries, pool)
		productRepository := repository.NewProductRepository(queries, pool)
		agentRepository := repository.NewAgentRepository(queries)
		groupRepository := repository.NewGroupRepository(queries)
		sosRepository := repository.NewSOSRepository(queries)
		chatRepository := repository.NewChatRepository(queries)
		groupLeaderRepository := repository.NewGroupLeaderRepository(queries)
		notificationRepository := repository.NewNotificationRepository(queries)
		auditRepository := repository.NewAuditRepository(queries)
		kloterRepository := repository.NewKloterRepository(queries, pool)
		identityRepository := repository.NewIdentityRepository(queries, agentRepository)
		orderRepository := repository.NewOrderRepository(queries, pool)
		broadcastRepository := repository.NewBroadcastRepository(queries)
		registrationRepository := repository.NewRegistrationRepository(queries)
		waitlistRepository := repository.NewWaitlistRepository(queries)
		cancellationRepository := repository.NewCancellationRepository(pool, queries)
		familyTrackerRepository := repository.NewFamilyTrackerRepository(queries)
		momentRepository := repository.NewMomentRepository(queries)
		cashFlowRepository := repository.NewCashFlowRepository(queries)
		installmentRepository := repository.NewInstallmentRepository(pool)
		crmRepository := repository.NewCRMRepository(pool)
		vendorRepository := repository.NewVendorRepository(queries)
		inventoryRepository := repository.NewInventoryRepository(queries, pool)
		manasikRepository := repository.NewManasikRepository(queries)
		agendaRepository := repository.NewAgendaRepository(queries)
		addonRepository := repository.NewAddonRepository(queries)
		profitLossRepository := repository.NewProfitLossRepository(pool)
		securitySettingsRepository := repository.NewSecuritySettingsRepository(queries)
		supportRepository := repository.NewSupportRepository(queries)
		notificationSettingsRepository := repository.NewNotificationSettingsRepository(queries)
		staffScheduleRepository := repository.NewStaffScheduleRepository(queries)
		insuranceRepository := repository.NewInsuranceRepository(queries)
		checklistRepository := repository.NewChecklistRepository(queries)
		lostReportRepository := repository.NewLostReportRepository(queries)
		tripRepository := repository.NewTripRepository(queries)
		journeyRepository := repository.NewJourneyRepository(queries)
		ritualRepository := repository.NewRitualRepository(queries)
		healthReportRepository := repository.NewHealthReportRepository(queries)
		outboxRepository := repository.NewOutboxRepository(queries)
		monitoringRepository := repository.NewMonitoringRepository(queries)
		analyticsRepository := repository.NewAnalyticsRepository(queries)
		branchRepository := repository.NewBranchRepository(queries)
		entitlementRepository := repository.NewEntitlementRepository(queries)
		storefrontRepository := repository.NewStorefrontRepository(pool)
		storefrontAssetRepository := repository.NewStorefrontAssetRepository(pool)
		operatorDomainRepository := repository.NewOperatorDomainRepository(pool)
		subscriptionRepository := repository.NewSubscriptionRepository(pool)
		ledgerRepository := repository.NewLedgerRepository(pool)
		refundRepository := repository.NewRefundRepository(pool)
		var refundPayoutRepository *repository.RefundPayoutRepository
		// Installed before anything can write an identity. Without a key, KYC
		// writes fail loudly rather than storing an identity number in the
		// clear — which would look like success and stay invisible until a
		// breach made it obvious.
		kycSealer, sealerErr := crypto.NewSealer(strings.TrimSpace(os.Getenv("KYC_ENCRYPTION_KEY")))
		if sealerErr != nil {
			logger.Error("read KYC_ENCRYPTION_KEY", "error", sealerErr)
			os.Exit(1)
		}
		if kycSealer == nil {
			logger.Warn("KYC identity numbers cannot be stored",
				"reason", "KYC_ENCRYPTION_KEY is not set",
				"effect", "submissions will be refused rather than saved unencrypted")
		}
		repository.SetKYCSealer(kycSealer)
		refundPayoutRepository = repository.NewRefundPayoutRepository(pool, kycSealer)
		if kycSealer != nil {
			// Printed at every start, on purpose. It identifies the key without
			// revealing it, so a deployment carrying the wrong one is visible
			// immediately in the logs rather than discovered later, one
			// unreadable identity at a time. It is also what somebody compares
			// a candidate key against when looking for the right one.
			kycRepository := repository.NewKYCRepository(pool)
			inUse, fingerprintErr := kycRepository.KeyFingerprintsInUse(context.Background())
			if fingerprintErr != nil {
				logger.Error("read KYC key fingerprints", "error", fingerprintErr)
			}
			payoutsInUse, payoutFingerprintErr := refundPayoutRepository.DestinationKeyFingerprintsInUse(context.Background())
			if payoutFingerprintErr != nil {
				logger.Error("read refund payout key fingerprints", "error", payoutFingerprintErr)
			}
			logger.Info("KYC encryption key loaded",
				"fingerprint", kycSealer.Fingerprint(), "kyc_records_by_key", inUse,
				"refund_payouts_by_key", payoutsInUse)
			for fingerprint, count := range inUse {
				if fingerprint != "" && fingerprint != kycSealer.Fingerprint() {
					logger.Error("stored identities were sealed with a different key",
						"their_fingerprint", fingerprint, "records", count,
						"loaded_fingerprint", kycSealer.Fingerprint(),
						"effect", "those records cannot be read until that key is restored")
				}
			}
			for fingerprint, count := range payoutsInUse {
				if fingerprint == "" {
					logger.Error("stored payout destinations have no key fingerprint",
						"records", count, "loaded_fingerprint", kycSealer.Fingerprint(),
						"effect", "run rotatekyc with the key that created these records before rotating it away")
				} else if fingerprint != kycSealer.Fingerprint() {
					logger.Error("stored payout destinations were sealed with a different key",
						"their_fingerprint", fingerprint, "records", count,
						"loaded_fingerprint", kycSealer.Fingerprint(),
						"effect", "those payout destinations cannot be dispatched until that key is restored")
				}
			}
		}
		platformRepository := repository.NewPlatformRepository(pool)
		supplierCostRepository := repository.NewSupplierCostRepository(pool)
		supplierRepository := repository.NewSupplierRepository(pool)
		fulfilmentRepository := repository.NewFulfilmentRepository(pool)

		// Gate for Caddy's on-demand TLS. Caddy asks before obtaining a
		// certificate for a hostname it has never seen; answering 200 here is
		// what authorises issuance, so this must be exactly as strict as
		// routing: verified, and on a plan that includes custom domains.
		//
		// Without this gate, anyone could point a DNS record at the server and
		// make it request certificates on their behalf — which burns Let's
		// Encrypt rate limits and would eventually stop issuance for real
		// clients. It is deliberately not routed publicly (see deploy/caddy).
		mux.HandleFunc("GET /internal/tls-authorize", func(w http.ResponseWriter, request *http.Request) {
			hostname := repository.NormalizeHostname(request.URL.Query().Get("domain"))
			if hostname == "" {
				http.Error(w, "domain is required", http.StatusBadRequest)
				return
			}
			if _, err := operatorDomainRepository.ResolveVerified(request.Context(), hostname); err != nil {
				// Same answer for "unknown" and "not entitled": this endpoint
				// should not become a way to probe which plan a domain is on.
				http.Error(w, "not authorized for this domain", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		// Plain http is only acceptable when the configured origin is itself
		// http, i.e. local development.
		tenantOrigins = middleware.NewTenantOriginAllowlist(operatorDomainRepository, logger, time.Minute, strings.HasPrefix(config.AllowedOrigin, "http://"))

		objectStorage, storageErr := storage.New(ctx, config.StorefrontStorage)
		if storageErr != nil {
			logger.Warn("storefront object storage disabled", "error", storageErr)
		}

		firebasePusher, err := notification.NewFirebasePusher(ctx, logger, config.FirebaseServiceAccountJSON, notificationRepository)
		if err != nil {
			logger.Error("init firebase", "error", err)
			sentry.CaptureException(err)
		}
		// firebasePusher is nilable; its methods are nil-receiver safe so local
		// development without Firebase credentials remains a no-op.
		// eventBus feeds the operator monitoring dashboard's real-time stream
		// (MonitoringService.StreamEvents). Redis-backed when REDIS_URL is set
		// (cross-replica delivery, horizontal-scale ready); otherwise an
		// in-process bus that only works for a single instance. See
		// internal/events/bus.go.
		var eventBus *events.Bus
		if redisClient != nil {
			eventBus = events.NewRedisBus(redisClient)
			logger.Info("event bus backend", "type", "redis", "note", "multi-replica ready")
		} else {
			eventBus = events.NewBus()
			logger.Info("event bus backend", "type", "in-memory", "note", "single instance only")
		}

		operatorService := service.NewOperatorService(operatorRepository, seasonRepository, storefrontRepository, storefrontAssetRepository, operatorDomainRepository, objectStorage, config.StorefrontStorageQuotaBytes)
		pilgrimService := service.NewPilgrimService(operatorRepository, pilgrimRepository, accommodationRepository, transportRepository, auditRepository, pool).
			WithEntitlementChecker(service.NewEntitlementChecker(entitlementRepository))
		seasonService := service.NewSeasonService(operatorRepository, seasonRepository, auditRepository, analyticsRepository, monitoringRepository)
		accommodationService := service.NewAccommodationService(operatorRepository, pilgrimRepository, accommodationRepository, auditRepository)
		transportService := service.NewTransportService(operatorRepository, transportRepository, auditRepository)
		productService := service.NewProductService(operatorRepository, productRepository)
		agentService := service.NewAgentService(operatorRepository, agentRepository, auditRepository, pool)
		journeyService := service.NewJourneyService(operatorRepository, journeyRepository, auditRepository)
		groupService := service.NewGroupService(operatorRepository, groupRepository, auditRepository, agentRepository, outboxRepository, pool, eventBus)
		pilgrimAppService := service.NewPilgrimAppService(pilgrimRepository, productRepository, auditRepository, identityRepository, broadcastRepository, journeyRepository, ritualRepository, notificationRepository, orderRepository, ledgerRepository)
		sosService := service.NewSOSService(operatorRepository, pilgrimRepository, sosRepository, auditRepository, firebasePusher, eventBus)
		chatService := service.NewChatService(operatorRepository, pilgrimRepository, chatRepository, groupRepository, groupLeaderRepository)
		groupLeaderService := service.NewGroupLeaderService(operatorRepository, groupLeaderRepository, sosRepository, pilgrimRepository, groupRepository, outboxRepository, pool, eventBus)
		notificationService := service.NewNotificationService(operatorRepository, notificationRepository)
		kloterService := service.NewKloterService(operatorRepository, kloterRepository, auditRepository, outboxRepository, pool, eventBus)
		ritualService := service.NewRitualService(operatorRepository, ritualRepository, journeyRepository, auditRepository, outboxRepository, pool, eventBus)
		healthReportService := service.NewHealthReportService(operatorRepository, healthReportRepository, pilgrimRepository, auditRepository, outboxRepository, pool, eventBus)
		monitoringService := service.NewMonitoringService(operatorRepository, monitoringRepository, groupRepository, eventBus)
		identityService := service.NewIdentityService(identityRepository)
		xenditClient := payment.NewClient(config.XenditSecretKey)
		orderService := service.NewOrderService(operatorRepository, pilgrimRepository, productRepository, orderRepository, auditRepository, ledgerRepository, refundRepository, agentRepository, seasonRepository, pool, xenditClient, config.AllowedOrigin)
		refundPayoutService := service.NewRefundPayoutService(operatorRepository, identityRepository, refundPayoutRepository, ledgerRepository, auditRepository, pool, xenditClient)
		broadcastService := service.NewBroadcastService(operatorRepository, broadcastRepository, auditRepository)
		registrationService := service.NewRegistrationService(operatorRepository, registrationRepository, auditRepository, agentRepository)
		waitlistService := service.NewWaitlistService(operatorRepository, waitlistRepository, auditRepository)
		cancellationService := service.NewCancellationService(operatorRepository, pilgrimRepository, seasonRepository, cancellationRepository, waitlistRepository, auditRepository)
		familyTrackerService := service.NewFamilyTrackerService(familyTrackerRepository, journeyRepository, ritualRepository, momentRepository, objectStorage)
		momentService := service.NewMomentService(operatorRepository, momentRepository, objectStorage)
		cashFlowService := service.NewCashFlowService(operatorRepository, cashFlowRepository, installmentRepository, auditRepository, service.NewEntitlementChecker(entitlementRepository))
		crmService := service.NewCRMService(operatorRepository, crmRepository, auditRepository, service.NewEntitlementChecker(entitlementRepository))
		vendorService := service.NewVendorService(operatorRepository, vendorRepository)
		inventoryService := service.NewInventoryService(operatorRepository, inventoryRepository)
		manasikService := service.NewManasikService(operatorRepository, manasikRepository)
		agendaService := service.NewAgendaService(operatorRepository, agendaRepository)
		addonService := service.NewAddonService(operatorRepository, addonRepository)
		profitLossService := service.NewProfitLossService(operatorRepository, profitLossRepository)
		securitySettingsService := service.NewSecuritySettingsService(operatorRepository, securitySettingsRepository)
		supportService := service.NewSupportService(operatorRepository, supportRepository)
		notificationSettingsService := service.NewNotificationSettingsService(operatorRepository, notificationSettingsRepository)
		staffScheduleService := service.NewStaffScheduleService(operatorRepository, staffScheduleRepository)
		insuranceService := service.NewInsuranceService(operatorRepository, insuranceRepository)
		checklistService := service.NewChecklistService(operatorRepository, pilgrimRepository, checklistRepository)
		lostReportService := service.NewLostReportService(operatorRepository, pilgrimRepository, lostReportRepository, groupLeaderRepository, firebasePusher)
		tripService := service.NewTripService(operatorRepository, tripRepository, pilgrimRepository, sosRepository, groupLeaderRepository, transportRepository, kloterService)
		subscriptionService := service.NewSubscriptionService(subscriptionRepository, operatorRepository, entitlementRepository, xenditClient, service.TransferAccount{
			BankName:      strings.TrimSpace(os.Getenv("SUBSCRIPTION_BANK_NAME")),
			AccountNumber: strings.TrimSpace(os.Getenv("SUBSCRIPTION_BANK_ACCOUNT")),
			AccountHolder: strings.TrimSpace(os.Getenv("SUBSCRIPTION_BANK_HOLDER")),
		}, config.AllowedOrigin)
		branchService := service.NewBranchService(operatorRepository, branchRepository, service.NewEntitlementChecker(entitlementRepository))
		// One funnel service, shared by every handler that records a step:
		// the operator signup that ends the platform's own funnel, and the
		// registration and waitlist forms that end a travel agency's. All of
		// them need the same hasher, so it is built before any of them.
		funnelHasher := funnel.NewHasher(strings.TrimSpace(os.Getenv("FUNNEL_SALT")))
		if !funnelHasher.Configured() {
			logger.Warn("FUNNEL_SALT is not set or too short; visitor funnel recording is disabled")
		}
		funnelService := service.NewFunnelService(repository.NewFunnelRepository(pool), funnelHasher)
		operatorHandler := handler.NewOperatorHandler(operatorService, funnelService)
		subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService)
		branchHandler := handler.NewBranchHandler(branchService)
		pilgrimHandler := handler.NewPilgrimHandler(pilgrimService)
		seasonHandler := handler.NewSeasonHandler(seasonService)
		accommodationHandler := handler.NewAccommodationHandler(accommodationService)
		transportHandler := handler.NewTransportHandler(transportService)
		productHandler := handler.NewProductHandler(productService)
		shipmentHandler := handler.NewShipmentHandler(
			service.NewShipmentService(operatorRepository, fulfilmentRepository, auditRepository, objectStorage, orderService))
		agentHandler := handler.NewAgentHandler(agentService)
		groupHandler := handler.NewGroupHandler(groupService)
		pilgrimAppHandler := handler.NewPilgrimAppHandler(pilgrimAppService)
		sosHandler := handler.NewSOSHandler(sosService)
		chatHandler := handler.NewChatHandler(chatService)
		groupLeaderHandler := handler.NewGroupLeaderHandler(groupLeaderService)
		notificationHandler := handler.NewNotificationHandler(notificationService)
		kloterHandler := handler.NewKloterHandler(kloterService)
		identityHandler := handler.NewIdentityHandler(identityService)
		fulfilmentService := service.NewFulfilmentService(fulfilmentRepository, supplierRepository, supplierCostRepository, orderRepository)
		// The fast path. Without Redis this is nil and every fulfilment waits
		// for the worker's sweep instead — slower, never wrong.
		if fulfilmentQueue, queueErr := queue.NewFulfilmentQueue(strings.TrimSpace(os.Getenv("REDIS_URL"))); queueErr != nil {
			logger.Error("init fulfilment queue", "error", queueErr)
		} else if fulfilmentQueue != nil {
			fulfilmentService.AttachQueue(fulfilmentQueue)
			defer func() { _ = fulfilmentQueue.Close() }()
		} else {
			logger.Warn("fulfilment dispatch will wait for the periodic sweep",
				"reason", "REDIS_URL is not set")
		}
		orderService.AttachFulfilment(fulfilmentService, fulfilmentRepository)
		impersonationRepository := repository.NewImpersonationRepository(pool)
		personalDataReadRepository := repository.NewPersonalDataReadRepository(pool)
		platformService := service.NewPlatformService(platformRepository, supplierCostRepository, supplierRepository, productRepository, subscriptionRepository, repository.NewKYCRepository(pool), auditRepository, repository.NewFunnelRepository(pool), impersonationRepository, personalDataReadRepository)
		// The platform review queue refunds when it resolves a failure, so it
		// needs both — composed after construction because the order service is
		// built later and takes the fulfilment service itself.
		bankMutationService := service.NewBankMutationService(subscriptionRepository, auditRepository, firebasePusher)
		platformService.AttachFulfilment(orderService, fulfilmentService)
		platformService.AttachBankMutations(bankMutationService)
		orderHandler := handler.NewOrderHandler(orderService)
		refundPayoutHandler := handler.NewRefundPayoutHandler(refundPayoutService)
		platformHandler := handler.NewPlatformHandler(platformService)
		broadcastHandler := handler.NewBroadcastHandler(broadcastService)
		registrationHandler := handler.NewRegistrationHandler(registrationService, funnelService)
		waitlistHandler := handler.NewWaitlistHandler(waitlistService, funnelService)
		cancellationHandler := handler.NewCancellationHandler(cancellationService)
		familyTrackerHandler := handler.NewFamilyTrackerHandler(familyTrackerService)
		cashFlowHandler := handler.NewCashFlowHandler(cashFlowService)
		crmHandler := handler.NewCRMHandler(crmService)
		vendorHandler := handler.NewVendorHandler(vendorService)
		inventoryHandler := handler.NewInventoryHandler(inventoryService)
		manasikHandler := handler.NewManasikHandler(manasikService)
		agendaHandler := handler.NewAgendaHandler(agendaService)
		addonHandler := handler.NewAddonHandler(addonService)
		profitLossHandler := handler.NewProfitLossHandler(profitLossService)
		securitySettingsHandler := handler.NewSecuritySettingsHandler(securitySettingsService)
		supportHandler := handler.NewSupportHandler(supportService)
		notificationSettingsHandler := handler.NewNotificationSettingsHandler(notificationSettingsService)
		momentHandler := handler.NewMomentHandler(momentService)
		staffScheduleHandler := handler.NewStaffScheduleHandler(staffScheduleService)
		insuranceHandler := handler.NewInsuranceHandler(insuranceService)
		checklistHandler := handler.NewChecklistHandler(checklistService)
		lostReportHandler := handler.NewLostReportHandler(lostReportService)
		tripHandler := handler.NewTripHandler(tripService)
		journeyHandler := handler.NewJourneyHandler(journeyService)
		ritualHandler := handler.NewRitualHandler(ritualService)
		healthReportHandler := handler.NewHealthReportHandler(healthReportService)
		monitoringHandler := handler.NewMonitoringHandler(monitoringService)
		rateLimitInterceptor := middleware.NewRateLimitInterceptor()
		if redisClient != nil {
			rateLimitInterceptor = middleware.NewRedisRateLimitInterceptor(redisClient, logger)
		}
		handlerOptions := []connect.HandlerOption{connect.WithInterceptors(
			rateLimitInterceptor,
			middleware.NewAuthInterceptorWithImpersonation(pool, identityRepository, subscriptionRepository, impersonationRepository, personalDataReadRepository),
		)}
		operatorPath, operatorServiceHandler := hajjv1connect.NewOperatorServiceHandler(operatorHandler, handlerOptions...)
		subscriptionPath, subscriptionServiceHandler := hajjv1connect.NewSubscriptionServiceHandler(subscriptionHandler, handlerOptions...)
		branchPath, branchServiceHandler := hajjv1connect.NewBranchServiceHandler(branchHandler, handlerOptions...)
		pilgrimPath, pilgrimServiceHandler := hajjv1connect.NewPilgrimServiceHandler(pilgrimHandler, handlerOptions...)
		seasonPath, seasonServiceHandler := hajjv1connect.NewSeasonServiceHandler(seasonHandler, handlerOptions...)
		accommodationPath, accommodationServiceHandler := hajjv1connect.NewAccommodationServiceHandler(accommodationHandler, handlerOptions...)
		transportPath, transportServiceHandler := hajjv1connect.NewTransportServiceHandler(transportHandler, handlerOptions...)
		productPath, productServiceHandler := hajjv1connect.NewProductServiceHandler(productHandler, handlerOptions...)
		shipmentPath, shipmentServiceHandler := hajjv1connect.NewShipmentServiceHandler(shipmentHandler, handlerOptions...)
		agentPath, agentServiceHandler := hajjv1connect.NewAgentServiceHandler(agentHandler, handlerOptions...)
		groupPath, groupServiceHandler := hajjv1connect.NewGroupServiceHandler(groupHandler, handlerOptions...)
		pilgrimAppPath, pilgrimAppServiceHandler := hajjv1connect.NewPilgrimAppServiceHandler(pilgrimAppHandler, handlerOptions...)
		sosPath, sosServiceHandler := hajjv1connect.NewSOSServiceHandler(sosHandler, handlerOptions...)
		chatPath, chatServiceHandler := hajjv1connect.NewChatServiceHandler(chatHandler, handlerOptions...)
		groupLeaderPath, groupLeaderServiceHandler := hajjv1connect.NewGroupLeaderServiceHandler(groupLeaderHandler, handlerOptions...)
		notificationPath, notificationServiceHandler := hajjv1connect.NewNotificationServiceHandler(notificationHandler, handlerOptions...)
		kloterPath, kloterServiceHandler := hajjv1connect.NewKloterServiceHandler(kloterHandler, handlerOptions...)
		identityPath, identityServiceHandler := hajjv1connect.NewIdentityServiceHandler(identityHandler, handlerOptions...)
		orderPath, orderServiceHandler := hajjv1connect.NewOrderServiceHandler(orderHandler, handlerOptions...)
		dataExportHandler := handler.NewDataExportHandler(
			service.NewDataExportService(operatorRepository, repository.NewDataExportRepository(pool), objectStorage))
		dataExportPath, dataExportServiceHandler := hajjv1connect.NewDataExportServiceHandler(dataExportHandler, handlerOptions...)
		refundPayoutPath, refundPayoutServiceHandler := hajjv1connect.NewRefundPayoutServiceHandler(refundPayoutHandler, handlerOptions...)
		platformPath, platformServiceHandler := hajjv1connect.NewPlatformServiceHandler(platformHandler, handlerOptions...)
		broadcastPath, broadcastServiceHandler := hajjv1connect.NewBroadcastServiceHandler(broadcastHandler, handlerOptions...)
		registrationPath, registrationServiceHandler := hajjv1connect.NewRegistrationServiceHandler(registrationHandler, handlerOptions...)
		waitlistPath, waitlistServiceHandler := hajjv1connect.NewWaitlistServiceHandler(waitlistHandler, handlerOptions...)
		cancellationPath, cancellationServiceHandler := hajjv1connect.NewCancellationServiceHandler(cancellationHandler, handlerOptions...)
		familyTrackerPath, familyTrackerServiceHandler := hajjv1connect.NewFamilyTrackerServiceHandler(familyTrackerHandler, handlerOptions...)
		cashFlowPath, cashFlowServiceHandler := hajjv1connect.NewCashFlowServiceHandler(cashFlowHandler, handlerOptions...)
		crmPath, crmServiceHandler := hajjv1connect.NewCRMServiceHandler(crmHandler, handlerOptions...)
		vendorPath, vendorServiceHandler := hajjv1connect.NewVendorServiceHandler(vendorHandler, handlerOptions...)
		inventoryPath, inventoryServiceHandler := hajjv1connect.NewInventoryServiceHandler(inventoryHandler, handlerOptions...)
		manasikPath, manasikServiceHandler := hajjv1connect.NewManasikServiceHandler(manasikHandler, handlerOptions...)
		agendaPath, agendaServiceHandler := hajjv1connect.NewAgendaServiceHandler(agendaHandler, handlerOptions...)
		addonPath, addonServiceHandler := hajjv1connect.NewAddonServiceHandler(addonHandler, handlerOptions...)
		profitLossPath, profitLossServiceHandler := hajjv1connect.NewProfitLossServiceHandler(profitLossHandler, handlerOptions...)
		securitySettingsPath, securitySettingsServiceHandler := hajjv1connect.NewSecuritySettingsServiceHandler(securitySettingsHandler, handlerOptions...)
		supportPath, supportServiceHandler := hajjv1connect.NewSupportServiceHandler(supportHandler, handlerOptions...)
		notificationSettingsPath, notificationSettingsServiceHandler := hajjv1connect.NewNotificationSettingsServiceHandler(notificationSettingsHandler, handlerOptions...)
		momentPath, momentServiceHandler := hajjv1connect.NewMomentServiceHandler(momentHandler, handlerOptions...)
		staffSchedulePath, staffScheduleServiceHandler := hajjv1connect.NewStaffScheduleServiceHandler(staffScheduleHandler, handlerOptions...)
		insurancePath, insuranceServiceHandler := hajjv1connect.NewInsuranceServiceHandler(insuranceHandler, handlerOptions...)
		checklistPath, checklistServiceHandler := hajjv1connect.NewChecklistServiceHandler(checklistHandler, handlerOptions...)
		lostReportPath, lostReportServiceHandler := hajjv1connect.NewLostReportServiceHandler(lostReportHandler, handlerOptions...)
		tripPath, tripServiceHandler := hajjv1connect.NewTripServiceHandler(tripHandler, handlerOptions...)
		journeyPath, journeyServiceHandler := hajjv1connect.NewJourneyServiceHandler(journeyHandler, handlerOptions...)
		ritualPath, ritualServiceHandler := hajjv1connect.NewRitualServiceHandler(ritualHandler, handlerOptions...)
		healthReportPath, healthReportServiceHandler := hajjv1connect.NewHealthReportServiceHandler(healthReportHandler, handlerOptions...)
		monitoringPath, monitoringServiceHandler := hajjv1connect.NewMonitoringServiceHandler(monitoringHandler, handlerOptions...)
		// Recording is skipped entirely when FUNNEL_SALT is unset: a visitor
		// token without a salt can be reversed from a list of addresses, and a
		// table that only looks anonymous is worse than an empty one. The
		// service logs nothing and fails silently, so a missing salt degrades
		// to "no measurements" rather than to errors on every page.
		if len(strings.TrimSpace(os.Getenv("FUNNEL_INGEST_SECRET"))) < 32 {
			logger.Warn("FUNNEL_INGEST_SECRET is not set or too short; trusted visitor forwarding is disabled")
		}
		funnelHandler := handler.NewFunnelHandler(funnelService, os.Getenv("FUNNEL_INGEST_SECRET"))
		funnelPath, funnelServiceHandler := hajjv1connect.NewFunnelServiceHandler(funnelHandler, handlerOptions...)
		funnelReportHandler := handler.NewFunnelReportHandler(service.NewFunnelReportService(
			operatorRepository, repository.NewFunnelRepository(pool), repository.FunnelRetentionDays))
		funnelReportPath, funnelReportServiceHandler := hajjv1connect.NewFunnelReportServiceHandler(funnelReportHandler, handlerOptions...)
		mux.Handle(operatorPath, operatorServiceHandler)
		mux.Handle(subscriptionPath, subscriptionServiceHandler)
		mux.Handle(branchPath, branchServiceHandler)
		mux.Handle(pilgrimPath, pilgrimServiceHandler)
		mux.Handle(seasonPath, seasonServiceHandler)
		mux.Handle(accommodationPath, accommodationServiceHandler)
		mux.Handle(transportPath, transportServiceHandler)
		mux.Handle(productPath, productServiceHandler)
		mux.Handle(shipmentPath, shipmentServiceHandler)
		mux.Handle(agentPath, agentServiceHandler)
		mux.Handle(groupPath, groupServiceHandler)
		mux.Handle(pilgrimAppPath, pilgrimAppServiceHandler)
		mux.Handle(sosPath, sosServiceHandler)
		mux.Handle(chatPath, chatServiceHandler)
		mux.Handle(groupLeaderPath, groupLeaderServiceHandler)
		mux.Handle(notificationPath, notificationServiceHandler)
		mux.Handle(kloterPath, kloterServiceHandler)
		mux.Handle(identityPath, identityServiceHandler)
		mux.Handle(orderPath, orderServiceHandler)
		mux.Handle(dataExportPath, dataExportServiceHandler)
		mux.Handle(refundPayoutPath, refundPayoutServiceHandler)
		mux.Handle(platformPath, platformServiceHandler)
		mux.Handle(broadcastPath, broadcastServiceHandler)
		mux.Handle(registrationPath, registrationServiceHandler)
		mux.Handle(waitlistPath, waitlistServiceHandler)
		mux.Handle(cancellationPath, cancellationServiceHandler)
		mux.Handle(familyTrackerPath, familyTrackerServiceHandler)
		mux.Handle(cashFlowPath, cashFlowServiceHandler)
		mux.Handle(crmPath, crmServiceHandler)
		mux.Handle(vendorPath, vendorServiceHandler)
		mux.Handle(inventoryPath, inventoryServiceHandler)
		mux.Handle(manasikPath, manasikServiceHandler)
		mux.Handle(agendaPath, agendaServiceHandler)
		mux.Handle(addonPath, addonServiceHandler)
		mux.Handle(profitLossPath, profitLossServiceHandler)
		mux.Handle(securitySettingsPath, securitySettingsServiceHandler)
		mux.Handle(supportPath, supportServiceHandler)
		mux.Handle(notificationSettingsPath, notificationSettingsServiceHandler)
		mux.Handle(momentPath, momentServiceHandler)
		mux.Handle(staffSchedulePath, staffScheduleServiceHandler)
		mux.Handle(insurancePath, insuranceServiceHandler)
		mux.Handle(checklistPath, checklistServiceHandler)
		mux.Handle(lostReportPath, lostReportServiceHandler)
		mux.Handle(tripPath, tripServiceHandler)
		mux.Handle(journeyPath, journeyServiceHandler)
		mux.Handle(ritualPath, ritualServiceHandler)
		mux.Handle(healthReportPath, healthReportServiceHandler)
		mux.Handle(monitoringPath, monitoringServiceHandler)
		mux.Handle(funnelPath, funnelServiceHandler)
		mux.Handle(funnelReportPath, funnelReportServiceHandler)
		mux.HandleFunc("POST /webhooks/supplier/{token}", handler.NewSupplierCallbackHandler(logger, fulfilmentService))
		// Bank credits from a poller or scraper. Signed over the body, and
		// refused outright when the secret is unset — an endpoint that grants
		// subscription access must not be open because a variable was forgotten.
		mux.HandleFunc("POST /webhooks/bank-feed", handler.NewBankFeedHandler(logger, bankMutationService, strings.TrimSpace(os.Getenv("BANK_FEED_SECRET"))))
		xenditSourceGuard := handler.NewWebhookSourceGuard(logger, os.Getenv("XENDIT_WEBHOOK_ALLOWED_IPS"))
		mux.HandleFunc("POST /webhooks/xendit", handler.NewXenditWebhookHandler(logger, orderRepository, orderService, subscriptionRepository, config.XenditWebhookToken, xenditSourceGuard))
		mux.HandleFunc("POST /webhooks/xendit/payout", handler.NewXenditPayoutWebhookHandler(logger, refundPayoutService, config.XenditWebhookToken, xenditSourceGuard))
		uploadDir := os.Getenv("UPLOAD_DIR")
		if uploadDir == "" {
			uploadDir = "./uploads/documents"
		}
		mux.HandleFunc("POST /upload/document", handler.NewDocumentUploadHandler(pool, pilgrimService, uploadDir))
		mux.HandleFunc("POST /upload/document/self", handler.NewPilgrimSelfDocumentUploadHandler(pilgrimService, uploadDir))
		mux.HandleFunc("POST /upload/agent-document", handler.NewAgentDocumentUploadHandler(pool, agentService, uploadDir))
		mux.HandleFunc("POST /upload/agent-document/self", handler.NewAgentSelfDocumentUploadHandler(pool, agentService, uploadDir))
		mux.HandleFunc("POST /upload/refund-payout-proof", handler.NewRefundPayoutProofUploadHandler(pool, refundPayoutService, uploadDir))
		mux.Handle("/uploads/documents/", http.StripPrefix("/uploads/documents/", http.FileServer(http.Dir(uploadDir))))
		mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, request *http.Request) {
			if err := pool.Ping(request.Context()); err != nil {
				http.Error(w, `{"status":"database_unavailable"}`, http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		})
	}

	server := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           cors(config.AllowedOrigin, tenantOrigins, logging(logger, mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("api listening", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("serve api", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown api", "error", err)
	}
}

// originMatcher accepts the exact configured origin, plus any single-label
// subdomain of the same base host — e.g. CORS_ALLOWED_ORIGIN=
// https://tawafiqhub.id also allows https://vacana.tawafiqhub.id,
// https://maktour.tawafiqhub.id, etc. (see apps/web/lib/tenant-link.ts and
// middleware.ts — every operator gets its own subdomain for /register,
// /apply, /waitlist). 127.0.0.1/localhost are treated as the same base for
// local dev, matching tenant-link.ts's "*.localhost" convention.
type originMatcher struct {
	exact    string
	scheme   string
	port     string
	baseHost string
}

func newOriginMatcher(allowedOrigin string) originMatcher {
	m := originMatcher{exact: allowedOrigin}
	parsed, err := url.Parse(allowedOrigin)
	if err != nil {
		return m
	}
	m.scheme = parsed.Scheme
	m.port = parsed.Port()
	host := parsed.Hostname()
	switch {
	case host == "127.0.0.1" || host == "localhost":
		host = "localhost"
	default:
		if label, rest, found := strings.Cut(host, "."); found && (label == "app" || label == "www") {
			host = rest
		}
	}
	m.baseHost = host
	return m
}

func (m originMatcher) allows(origin string) bool {
	if origin == "" {
		return false
	}
	if origin == m.exact {
		return true
	}
	if m.baseHost == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != m.scheme || parsed.Port() != m.port {
		return false
	}
	sub, rest, found := strings.Cut(parsed.Hostname(), ".")
	return found && repository.IsValidOperatorSlug(sub) && rest == m.baseHost
}

// tenantOrigins is nil until client-owned domains exist, in which case CORS
// behaves exactly as it did before.
func cors(allowedOrigin string, tenantOrigins *middleware.TenantOriginAllowlist, next http.Handler) http.Handler {
	matcher := newOriginMatcher(allowedOrigin)
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		// The static matcher covers the platform apex and its subdomains; the
		// allowlist adds domains clients own and have verified.
		allowed := matcher.allows(origin) || tenantOrigins.Allows(request.Context(), origin)
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		if request.Method == http.MethodOptions {
			if !allowed {
				http.Error(w, "origin is not allowed", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, request)
	})
}

func logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, request)
		logger.Info("http request", "method", request.Method, "path", request.URL.Path, "duration_ms", time.Since(startedAt).Milliseconds())
	})
}
