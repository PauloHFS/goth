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

	"github.com/PauloHFS/goth/internal/cmd"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/platform/config"
	"github.com/PauloHFS/goth/internal/platform/logging"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
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
			Port:               "8099",
			DatabaseURL:        TestDBPath,
			SessionSecret:      "test-secret-key-for-integration-tests",
			AppURL:             TestBaseURL,
			AsaasAPIKey:        "test_asaas_key",
			AsaasEnvironment:   "sandbox",
			AsaasWebhookToken:  "test_webhook_token",
			AsaasHmacSecret:    "test_hmac_secret",
			GoogleClientID:     "",
			GoogleClientSecret: "",
			PasswordPepper:     "test-pepper",
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

	testServer = cmd.SetupTestServer(cmd.TestServerDeps{
		DB:             testDB,
		Queries:        testQueries,
		SessionManager: sessionMgr,
		Logger:         testLogger,
		Config:         testConfig,
	})
}

func TeardownTestServer(t *testing.T) {
	if testServer != nil {
		testServer.Close()
	}
	cmd.ShutdownTestServer()
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
			Port:               "8099",
			DatabaseURL:        TestDBPath,
			SessionSecret:      "test-secret-key-for-integration-tests",
			AppURL:             TestBaseURL,
			AsaasAPIKey:        "test_asaas_key",
			AsaasEnvironment:   "sandbox",
			AsaasWebhookToken:  "test_webhook_token",
			AsaasHmacSecret:    "test_hmac_secret",
			GoogleClientID:     "",
			GoogleClientSecret: "",
			PasswordPepper:     "test-pepper",
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

	// Setup test server using shared function
	testServer = cmd.SetupTestServer(cmd.TestServerDeps{
		DB:             testDB,
		Queries:        testQueries,
		SessionManager: sessionMgr,
		Logger:         testLogger,
		Config:         testConfig,
	})

	// Cleanup
	defer func() {
		testServer.Close()
		cmd.ShutdownTestServer()
	}()

	code := m.Run()
	os.Exit(code)
}
