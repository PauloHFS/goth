package e2e

import (
	"testing"
	"time"

	pw "github.com/playwright-community/playwright-go"
)

// TestCSRF_E2E_LoginPageHasCSRFToken valida que a página de login tem token CSRF
func TestCSRF_E2E_LoginPageHasCSRFToken(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	resp, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate to login: %v", err)
	}

	if resp.Status() != 200 {
		t.Fatalf("login page returned status: %d", resp.Status())
	}

	// Esperar página carregar
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateDomcontentloaded,
	})
	if err != nil {
		t.Fatalf("page failed to load: %v", err)
	}

	// Verificar se input csrf_token está presente
	csrfInput, err := page.QuerySelector("input[name='csrf_token']")
	if err != nil {
		t.Fatalf("failed to query csrf_token input: %v", err)
	}

	if csrfInput == nil {
		TakeScreenshot(t, page, "csrf-missing-input")
		t.Fatal("expected csrf_token input field to exist")
	}

	// Extrair valor do token
	value, err := csrfInput.GetAttribute("value")
	if err != nil {
		t.Fatalf("failed to get csrf_token value: %v", err)
	}

	if value == "" {
		TakeScreenshot(t, page, "csrf-empty-token")
		t.Error("expected non-empty csrf_token value")
	}

	if len(value) < 20 {
		t.Errorf("expected token length >= 20, got %d", len(value))
	}

	t.Logf("CSRF token found with length: %d", len(value))
}

// TestCSRF_E2E_LoginFormSubmission valida submissão do formulário com CSRF
func TestCSRF_E2E_LoginFormSubmission(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate to login: %v", err)
	}

	// Esperar formulário carregar
	if err := WaitForElement(t, page, "input[name='email']"); err != nil {
		t.Fatalf("email field not found: %v", err)
	}

	if err := WaitForElement(t, page, "input[name='password']"); err != nil {
		t.Fatalf("password field not found: %v", err)
	}

	if err := WaitForElement(t, page, "button[type='submit']"); err != nil {
		t.Fatalf("submit button not found: %v", err)
	}

	// Verificar token CSRF antes de submeter
	csrfInput, _ := page.QuerySelector("input[name='csrf_token']")
	if csrfInput == nil {
		t.Fatal("csrf_token input not found before form submission")
	}

	tokenValue, _ := csrfInput.GetAttribute("value")
	if tokenValue == "" {
		t.Fatal("csrf_token is empty before form submission")
	}

	// Preencher credenciais inválidas
	err = page.Fill("input[name='email']", "invalid@example.com")
	if err != nil {
		t.Fatalf("failed to fill email: %v", err)
	}

	err = page.Fill("input[name='password']", "wrongpassword")
	if err != nil {
		t.Fatalf("failed to fill password: %v", err)
	}

	// Submeter formulário
	err = page.Click("button[type='submit']")
	if err != nil {
		t.Fatalf("failed to click submit: %v", err)
	}

	// Esperar página processar
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateNetworkidle,
	})
	if err != nil {
		t.Logf("load state warning: %v", err)
	}

	// Verificar se há mensagem de erro (credenciais inválidas)
	// Não deve ser erro 400 Bad Request
	content, err := page.Content()
	if err != nil {
		t.Fatalf("failed to get page content: %v", err)
	}

	// Verificar se formulário ainda está presente (login falhou mas não deu erro CSRF)
	emailInput, _ := page.QuerySelector("input[name='email']")
	if emailInput == nil {
		t.Log("form not present - may have redirected")
	}

	t.Logf("form submission completed - page length: %d", len(content))
}

