//go:build fts5
// +build fts5

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/platform/config"
	httpMiddleware "github.com/PauloHFS/goth/internal/platform/http/middleware"
	"github.com/PauloHFS/goth/internal/platform/logging"
	"github.com/PauloHFS/goth/internal/routes"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
	_ "github.com/mattn/go-sqlite3"
)

var (
	testDB      *sql.DB
	testQueries *db.Queries
	sessionMgr  *scs.SessionManager
	testServer  *httptest.Server
	testConfig  *config.Config
	testLogger  *slog.Logger
)

const (
	TestDBPath  = "test/integration/test_integration.db"
	TestPort    = ":8099"
	TestBaseURL = "http://localhost:8099"
)

func SetupTestDB(t *testing.T) {
	os.Remove(TestDBPath)
	os.Remove(TestDBPath + "-wal")
	os.Remove(TestDBPath + "-shm")

	var err error
	testDB, err = sql.Open("sqlite3", TestDBPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := testDB.Ping(); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}

	testQueries = db.New(testDB)

	if err := db.RunMigrations(context.Background(), testDB); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
}

func SetupTestServer(t *testing.T) {
	testLogger = logging.New("debug")

	cfg, err := config.Load()
	if err != nil {
		t.Logf("using default config: %v", err)
		cfg = &config.Config{
			Port:              "8099",
			DatabaseURL:       TestDBPath,
			SessionSecret:     "test-secret-key-for-integration-tests",
			AppURL:            TestBaseURL,
			AsaasAPIKey:       "test_asaas_key",
			AsaasEnvironment:  "sandbox",
			AsaasWebhookToken: "test_webhook_token",
		}
	}
	testConfig = cfg

	testDB.SetMaxOpenConns(25)
	testDB.SetMaxIdleConns(5)
	testDB.SetConnMaxLifetime(300 * time.Second)

	sessionMgr = scs.New()
	sessionMgr.Store = sqlite3store.New(testDB)
	sessionMgr.Lifetime = 24 * time.Hour
	sessionMgr.Cookie.Name = "goth_session_test"
	sessionMgr.Cookie.HttpOnly = true
	sessionMgr.Cookie.Secure = false
	sessionMgr.Cookie.SameSite = http.SameSiteLaxMode

	mux := http.NewServeMux()

	web.RegisterRoutes(mux, web.HandlerDeps{
		DB:             testDB,
		Queries:        testQueries,
		SessionManager: sessionMgr,
		Logger:         testLogger,
		Config:         testConfig,
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("GOTH Stack Running"))
	})

	mux.Handle("POST "+routes.AsaasWebhook, featuresBilling.NewWebhookHandler(testQueries, testConfig.AsaasWebhookToken, "test_hmac_secret", testLogger))

	csrfHandler := nosurf.New(mux)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   false,
	})

	// Rate limiting now handled by Traefik

	handler := httpMiddleware.Recovery(
		httpMiddleware.TenantExtractor("default")(
			httpMiddleware.Logger(
				httpMiddleware.Locale(
					sessionMgr.LoadAndSave(
						httpMiddleware.InjectCSRF(csrfHandler),
					),
				),
			),
		),
	)

	testServer = httptest.NewServer(handler)
}

func TeardownTestServer(t *testing.T) {
	if testServer != nil {
		testServer.Close()
	}
}

func TeardownTestDB(t *testing.T) {
	if testDB != nil {
		testDB.Close()
	}
	os.Remove(TestDBPath)
	os.Remove(TestDBPath + "-wal")
	os.Remove(TestDBPath + "-shm")
}

func GetTestServer() *httptest.Server {
	return testServer
}

func GetTestDB() *sql.DB {
	return testDB
}

func GetTestQueries() *db.Queries {
	return testQueries
}

func GetSessionManager() *scs.SessionManager {
	return sessionMgr
}

func GetTestConfig() *config.Config {
	return testConfig
}

func GetTestLogger() *slog.Logger {
	return testLogger
}

func TestMain(m *testing.M) {
	// Ensure test directory exists
	if err := os.MkdirAll("test/integration", 0755); err != nil {
		fmt.Printf("failed to create test directory: %v\n", err)
		os.Exit(1)
	}

	// Setup database once
	os.Remove(TestDBPath)
	os.Remove(TestDBPath + "-wal")
	os.Remove(TestDBPath + "-shm")

	var err error
	testDB, err = sql.Open("sqlite3", TestDBPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		fmt.Printf("failed to open test database: %v\n", err)
		os.Exit(1)
	}

	if err := testDB.Ping(); err != nil {
		fmt.Printf("failed to ping test database: %v\n", err)
		testDB.Close()
		os.Exit(1)
	}

	testQueries = db.New(testDB)

	if err := db.RunMigrations(context.Background(), testDB); err != nil {
		fmt.Printf("failed to run migrations: %v\n", err)
		testDB.Close()
		os.Exit(1)
	}

	defer func() {
		if testDB != nil {
			testDB.Close()
		}
		os.Remove(TestDBPath)
		os.Remove(TestDBPath + "-wal")
		os.Remove(TestDBPath + "-shm")
	}()

	// Setup logging
	testLogger = logging.New("debug")

	// Setup config
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{
			Port:              "8099",
			DatabaseURL:       TestDBPath,
			SessionSecret:     "test-secret-key-for-integration-tests",
			AppURL:            TestBaseURL,
			AsaasAPIKey:       "test_asaas_key",
			AsaasEnvironment:  "sandbox",
			AsaasWebhookToken: "test_webhook_token",
		}
	}
	testConfig = cfg

	testDB.SetMaxOpenConns(25)
	testDB.SetMaxIdleConns(5)
	testDB.SetConnMaxLifetime(300 * time.Second)

	// Setup session manager
	sessionMgr = scs.New()
	sessionMgr.Store = sqlite3store.New(testDB)
	sessionMgr.Lifetime = 24 * time.Hour
	sessionMgr.Cookie.Name = "goth_session_test"
	sessionMgr.Cookie.HttpOnly = true
	sessionMgr.Cookie.Secure = false
	sessionMgr.Cookie.SameSite = http.SameSiteLaxMode

	// Setup mux
	mux := http.NewServeMux()

	web.RegisterRoutes(mux, web.HandlerDeps{
		DB:             testDB,
		Queries:        testQueries,
		SessionManager: sessionMgr,
		Logger:         testLogger,
		Config:         testConfig,
	})

	// Override health check for testing
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	mux.Handle("POST "+routes.AsaasWebhook, featuresBilling.NewWebhookHandler(testQueries, testConfig.AsaasWebhookToken, "test_hmac_secret", testLogger))

	// Setup CSRF
	// Disable CSRF for tests as it requires valid tokens that are impractical for automated tests
	// In production, CSRF protection is enabled via the middleware chain in cmd/server.go
	csrfHandler := nosurf.New(mux)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   false,
	})
	// Exempt all paths for testing purposes
	csrfHandler.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF validation in tests
		mux.ServeHTTP(w, r)
	}))

	// Initialize global broker for SSE
	web.InitGlobalBroker()

	// Rate limiting now handled by Traefik

	// Build middleware chain - skip CSRF in tests
	handler := httpMiddleware.Recovery(
		httpMiddleware.TenantExtractor("default")(
			httpMiddleware.Logger(
				httpMiddleware.Locale(
					sessionMgr.LoadAndSave(mux),
				),
			),
		),
	)

	testServer = httptest.NewServer(handler)

	// Cleanup in reverse order of initialization
	defer func() {
		testServer.Close()
		web.ShutdownGlobalBroker()
	}()

	code := m.Run()
	os.Exit(code)
}
