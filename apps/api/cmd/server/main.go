package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/config"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/hajj-saas/api/internal/gen/hajj/v1/hajjv1connect"
	"github.com/hajj-saas/api/internal/handler"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/notification"
	"github.com/hajj-saas/api/internal/payment"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
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
		operatorRepository := repository.NewOperatorRepository(queries)
		pilgrimRepository := repository.NewPilgrimRepository(queries)
		seasonRepository := repository.NewSeasonRepository(queries)
		accommodationRepository := repository.NewAccommodationRepository(queries)
		transportRepository := repository.NewTransportRepository(queries, pool)
		productRepository := repository.NewProductRepository(queries)
		agentRepository := repository.NewAgentRepository(queries)
		groupRepository := repository.NewGroupRepository(queries)
		sosRepository := repository.NewSOSRepository(queries)
		chatRepository := repository.NewChatRepository(queries)
		groupLeaderRepository := repository.NewGroupLeaderRepository(queries)
		notificationRepository := repository.NewNotificationRepository(queries)
		auditRepository := repository.NewAuditRepository(queries)
		kloterRepository := repository.NewKloterRepository(queries)
		identityRepository := repository.NewIdentityRepository(queries)
		orderRepository := repository.NewOrderRepository(queries)

		firebasePusher, err := notification.NewFirebasePusher(ctx, logger, config.FirebaseServiceAccountJSON, notificationRepository)
		if err != nil {
			logger.Error("init firebase", "error", err)
			sentry.CaptureException(err)
		}

		operatorService := service.NewOperatorService(operatorRepository)
		pilgrimService := service.NewPilgrimService(operatorRepository, pilgrimRepository, accommodationRepository, transportRepository, auditRepository, pool)
		seasonService := service.NewSeasonService(operatorRepository, seasonRepository, auditRepository)
		accommodationService := service.NewAccommodationService(operatorRepository, pilgrimRepository, accommodationRepository, auditRepository)
		transportService := service.NewTransportService(operatorRepository, transportRepository, auditRepository)
		productService := service.NewProductService(operatorRepository, productRepository)
		agentService := service.NewAgentService(operatorRepository, agentRepository, auditRepository, pool)
		groupService := service.NewGroupService(operatorRepository, groupRepository, auditRepository, agentRepository)
		pilgrimAppService := service.NewPilgrimAppService(pilgrimRepository, productRepository, auditRepository, identityRepository)
		sosService := service.NewSOSService(operatorRepository, pilgrimRepository, sosRepository, auditRepository, firebasePusher)
		chatService := service.NewChatService(operatorRepository, pilgrimRepository, chatRepository, groupRepository)
		groupLeaderService := service.NewGroupLeaderService(operatorRepository, groupLeaderRepository, sosRepository)
		notificationService := service.NewNotificationService(operatorRepository, notificationRepository)
		kloterService := service.NewKloterService(operatorRepository, kloterRepository, auditRepository)
		identityService := service.NewIdentityService(identityRepository)
		xenditClient := payment.NewClient(config.XenditSecretKey)
		orderService := service.NewOrderService(operatorRepository, pilgrimRepository, productRepository, orderRepository, auditRepository, xenditClient, config.AllowedOrigin)
		operatorHandler := handler.NewOperatorHandler(operatorService)
		pilgrimHandler := handler.NewPilgrimHandler(pilgrimService)
		seasonHandler := handler.NewSeasonHandler(seasonService)
		accommodationHandler := handler.NewAccommodationHandler(accommodationService)
		transportHandler := handler.NewTransportHandler(transportService)
		productHandler := handler.NewProductHandler(productService)
		agentHandler := handler.NewAgentHandler(agentService)
		groupHandler := handler.NewGroupHandler(groupService)
		pilgrimAppHandler := handler.NewPilgrimAppHandler(pilgrimAppService)
		sosHandler := handler.NewSOSHandler(sosService)
		chatHandler := handler.NewChatHandler(chatService)
		groupLeaderHandler := handler.NewGroupLeaderHandler(groupLeaderService)
		notificationHandler := handler.NewNotificationHandler(notificationService)
		kloterHandler := handler.NewKloterHandler(kloterService)
		identityHandler := handler.NewIdentityHandler(identityService)
		orderHandler := handler.NewOrderHandler(orderService)
		handlerOptions := []connect.HandlerOption{connect.WithInterceptors(
			middleware.NewRateLimitInterceptor(),
			middleware.NewAuthInterceptor(pool),
		)}
		operatorPath, operatorServiceHandler := hajjv1connect.NewOperatorServiceHandler(operatorHandler, handlerOptions...)
		pilgrimPath, pilgrimServiceHandler := hajjv1connect.NewPilgrimServiceHandler(pilgrimHandler, handlerOptions...)
		seasonPath, seasonServiceHandler := hajjv1connect.NewSeasonServiceHandler(seasonHandler, handlerOptions...)
		accommodationPath, accommodationServiceHandler := hajjv1connect.NewAccommodationServiceHandler(accommodationHandler, handlerOptions...)
		transportPath, transportServiceHandler := hajjv1connect.NewTransportServiceHandler(transportHandler, handlerOptions...)
		productPath, productServiceHandler := hajjv1connect.NewProductServiceHandler(productHandler, handlerOptions...)
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
		mux.Handle(operatorPath, operatorServiceHandler)
		mux.Handle(pilgrimPath, pilgrimServiceHandler)
		mux.Handle(seasonPath, seasonServiceHandler)
		mux.Handle(accommodationPath, accommodationServiceHandler)
		mux.Handle(transportPath, transportServiceHandler)
		mux.Handle(productPath, productServiceHandler)
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
		mux.HandleFunc("POST /webhooks/xendit", handler.NewXenditWebhookHandler(logger, orderRepository, config.XenditWebhookToken))
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
		Handler:           cors(config.AllowedOrigin, logging(logger, mux)),
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

func cors(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		if request.Method == http.MethodOptions {
			if origin != allowedOrigin {
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