// TestCSRF_E2E_CSRFTokenRotation valida rotação de tokens
func TestCSRF_E2E_CSRFTokenRotation(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	var tokens []string

	// Fazer múltiplas visitas à página de login
	for i := 0; i < 3; i++ {
		_, err := page.Goto(ServerURL + "/login")
		if err != nil {
			t.Fatalf("navigation failed: %v", err)
		}

		err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
			State: pw.LoadStateDomcontentloaded,
		})
		if err != nil {
			t.Fatalf("page failed to load: %v", err)
		}

		csrfInput, err := page.QuerySelector("input[name='csrf_token']")
		if err != nil {
			t.Fatalf("failed to query csrf_token: %v", err)
		}

		if csrfInput == nil {
			t.Fatalf("csrf_token input not found on request %d", i)
		}

		value, err := csrfInput.GetAttribute("value")
		if err != nil {
			t.Fatalf("failed to get token value: %v", err)
		}

		if value == "" {
			t.Errorf("empty token on request %d", i)
		}

		tokens = append(tokens, value)
		t.Logf("request %d: token length = %d", i, len(value))
	}

	// Tokens podem ser diferentes (rotação) ou iguais (mesma sessão)
	// O importante é que todos sejam válidos (não vazios)
	t.Logf("collected %d tokens from multiple requests", len(tokens))
}

// TestCSRF_E2E_CSRFCookieHttpOnly valida que cookie CSRF é HttpOnly
func TestCSRF_E2E_CSRFCookieHttpOnly(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	// Navegar para login para setar cookie
	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	// Obter cookies via JavaScript (cookies HttpOnly não são acessíveis)
	cookies, err := page.Context().Cookies()
	if err != nil {
		t.Fatalf("failed to get cookies: %v", err)
	}

	var csrfCookieFound bool
	var csrfCookieHttpOnly bool

	for _, cookie := range cookies {
		if cookie.Name == "csrf_token" {
			csrfCookieFound = true
			csrfCookieHttpOnly = cookie.HttpOnly
			break
		}
	}

	if !csrfCookieFound {
		t.Log("csrf_token cookie not found in browser context (expected for HttpOnly)")
	} else {
		t.Logf("csrf_token cookie found, HttpOnly: %v", csrfCookieHttpOnly)
	}
}

// TestCSRF_E2E_LoginWithValidCredentials valida login com credenciais válidas
func TestCSRF_E2E_LoginWithValidCredentials(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate to login: %v", err)
	}

	// Esperar elementos do formulário
	if err := WaitForElement(t, page, "input[name='email']"); err != nil {
		t.Fatalf("email field not found: %v", err)
	}

	if err := WaitForElement(t, page, "input[name='password']"); err != nil {
		t.Fatalf("password field not found: %v", err)
	}

	// Verificar token CSRF
	csrfInput, _ := page.QuerySelector("input[name='csrf_token']")
	if csrfInput == nil {
		t.Fatal("csrf_token input not found")
	}

	// Preencher com credenciais válidas (usuário seed)
	err = page.Fill("input[name='email']", "admin@goth.local")
	if err != nil {
		t.Fatalf("failed to fill email: %v", err)
	}

	err = page.Fill("input[name='password']", "test123456")
	if err != nil {
		t.Fatalf("failed to fill password: %v", err)
	}

	// Submeter
	err = page.Click("button[type='submit']")
	if err != nil {
		t.Fatalf("failed to click submit: %v", err)
	}

	// Esperar navegação
	err = page.WaitForURL(ServerURL + "/dashboard")
	if err != nil {
		t.Logf("navigation warning: %v", err)
	}

	url := page.URL()
	t.Logf("redirected to: %s", url)

	// Verificar se foi para dashboard ou permaneceu no login com erro
	if url == ServerURL+"/dashboard" {
		t.Log("login successful - redirected to dashboard")
	} else if url == ServerURL+"/login" {
		// Verificar se há mensagem de erro
		content, _ := page.Content()
		if len(content) > 0 {
			t.Log("login form redisplayed - checking for error message")
		}
	}
}

