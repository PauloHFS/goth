package integration

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/PauloHFS/goth/internal/config"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/middleware"
	"github.com/PauloHFS/goth/internal/routes"
	"github.com/PauloHFS/goth/internal/web"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	_ "github.com/mattn/go-sqlite3"
)

type TestServer struct {
	DB      *sql.DB
	Queries *db.Queries
	Mux     *http.ServeMux
	Server  *httptest.Server
	Deps    web.HandlerDeps
}

func setupTestServer(t *testing.T) *TestServer {
	dbPath := "./test_integration.db"
	os.Remove(dbPath)
	os.Remove(dbPath + "-shm")
	os.Remove(dbPath + "-wal")

	dbConn, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := dbConn.Ping(); err != nil {
		t.Fatal(err)
	}

	if err := db.RunMigrations(ctx, dbConn); err != nil {
		t.Logf("Aviso: Migração falhou: %v", err)
	}

	queries := db.New(dbConn)

	sessionManager := scs.New()
	sessionManager.Store = sqlite3store.New(dbConn)
	sessionManager.Lifetime = 24 * time.Hour

	deps := web.HandlerDeps{
		DB:             dbConn,
		Queries:        queries,
		SessionManager: sessionManager,
		Config:         &config.Config{Env: "test", Port: "8080"},
	}

	mux := http.NewServeMux()
	web.RegisterRoutes(mux, deps)

	handler := middleware.Recovery(
		middleware.Logger(
			middleware.SecurityHeaders(false)(
				middleware.Locale(
					sessionManager.LoadAndSave(
						mux,
					),
				),
			),
		),
	)

	server := httptest.NewServer(handler)

	ts := &TestServer{
		DB:      dbConn,
		Queries: queries,
		Mux:     mux,
		Server:  server,
		Deps:    deps,
	}

	t.Cleanup(func() {
		server.Close()
		dbConn.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-shm")
		os.Remove(dbPath + "-wal")
	})

	return ts
}

func TestHealthEndpoint(t *testing.T) {
	ts := setupTestServer(t)

	resp, err := ts.Server.Client().Get(ts.Server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, resp.StatusCode)
	}
}

func TestHomeEndpoint(t *testing.T) {
	ts := setupTestServer(t)

	resp, err := ts.Server.Client().Get(ts.Server.URL + routes.Home)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestLoginPage(t *testing.T) {
	ts := setupTestServer(t)

	resp, err := ts.Server.Client().Get(ts.Server.URL + routes.Login)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestRegisterPage(t *testing.T) {
	ts := setupTestServer(t)

	resp, err := ts.Server.Client().Get(ts.Server.URL + routes.Register)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestDuplicateRegistration(t *testing.T) {
	ts := setupTestServer(t)

	_, err := ts.DB.Exec("INSERT INTO tenants (id, name) VALUES ('default', 'Default')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.DB.Exec("INSERT OR IGNORE INTO roles (id, name, permissions) VALUES ('user', 'User', '[]')")
	if err != nil {
		t.Fatal(err)
	}

	form := map[string][]string{
		"email":    {"duplicate@example.com"},
		"password": {"securepassword123"},
	}

	_, err = ts.Queries.CreateUser(context.Background(), db.CreateUserParams{
		TenantID:     "default",
		Email:        "duplicate@example.com",
		PasswordHash: "existinghash",
		RoleID:       "user",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := ts.Server.Client().PostForm(ts.Server.URL+routes.Register, form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestDashboardRequiresAuth(t *testing.T) {
	ts := setupTestServer(t)

	client := ts.Server.Client()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get(ts.Server.URL + routes.Dashboard)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d", http.StatusSeeOther, resp.StatusCode)
	}
}

func TestUserCreation(t *testing.T) {
	ts := setupTestServer(t)

	// Verificar schema
	rows, _ := ts.DB.Query(".tables")
	_ = rows

	// Criar tenant default que é referenciado
	_, err := ts.DB.Exec("INSERT INTO tenants (id, name, settings) VALUES ('default', 'Default', '{}')")
	if err != nil {
		t.Fatalf("Failed to insert tenant: %v", err)
	}

	user, err := ts.Queries.CreateUser(context.Background(), db.CreateUserParams{
		TenantID:     "default",
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		RoleID:       "user",
	})
	if err != nil {
		t.Fatal(err)
	}

	if user.ID == 0 {
		t.Error("expected user ID to be set")
	}
}
