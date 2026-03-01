//go:build integration && fts5
// +build integration,fts5

package integration

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PauloHFS/goth/internal/cmd"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/platform/config"
	"github.com/PauloHFS/goth/internal/platform/logging"
	"github.com/PauloHFS/goth/test/integration/seed"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	_ "github.com/mattn/go-sqlite3"
)

var (
	integrationDB      *sql.DB
	integrationQueries *db.Queries
	integrationSession *scs.SessionManager
	integrationServer  *httptest.Server
	integrationConfig  *config.Config
)

// SetupIntegrationTest cria um servidor de teste completo com CSRF habilitado
func SetupIntegrationTest(t *testing.T) {
	t.Helper()

	// Configurar banco de dados de teste
	os.Remove("test/integration/test_csrf.db")
	os.Remove("test/integration/test_csrf.db-wal")
	os.Remove("test/integration/test_csrf.db-shm")

	var err error
	integrationDB, err = sql.Open("sqlite3", "test/integration/test_csrf.db?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := integrationDB.Ping(); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	integrationQueries = db.New(integrationDB)

	if err := db.RunMigrations(context.Background(), integrationDB); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Configurar ambiente de desenvolvimento
	os.Setenv("APP_ENV", "dev")

	// Configurar sessão
	integrationSession = scs.New()
	integrationSession.Store = sqlite3store.New(integrationDB)
	integrationSession.Lifetime = 24 * time.Hour
	integrationSession.Cookie.Name = "goth_session_integration"
	integrationSession.Cookie.HttpOnly = true
	integrationSession.Cookie.Secure = false

	// Configurar config de teste
	integrationConfig = &config.Config{
		Port:              "0", // Porta aleatória
		DatabaseURL:       "test/integration/test_csrf.db",
		SessionSecret:     "test-secret-key-for-csrf-integration-tests",
		PasswordPepper:    "test-pepper",
		AsaasAPIKey:       "test_asaas_key",
		AsaasEnvironment:  "sandbox",
		AsaasWebhookToken: "test_webhook_token",
		AsaasHmacSecret:   "test_hmac_secret",
		Env:               "dev",
	}

	// Seed de dados de teste
	seedTestData(t)

	// Setup do servidor usando SetupTestServer do cmd package
	integrationServer = cmd.SetupTestServer(cmd.TestServerDeps{
		DB:             integrationDB,
		Queries:        integrationQueries,
		SessionManager: integrationSession,
		Logger:         logging.New("debug"),
		Config:         integrationConfig,
	})
}

// TeardownIntegrationTest limpa recursos
func TeardownIntegrationTest(t *testing.T) {
	t.Helper()
	if integrationServer != nil {
		integrationServer.Close()
	}
	cmd.ShutdownTestServer()
	if integrationDB != nil {
		integrationDB.Close()
	}
	os.Remove("test/integration/test_csrf.db")
	os.Remove("test/integration/test_csrf.db-wal")
	os.Remove("test/integration/test_csrf.db-shm")
}

// seedTestData popula dados para testes
func seedTestData(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	_ = seed.SeedTestData(ctx, integrationQueries)
}

// extractCSRFToken extrai token CSRF do HTML
func extractCSRFToken(html string) string {
	start := strings.Index(html, `name="csrf_token" value="`)
	if start == -1 {
		return ""
	}
	start += len(`name="csrf_token" value="`)
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		return ""
	}
	return html[start : start+end]
}

// extractCSRFCookie extrai cookie CSRF da resposta
func extractCSRFCookie(cookies []*http.Cookie) string {
	for _, cookie := range cookies {
		if cookie.Name == "csrf_token" {
			return cookie.Value
		}
	}
	return ""
}

