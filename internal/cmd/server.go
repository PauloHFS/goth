package cmd

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/PauloHFS/goth/docs"
	"github.com/PauloHFS/goth/internal/db"
	featuresAdmin "github.com/PauloHFS/goth/internal/features/admin"
	featuresAuth "github.com/PauloHFS/goth/internal/features/auth"
	featuresBilling "github.com/PauloHFS/goth/internal/features/billing"
	"github.com/PauloHFS/goth/internal/features/jobs"
	"github.com/PauloHFS/goth/internal/features/jobs/worker"
	featuresSSE "github.com/PauloHFS/goth/internal/features/sse"
	"github.com/PauloHFS/goth/internal/platform/config"
	featureflags "github.com/PauloHFS/goth/internal/platform/featureflags"
	httpHandler "github.com/PauloHFS/goth/internal/platform/http"
	httpMiddleware "github.com/PauloHFS/goth/internal/platform/http/middleware"
	"github.com/PauloHFS/goth/internal/platform/logging"
	"github.com/PauloHFS/goth/internal/platform/metrics"
	"github.com/PauloHFS/goth/internal/platform/observability/audit"
	"github.com/PauloHFS/goth/internal/platform/observability/tracing"
	"github.com/PauloHFS/goth/internal/platform/secrets"
	"github.com/PauloHFS/goth/internal/routes"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/swaggo/http-swagger"
)