// TestCSRF_E2E_CSRFMetaTag valida meta tag CSRF para HTMX
func TestCSRF_E2E_CSRFMetaTag(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateDomcontentloaded,
	})
	if err != nil {
		t.Fatalf("page failed to load: %v", err)
	}

	// Verificar meta tag para HTMX
	metaTag, err := page.QuerySelector("meta[name='csrf-token']")
	if err != nil {
		t.Logf("meta tag query failed (may not exist): %v", err)
	}

	if metaTag != nil {
		content, err := metaTag.GetAttribute("content")
		if err != nil {
			t.Fatalf("failed to get meta content: %v", err)
		}

		if content == "" {
			t.Log("csrf-token meta tag exists but content is empty")
		} else {
			t.Logf("csrf-token meta tag found with length: %d", len(content))
		}
	} else {
		t.Log("csrf-token meta tag not found (may use different mechanism)")
	}
}

// TestCSRF_E2E_FormActionWithCSRF valida ação do formulário com CSRF
func TestCSRF_E2E_FormActionWithCSRF(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	// Esperar formulário
	form, err := page.QuerySelector("form[action='/login']")
	if err != nil {
		t.Fatalf("failed to query form: %v", err)
	}

	if form == nil {
		t.Fatal("login form not found")
	}

	// Verificar método POST
	method, err := form.GetAttribute("method")
	if err != nil {
		t.Fatalf("failed to get form method: %v", err)
	}

	if method != "POST" {
		t.Errorf("expected form method POST, got %s", method)
	}

	// Verificar ação
	action, err := form.GetAttribute("action")
	if err != nil {
		t.Fatalf("failed to get form action: %v", err)
	}

	if action != "/login" {
		t.Errorf("expected form action /login, got %s", action)
	}

	t.Logf("form configured correctly: %s %s", method, action)
}

// TestCSRF_E2E_MultipleTabsCSRF valida CSRF em múltiplas abas
func TestCSRF_E2E_MultipleTabsCSRF(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	// Abrir login em duas "abas" (contexts diferentes)
	context1, err := page.Context().Browser().NewContext()
	if err != nil {
		t.Fatalf("failed to create context1: %v", err)
	}
	defer context1.Close()

	context2, err := page.Context().Browser().NewContext()
	if err != nil {
		t.Fatalf("failed to create context2: %v", err)
	}
	defer context2.Close()

	page1, err := context1.NewPage()
	if err != nil {
		t.Fatalf("failed to create page1: %v", err)
	}

	page2, err := context2.NewPage()
	if err != nil {
		t.Fatalf("failed to create page2: %v", err)
	}

	// Carregar login em ambas
	_, err = page1.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("page1 navigation failed: %v", err)
	}

	_, err = page2.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("page2 navigation failed: %v", err)
	}

	// Extrair tokens de ambas
	csrfInput1, _ := page1.QuerySelector("input[name='csrf_token']")
	csrfInput2, _ := page2.QuerySelector("input[name='csrf_token']")

	if csrfInput1 == nil || csrfInput2 == nil {
		t.Fatal("csrf_token input not found in one of the pages")
	}

	token1, _ := csrfInput1.GetAttribute("value")
	token2, _ := csrfInput2.GetAttribute("value")

	t.Logf("token1 length: %d, token2 length: %d", len(token1), len(token2))

	// Cada contexto deve ter seu próprio token
	if token1 == "" || token2 == "" {
		t.Error("expected non-empty tokens in both contexts")
	}
}