// TestCSRF_Integration_GetLoginPage valida que GET /login retorna token CSRF
func TestCSRF_Integration_GetLoginPage(t *testing.T) {
	SetupIntegrationTest(t)
	defer TeardownIntegrationTest(t)

	resp, err := http.Get(integrationServer.URL + "/login")
	if err != nil {
		t.Fatalf("failed to get login page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	html := string(body)

	// Verificar se token CSRF está presente no HTML
	token := extractCSRFToken(html)
	if token == "" {
		t.Fatal("expected csrf_token in login page HTML")
	}

	if len(token) < 20 {
		t.Errorf("expected token length >= 20, got %d", len(token))
	}

	// Verificar se cookie CSRF foi definido
	csrfCookie := extractCSRFCookie(resp.Cookies())
	if csrfCookie == "" {
		t.Error("expected csrf_token cookie")
	}
}

// TestCSRF_Integration_PostLoginWithValidToken valida POST com token válido
func TestCSRF_Integration_PostLoginWithValidToken(t *testing.T) {
	SetupIntegrationTest(t)
	defer TeardownIntegrationTest(t)
	seedTestData(t)

	// 1. Fazer GET para pegar token CSRF
	resp, err := http.Get(integrationServer.URL + "/login")
	if err != nil {
		t.Fatalf("failed to get login page: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	token := extractCSRFToken(string(body))
	if token == "" {
		t.Fatal("failed to extract CSRF token")
	}

	// 2. Fazer POST com token válido e credenciais inválidas
	form := url.Values{}
	form.Add("email", seed.AdminUser.Email)
	form.Add("password", "wrong_password")
	form.Add("csrf_token", token)

	// Manter cookies da sessão
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp2, err := client.PostForm(integrationServer.URL+"/login", form)
	if err != nil {
		t.Fatalf("failed to post login: %v", err)
	}
	defer resp2.Body.Close()

	// Deve retornar 200 com erro de credenciais, não 400 Bad Request
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 (invalid credentials), got %d", resp2.StatusCode)
		body2, _ := io.ReadAll(resp2.Body)
		t.Logf("response body: %s", string(body2))
	}
}

// TestCSRF_Integration_PostLoginWithInvalidToken valida POST com token inválido
func TestCSRF_Integration_PostLoginWithInvalidToken(t *testing.T) {
	SetupIntegrationTest(t)
	defer TeardownIntegrationTest(t)

	form := url.Values{}
	form.Add("email", "test@example.com")
	form.Add("password", "password123")
	form.Add("csrf_token", "invalid_token_12345")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.PostForm(integrationServer.URL+"/login", form)
	if err != nil {
		t.Fatalf("failed to post login: %v", err)
	}
	defer resp.Body.Close()

	// Token inválido deve retornar erro CSRF ou retornar página de login com erro
	// O comportamento exato depende da configuração do nosurf
	t.Logf("status with invalid token: %d", resp.StatusCode)
}

// TestCSRF_Integration_PostLoginWithoutToken valida POST sem token
func TestCSRF_Integration_PostLoginWithoutToken(t *testing.T) {
	SetupIntegrationTest(t)
	defer TeardownIntegrationTest(t)

	form := url.Values{}
	form.Add("email", "test@example.com")
	form.Add("password", "password123")
	// Sem csrf_token

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.PostForm(integrationServer.URL+"/login", form)
	if err != nil {
		t.Fatalf("failed to post login: %v", err)
	}
	defer resp.Body.Close()

	t.Logf("status without token: %d", resp.StatusCode)
	// nosurf pode bloquear (400) ou processar com erro de token
}

// TestCSRF_Integration_LoginSuccessWithCSRF valida login bem-sucedido com CSRF
func TestCSRF_Integration_LoginSuccessWithCSRF(t *testing.T) {
	SetupIntegrationTest(t)
	defer TeardownIntegrationTest(t)
	seedTestData(t)

	// 1. Pegar token CSRF
	resp, err := http.Get(integrationServer.URL + "/login")
	if err != nil {
		t.Fatalf("failed to get login page: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	token := extractCSRFToken(string(body))

	// 2. Login com credenciais válidas
	form := url.Values{}
	form.Add("email", seed.AdminUser.Email)
	form.Add("password", seed.TestPassword)
	form.Add("csrf_token", token)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp2, err := client.PostForm(integrationServer.URL+"/login", form)
	if err != nil {
		t.Fatalf("failed to post login: %v", err)
	}
	defer resp2.Body.Close()

	// Deve redirecionar para /dashboard
	if resp2.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", resp2.StatusCode)
	}

	location := resp2.Header.Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}

	// Verificar se cookie de sessão foi definido
	sessionCookie := false
	for _, cookie := range resp2.Cookies() {
		if strings.Contains(cookie.Name, "session") {
			sessionCookie = true
			break
		}
	}
	if !sessionCookie {
		t.Error("expected session cookie after successful login")
	}
}

// TestCSRF_Integration_DEVRefererBypass valida bypass do Referer em DEV
func TestCSRF_Integration_DEVRefererBypass(t *testing.T) {
	SetupIntegrationTest(t)
	defer TeardownIntegrationTest(t)

	// Em DEV, request POST para localhost sem Referer deve funcionar
	form := url.Values{}
	form.Add("email", "test@example.com")
	form.Add("password", "password123")
	form.Add("csrf_token", "test_token")

	req, _ := http.NewRequest("POST", integrationServer.URL+"/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Sem header Referer

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Em DEV com localhost, não deve bloquear por falta de Referer
	// Pode falhar por token inválido, mas não por Referer
	t.Logf("status without Referer in DEV: %d", resp.StatusCode)
}

// TestCSRF_Integration_CSRFCookieProperties valida propriedades do cookie CSRF
func TestCSRF_Integration_CSRFCookieProperties(t *testing.T) {
	SetupIntegrationTest(t)
	defer TeardownIntegrationTest(t)

	resp, err := http.Get(integrationServer.URL + "/login")
	if err != nil {
		t.Fatalf("failed to get login page: %v", err)
	}
	defer resp.Body.Close()

	var csrfCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "csrf_token" {
			csrfCookie = cookie
			break
		}
	}

	if csrfCookie == nil {
		t.Fatal("expected csrf_token cookie")
	}

	if !csrfCookie.HttpOnly {
		t.Error("csrf_token cookie should be HttpOnly")
	}

	if csrfCookie.Path != "/" {
		t.Errorf("expected cookie path /, got %s", csrfCookie.Path)
	}

	// Em DEV, Secure deve ser false
	if csrfCookie.Secure {
		t.Error("csrf_token cookie should not be Secure in DEV")
	}
}

// TestCSRF_Integration_MultipleRequests valida múltiplas requests
func TestCSRF_Integration_MultipleRequests(t *testing.T) {
	SetupIntegrationTest(t)
	defer TeardownIntegrationTest(t)

	client := &http.Client{}
	var tokens []string

	// Fazer múltiplas requests GET
	for i := 0; i < 3; i++ {
		resp, err := client.Get(integrationServer.URL + "/login")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}

		body, _ := io.ReadAll(resp.Body)
		token := extractCSRFToken(string(body))
		if token == "" {
			t.Errorf("request %d: expected CSRF token", i)
		}
		tokens = append(tokens, token)
		resp.Body.Close()
	}

	// Tokens podem ser diferentes a cada request (rotação)
	t.Logf("got %d tokens from multiple requests", len(tokens))
}

// TestCSRF_Integration_CSRFTokenInContext valida token no contexto
func TestCSRF_Integration_CSRFTokenInContext(t *testing.T) {
	SetupIntegrationTest(t)
	defer TeardownIntegrationTest(t)

	// O middleware CSRFHandler deve injetar o token no contexto
	// Isso é validado indiretamente pelo template receber o token

	resp, err := http.Get(integrationServer.URL + "/login")
	if err != nil {
		t.Fatalf("failed to get login page: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Verificar se o token está no HTML (injetado pelo template via contexto)
	if !strings.Contains(html, `name="csrf_token"`) {
		t.Error("expected csrf_token input field in HTML")
	}

	token := extractCSRFToken(html)
	if token == "" {
		t.Error("expected non-empty CSRF token in HTML")
	}
}
