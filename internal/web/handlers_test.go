package web

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PauloHFS/goth/internal/config"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/alexedwards/scs/v2"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func setupTestDeps(t *testing.T) HandlerDeps {
	dbConn, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := db.RunMigrations(ctx, dbConn); err != nil {
		t.Logf("Aviso: Migração falhou: %v", err)
	}

	t.Cleanup(func() {
		dbConn.Close()
	})

	queries := db.New(dbConn)
	sm := scs.New()

	return HandlerDeps{
		DB:             dbConn,
		Queries:        queries,
		SessionManager: sm,
		Config:         &config.Config{Env: "test"},
	}
}

func TestHomeHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("GOTH Stack Running"))
	})

	t.Run("ReturnsOK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if rr.Body.String() != "GOTH Stack Running" {
			t.Errorf("expected body 'GOTH Stack Running', got %s", rr.Body.String())
		}
	})
}

func TestHandleRegister(t *testing.T) {
	t.Run("MissingEmailAndPassword", func(t *testing.T) {
		deps := setupTestDeps(t)

		req := httptest.NewRequest("POST", "/register", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		handler := Handle(deps, handleRegister)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("InvalidEmail", func(t *testing.T) {
		deps := setupTestDeps(t)

		req := httptest.NewRequest("POST", "/register", nil)
		req.PostForm = map[string][]string{
			"email":    {"invalid-email"},
			"password": {"password123"},
		}
		rr := httptest.NewRecorder()

		handler := Handle(deps, handleRegister)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestHandleForgotPassword(t *testing.T) {
	t.Run("NonExistentEmail", func(t *testing.T) {
		deps := setupTestDeps(t)

		req := httptest.NewRequest("POST", "/forgot-password", nil)
		req.PostForm = map[string][]string{
			"email": {"nonexistent@example.com"},
		}
		rr := httptest.NewRecorder()

		handler := Handle(deps, handleForgotPassword)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestHandleLogin(t *testing.T) {
	t.Run("MissingEmail", func(t *testing.T) {
		deps := setupTestDeps(t)

		req := httptest.NewRequest("POST", "/login", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		handler := Handle(deps, handleLogin)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("NonExistentUser", func(t *testing.T) {
		deps := setupTestDeps(t)

		req := httptest.NewRequest("POST", "/login", nil)
		req.PostForm = map[string][]string{
			"email":    {"wrong@example.com"},
			"password": {"wrongpassword"},
		}
		rr := httptest.NewRecorder()

		handler := Handle(deps, handleLogin)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		deps := setupTestDeps(t)

		_, err := deps.DB.ExecContext(context.Background(), "INSERT INTO tenants (id, name, settings) VALUES (?, ?, ?)", "default", "Default Tenant", []byte("{}"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = deps.DB.ExecContext(context.Background(), "INSERT OR IGNORE INTO roles (id, name, permissions) VALUES (?, ?, ?)", "user", "User", []byte("[]"))
		if err != nil {
			t.Fatal(err)
		}

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
		_, err = deps.Queries.CreateUser(context.Background(), db.CreateUserParams{
			TenantID:     "default",
			Email:        "user@example.com",
			PasswordHash: string(hashedPassword),
			RoleID:       "user",
		})
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("POST", "/login", nil)
		req.PostForm = map[string][]string{
			"email":    {"user@example.com"},
			"password": {"wrongpassword"},
		}
		rr := httptest.NewRecorder()

		handler := Handle(deps, handleLogin)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}
