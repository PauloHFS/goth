//go:build fts5
// +build fts5

package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	tempFile, err := os.CreateTemp("", "health_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	tempFile.Close()

	db, err := sql.Open("sqlite3", tempFile.Name()+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func TestHealthHandler_Health(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	handler := NewHealthHandler(db, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "OK" {
		t.Errorf("expected body 'OK', got '%s'", rr.Body.String())
	}
}

func TestHealthHandler_Ready_Degraded(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHealthHandler(db, nil)

	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()

	handler.Ready(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Status != "degraded" && response.Status != "ready" {
		t.Errorf("expected status 'degraded' or 'ready', got '%s'", response.Status)
	}

	// Now we have 4 checks: database, disk_space, worker, smtp
	if len(response.Checks) != 4 {
		t.Errorf("expected 4 checks, got %d", len(response.Checks))
	}

	if rr.Header().Get("X-Health-Status") != response.Status {
		t.Errorf("expected X-Health-Status header '%s', got '%s'", response.Status, rr.Header().Get("X-Health-Status"))
	}
}

func TestHealthHandler_Ready_DBConnectionError(t *testing.T) {
	db, err := sql.Open("sqlite3", "/nonexistent/path/test.db")
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHealthHandler(db, nil)

	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()

	handler.Ready(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var response HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Status != "not_ready" {
		t.Errorf("expected status 'not_ready', got '%s'", response.Status)
	}

	if rr.Header().Get("X-Health-Status") != "not_ready" {
		t.Errorf("expected X-Health-Status header 'not_ready', got '%s'", rr.Header().Get("X-Health-Status"))
	}
}

func TestHealthHandler_Ready_NilDB(t *testing.T) {
	handler := NewHealthHandler(nil, nil)

	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()

	handler.Ready(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var response HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if response.Status != "not_ready" {
		t.Errorf("expected status 'not_ready', got '%s'", response.Status)
	}
}