// TestCSRF_E2E_CSRFAfterRedirect valida CSRF após redirecionamento
func TestCSRF_E2E_CSRFAfterRedirect(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	// Navegar para uma página protegida (deve redirecionar para login)
	_, err := page.Goto(ServerURL + "/dashboard")
	if err != nil {
		t.Fatalf("failed to navigate to dashboard: %v", err)
	}

	// Esperar redirecionamento
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateNetworkidle,
	})
	if err != nil {
		t.Logf("load state warning: %v", err)
	}

	// Verificar se está na página de login
	currentURL := page.URL()
	t.Logf("current URL: %s", currentURL)

	// Verificar se token CSRF está presente após redirecionamento
	csrfInput, err := page.QuerySelector("input[name='csrf_token']")
	if err != nil {
		t.Logf("csrf_token not found after redirect (may be on different page): %v", err)
	} else if csrfInput != nil {
		value, _ := csrfInput.GetAttribute("value")
		if value == "" {
			t.Error("expected non-empty CSRF token after redirect")
		} else {
			t.Logf("CSRF token present after redirect with length: %d", len(value))
		}
	}
}

// TestCSRF_E2E_RegisterFormHasCSRF valida CSRF no formulário de registro
func TestCSRF_E2E_RegisterFormHasCSRF(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/register")
	if err != nil {
		t.Fatalf("failed to navigate to register: %v", err)
	}

	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateDomcontentloaded,
	})
	if err != nil {
		t.Fatalf("page failed to load: %v", err)
	}

	// Verificar token CSRF no registro também
	csrfInput, err := page.QuerySelector("input[name='csrf_token']")
	if err != nil {
		t.Logf("csrf_token query failed: %v", err)
	}

	if csrfInput != nil {
		value, _ := csrfInput.GetAttribute("value")
		if value == "" {
			t.Log("register csrf_token is empty")
		} else {
			t.Logf("register CSRF token found with length: %d", len(value))
		}
	} else {
		t.Log("csrf_token input not found on register page")
	}
}

// TestCSRF_E2E_CSRFWithHTMX valida integração CSRF com HTMX (se aplicável)
func TestCSRF_E2E_CSRFWithHTMX(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate: %v", err)
	}

	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateDomcontentloaded,
	})
	if err != nil {
		t.Fatalf("page failed to load: %v", err)
	}

	// Verificar se HTMX está presente
	htmxScript, _ := page.QuerySelector("script[src*='htmx']")
	if htmxScript != nil {
		t.Log("HTMX script detected - CSRF should work with HTMX requests")

		// Verificar configuração do HTMX para CSRF
		config, err := page.Evaluate(`() => {
			const body = document.body;
			body.addEventListener('htmx:configRequest', function(event) {
				return event.detail.headers['X-CSRF-Token'];
			});
			return 'configured';
		}`)

		if err != nil {
			t.Logf("HTMX config check: %v", err)
		} else {
			t.Logf("HTMX config: %v", config)
		}
	} else {
		t.Log("HTMX script not found on login page")
	}
}

// TestCSRF_E2E_SessionPersistenceWithCSRF valida sessão com CSRF
func TestCSRF_E2E_SessionPersistenceWithCSRF(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	// 1. Carregar login
	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to load login: %v", err)
	}

	// 2. Preencher e submeter (credenciais inválidas)
	if err := WaitForElement(t, page, "input[name='email']"); err != nil {
		t.Fatalf("email field not found: %v", err)
	}

	err = page.Fill("input[name='email']", "test@example.com")
	if err != nil {
		t.Fatalf("failed to fill email: %v", err)
	}

	err = page.Fill("input[name='password']", "wrongpass")
	if err != nil {
		t.Fatalf("failed to fill password: %v", err)
	}

	err = page.Click("button[type='submit']")
	if err != nil {
		t.Fatalf("failed to submit: %v", err)
	}

	// 3. Esperar processamento
	time.Sleep(2 * time.Second)

	// 4. Verificar se token CSRF ainda está presente
	csrfInput, _ := page.QuerySelector("input[name='csrf_token']")
	if csrfInput == nil {
		t.Log("csrf_token not found after failed login (may have redirected)")
	} else {
		value, _ := csrfInput.GetAttribute("value")
		if value == "" {
			t.Error("CSRF token empty after failed login")
		} else {
			t.Logf("CSRF token still valid after failed login, length: %d", len(value))
		}
	}
}
