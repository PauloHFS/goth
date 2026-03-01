// Package cmd fornece funções para inicializar a aplicação.
// Este arquivo é usado principalmente para testes de integração.
package cmd

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	featuresAuth "github.com/PauloHFS/goth/internal/features/auth"
	featuresBilling "github.com/PauloHFS/goth/internal/features/billing"
	featuresSSE "github.com/PauloHFS/goth/internal/features/sse"
	featuresUser "github.com/PauloHFS/goth/internal/features/user"
	"github.com/PauloHFS/goth/internal/platform/config"
	httpHandler "github.com/PauloHFS/goth/internal/platform/http"
	httpMiddleware "github.com/PauloHFS/goth/internal/platform/http/middleware"
	"github.com/PauloHFS/goth/internal/platform/logging"
	"github.com/PauloHFS/goth/internal/platform/metrics"
	"github.com/PauloHFS/goth/internal/platform/observability/audit"
	"github.com/PauloHFS/goth/internal/routes"
	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	testBroker featuresSSE.Broker
	testCancel context.CancelFunc
)

// TestServerDeps contém dependências para setup de teste
type TestServerDeps struct {
	DB             *sql.DB
	Queries        *db.Queries
	SessionManager *scs.SessionManager
	Logger         *slog.Logger
	Config         *config.Config
}

// SetupTestServer cria um httptest.Server configurado com todas as rotas
// Esta função é usada apenas para testes de integração
func SetupTestServer(deps TestServerDeps) *httptest.Server {
	// Setup audit logging
	auditRepo := audit.NewRepository(deps.DB)
	auditLogger := audit.NewAuthAuditLogger(auditRepo)

	// Setup auth
	authUserRepo := featuresAuth.NewRepository(deps.DB)
	authEmailVerifRepo := featuresAuth.NewEmailVerificationRepository(deps.DB)
	authPasswordResetRepo := featuresAuth.NewPasswordResetRepository(deps.DB)
	authJobQueue := featuresAuth.NewJobQueue(deps.DB, deps.Queries)
	authService := featuresAuth.NewService(featuresAuth.ServiceDeps{
		UserRepo:          authUserRepo,
		EmailVerifRepo:    authEmailVerifRepo,
		PasswordResetRepo: authPasswordResetRepo,
		JobQueue:          authJobQueue,
		PasswordPepper:    deps.Config.PasswordPepper,
	})
	authHandler := featuresAuth.NewHandler(authService, deps.SessionManager, deps.DB, deps.Queries, auditLogger)
	authOAuthHandler := featuresAuth.NewOAuthHandler(deps.Queries, deps.Config, deps.Logger)

	// Setup billing
	billingCustomerRepo := featuresBilling.NewCustomerRepository(deps.DB)
	billingPaymentRepo := featuresBilling.NewPaymentRepository(deps.DB)
	billingAsaasClient := featuresBilling.NewAsaasClientImpl(deps.Config.AsaasAPIKey, deps.Config.AsaasEnvironment)
	billingService := featuresBilling.NewService(featuresBilling.ServiceDeps{
		CustomerRepo: billingCustomerRepo,
		PaymentRepo:  billingPaymentRepo,
		AsaasClient:  billingAsaasClient,
	})
	billingHandler := featuresBilling.NewHandler(billingService, deps.DB, deps.Queries)
	billingWebhookHandler := featuresBilling.NewWebhookHandler(deps.Queries, deps.Config.AsaasWebhookToken, deps.Config.AsaasHmacSecret, deps.Logger)

	// Setup user
	userRepo := featuresUser.NewRepository(deps.DB)
	userHandler := featuresUser.NewHandler(userRepo, deps.SessionManager, deps.DB, deps.Queries)

	// Setup SSE broker
	ctx, cancel := context.WithCancel(context.Background())
	testCancel = cancel
	testBroker = featuresSSE.NewBroker(ctx)
	sseHandler := featuresSSE.NewHandler(testBroker)

	// Setup mux
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /events", sseHandler)

	// Health handlers
	healthHandler := httpHandler.NewHealthHandler(deps.DB, deps.Logger)
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("GET /ready", healthHandler.Ready)

	// Home route
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("GOTH Stack Running"))
	})

	// Rate limiter
	authRateLimiter := httpMiddleware.RateLimitMiddleware(nil)

	// Register routes
	authHandler.RegisterRoutes(mux, authRateLimiter)

	// Billing routes
	mux.HandleFunc("POST "+routes.CheckoutSubscribe, func(w http.ResponseWriter, r *http.Request) {
		_ = billingHandler.Subscribe(w, r)
	})
	mux.Handle("POST "+routes.AsaasWebhook, billingWebhookHandler)

	// User routes
	userHandler.RegisterRoutes(mux)

	// OAuth routes (if configured)
	googleProvider := featuresAuth.NewGoogleProvider(deps.Config)
	if googleProvider != nil && deps.Config.GoogleClientID != "" && deps.Config.GoogleClientSecret != "" {
		mux.HandleFunc("GET "+routes.GoogleLogin, func(w http.ResponseWriter, r *http.Request) {
			authOAuthHandler.GoogleLogin(w, r, googleProvider)
		})
		mux.HandleFunc("GET "+routes.GoogleCallback, func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = "default"
			}
			err := authOAuthHandler.GoogleCallback(w, r, googleProvider, deps.SessionManager, tenantID)
			if err != nil {
				deps.Logger.Error("Google OAuth callback failed", "error", err)
				http.Redirect(w, r, routes.Login+"?error=oauth_failed", http.StatusTemporaryRedirect)
			}
		})
	}

	// CSRF
	csrfHandler := nosurf.New(mux)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   false,
	})

	// Middleware chain (simplified for tests)
	handler := httpMiddleware.Recovery(
		httpMiddleware.TenantExtractor("default")(
			httpMiddleware.Logger(
				httpMiddleware.Locale(
					deps.SessionManager.LoadAndSave(
						httpMiddleware.InjectCSRF(csrfHandler),
					),
				),
			),
		),
	)

	return httptest.NewServer(handler)
}

// ShutdownTestServer limpa recursos do servidor de teste
func ShutdownTestServer() {
	if testCancel != nil {
		testCancel()
	}
}

// StartDBStatsCollector inicia coletor de métricas do database
func StartDBStatsCollector(db *sql.DB, interval time.Duration, stopCh chan struct{}) {
	metrics.StartDBStatsCollector(db, interval, stopCh)
}

// Ensure logging package is used
var _ = logging.New