func RunServer(assetsFS embed.FS) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize dynamic logging
	if err := logging.InitDynamicLogging(cfg.LogLevel); err != nil {
		return fmt.Errorf("failed to initialize dynamic logging: %w", err)
	}
	logger := logging.NewLogger(cfg.LogLevel)

	// Initialize secrets manager with hot reload
	envFile := ".env"
	secretManager, err := secrets.NewManager(envFile)
	if err != nil {
		logger.Warn("failed to initialize secrets manager, hot reload disabled", "error", err)
	} else {
		logger.Info("secrets manager initialized", "watcher_enabled", true)
		defer func() {
			if err := secretManager.Close(); err != nil {
				logger.Error("failed to close secret manager", "error", err)
			}
		}()

		// Validate required secrets
		if err := secretManager.Validate(cfg.Env); err != nil {
			logger.Warn("secret validation warning", "error", err)
		}
	}

	// Initialize OpenTelemetry tracing
	tracingCfg := tracing.Config{
		ServiceName: "goth",
		Endpoint:    os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Protocol:    os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"),
		UseStdout:   cfg.Env == "dev",
	}
	if _, err := tracing.Init(tracingCfg); err != nil {
		logger.Warn("failed to initialize tracing", "error", err)
	} else {
		logger.Info("tracing initialized", "endpoint", tracingCfg.Endpoint)
	}

	dsn := cfg.DatabaseURL
	if strings.Contains(dsn, "?") {
		dsn += "&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	} else {
		dsn += "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	}

	dbConn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		return fmt.Errorf("failed to open database: %w", err)
	}

	dbConn.SetMaxOpenConns(cfg.MaxOpenConns)
	dbConn.SetMaxIdleConns(cfg.MaxIdleConns)
	dbConn.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	if err := dbConn.Ping(); err != nil {
		logger.Error("failed to ping database", "error", err)
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure WAL mode optimizations
	if err := configureSQLite(dbConn); err != nil {
		logger.Warn("failed to configure SQLite optimizations", "error", err)
	}

	defer func() {
		_ = dbConn.Close()
	}()

	if err := os.MkdirAll("storage/avatars", 0755); err != nil {
		logger.Error("failed to create storage directories", "error", err)
		return fmt.Errorf("failed to create storage directories: %w", err)
	}

	queries := db.New(dbConn)

	if err := db.RunMigrations(context.Background(), dbConn); err != nil {
		logger.Error("failed to run migrations", "error", err)
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = sqlite3store.New(dbConn)

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	sseBroker := featuresSSE.NewBroker(workerCtx)
	defer sseBroker.Shutdown()
	sseHandler := featuresSSE.NewHandler(sseBroker)

	w := worker.New(cfg, dbConn, queries, logger, sseBroker)
	if err := w.RescueZombies(workerCtx); err != nil {
		logger.Error("zombie hunter failed", "error", err)
	}
	go w.Start(workerCtx, cfg.WorkerCount) // Default 3 workers if not configured

	// JobQueue com notificação para o worker
	_ = jobs.NewJobQueueWithNotify(dbConn, w.NotifyNewJob)

	// ============================================================
	// FEATURES - Dependency Injection
	// ============================================================

	// Feature: Feature Flags
	ffRepo := featureflags.NewRepository(dbConn)
	ffManager := featureflags.NewManager(ffRepo, 5*time.Minute)
	ffHandler := featureflags.NewHandler(ffManager)

	// Start DB stats collector
	dbStatsStopCh := make(chan struct{})
	var dbStatsWg sync.WaitGroup
	dbStatsWg.Add(1)
	go func() {
		defer dbStatsWg.Done()
		metrics.StartDBStatsCollector(dbConn, 10*time.Second, dbStatsStopCh)
	}()
	logger.Info("DB stats collector started")

	// Audit logging
	auditRepo := audit.NewRepository(dbConn)
	auditLogger := audit.NewAuthAuditLogger(auditRepo)

	// Feature: Auth
	authUserRepo := featuresAuth.NewRepository(dbConn)
	authEmailVerifRepo := featuresAuth.NewEmailVerificationRepository(dbConn)
	authPasswordResetRepo := featuresAuth.NewPasswordResetRepository(dbConn)
	authJobQueue := featuresAuth.NewJobQueue(dbConn, queries)
	authService := featuresAuth.NewService(featuresAuth.ServiceDeps{
		UserRepo:          authUserRepo,
		EmailVerifRepo:    authEmailVerifRepo,
		PasswordResetRepo: authPasswordResetRepo,
		JobQueue:          authJobQueue,
		PasswordPepper:    cfg.PasswordPepper,
	})
	authHandler := featuresAuth.NewHandler(authService, sessionManager, dbConn, queries, auditLogger)
	authOAuthHandler := featuresAuth.NewOAuthHandler(queries, cfg, logger)

	// Circuit Breaker metrics collector para Google OAuth
	go func() {
		if client := authOAuthHandler.GetHTTPClient(); client != nil {
			metrics.StartCircuitBreakerMetricsCollector(client, 10*time.Second, dbStatsStopCh)
			logger.Info("circuit breaker metrics collector started", "client", "google-oauth")
		}
	}()

	// Feature: Billing
	billingCustomerRepo := featuresBilling.NewCustomerRepository(dbConn)
	billingPaymentRepo := featuresBilling.NewPaymentRepository(dbConn)
	billingAsaasClient := featuresBilling.NewAsaasClientImpl(cfg.AsaasAPIKey, cfg.AsaasEnvironment)
	billingService := featuresBilling.NewService(featuresBilling.ServiceDeps{
		CustomerRepo: billingCustomerRepo,
		PaymentRepo:  billingPaymentRepo,
		AsaasClient:  billingAsaasClient,
	})
	billingHandler := featuresBilling.NewHandler(billingService, dbConn, queries)
	billingWebhookHandler := featuresBilling.NewWebhookHandler(queries, cfg.AsaasWebhookToken, cfg.AsaasHmacSecret, logger)

	// Feature: Jobs
	_ = jobs.NewRepository(dbConn)
	// jobsQueue já foi criado acima com notificação

	// Feature: Admin
	adminHandler := featuresAdmin.NewAdminHandler(queries)

	// ============================================================
	// ROUTING
	// ============================================================

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS))))
	mux.Handle("GET /storage/", http.StripPrefix("/storage/", http.FileServer(http.Dir("storage"))))
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /events", sseHandler)
	mux.Handle("GET /swagger/", httpSwagger.WrapHandler)

	// Feature Flags routes
	ffHandler.RegisterRoutes(mux)

	// Log Level routes
	logLevelHandler := logging.NewLogLevelHandler()
	logLevelHandler.RegisterRoutes(mux)

	// Secrets routes (if manager initialized)
	if secretManager != nil {
		secretsHandler := secrets.NewHandler(secretManager, cfg.Env)
		secretsHandler.RegisterRoutes(mux)
	}

	// Admin routes
	adminHandler.RegisterRoutes(mux)

	// Billing routes
	mux.HandleFunc("POST /checkout/subscribe", func(w http.ResponseWriter, r *http.Request) {
		_ = billingHandler.Subscribe(w, r)
	})
	mux.Handle("POST "+routes.AsaasWebhook, billingWebhookHandler)

	// OAuth routes
	googleProvider := featuresAuth.NewGoogleProvider(cfg)
	if googleProvider != nil && cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		mux.HandleFunc("GET "+routes.GoogleLogin, func(w http.ResponseWriter, r *http.Request) {
			authOAuthHandler.GoogleLogin(w, r, googleProvider)
		})
		mux.HandleFunc("GET "+routes.GoogleCallback, func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = "default"
			}
			err := authOAuthHandler.GoogleCallback(w, r, googleProvider, sessionManager, tenantID)
			if err != nil {
				logger.Error("Google OAuth callback failed", "error", err)
				http.Redirect(w, r, routes.Login+"?error=oauth_failed", http.StatusTemporaryRedirect)
			}
		})
	}

	// Health handlers
	healthHandler := httpHandler.NewHealthHandler(dbConn, logger)
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("GET /ready", healthHandler.Ready)

	// Rate limiting middleware apenas para endpoints críticos de auth
	// (Traefik faz rate limiting global, este é uma camada extra de proteção)
	authRateLimiter := httpMiddleware.RateLimitMiddleware(nil)

	// Auth routes (registered after rate limiter is created)
	authHandler.RegisterRoutes(mux, authRateLimiter)

	// Middleware chain
	csrfHandler := nosurf.New(mux)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   cfg.Env == "prod",
	})

	// Rate limiting é feito pelo Traefik (ver traefik/dynamic/config.yml)
	// Middleware chain foca em segurança e observabilidade
	handler := httpMiddleware.Recovery(
		httpMiddleware.TenantExtractor("default")(
			httpMiddleware.Logger(
				httpMiddleware.Locale(
					sessionManager.LoadAndSave(
						httpMiddleware.InjectCSRF(
							httpMiddleware.RequestID( // Request ID antes do logger para correlation
								httpMiddleware.SecurityHeaders(cfg.Env == "prod")( // Security headers
									httpMiddleware.AddLoggerToContext(logger, csrfHandler),
								),
							),
						),
					),
				),
			),
		),
	)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadTimeout:       time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(cfg.WriteTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(cfg.ReadHeaderTimeout) * time.Second,
		IdleTimeout:       time.Duration(cfg.IdleTimeout) * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("server stopping")

	// Graceful shutdown sequence:
	// 1. Stop DB stats collector
	close(dbStatsStopCh)
	dbStatsWg.Wait()
	logger.Info("DB stats collector stopped")

	// 2. Stop worker from polling new jobs
	cancelWorker()

	// 3. Wait for in-flight jobs to complete
	w.Wait()

	// 4. Shutdown SSE broker (after jobs complete, they may broadcast)
	sseBroker.Shutdown()

	// 5. Graceful HTTP shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	logger.Info("server exited properly")
	return nil
}

// configureSQLite configura otimizações do SQLite para produção
func configureSQLite(db *sql.DB) error {
	// CRÍTICO: Habilitar foreign keys (não é ativado por padrão no SQLite)
	// Deve ser executado por conexão
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// WAL mode já está ativada via DSN, mas configurar parâmetros adicionais
	pragmas := []string{
		"PRAGMA wal_autocheckpoint=1000",     // Checkpoint automático a cada 1000 páginas (~4MB)
		"PRAGMA cache_size=-2000",            // 2MB cache
		"PRAGMA temp_store=MEMORY",           // Temp tables em memória
		"PRAGMA mmap_size=268435456",         // 256MB memory mapped I/O
		"PRAGMA synchronous=NORMAL",          // Balance entre segurança e performance
		"PRAGMA journal_size_limit=67108864", // Limitar WAL a 64MB
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}

	return nil
}
