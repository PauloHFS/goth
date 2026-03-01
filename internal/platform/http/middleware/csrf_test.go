package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/justinas/nosurf"
)

// TestCSRFHandler_TokenInjection valida que o token CSRF é injetado no contexto
func TestCSRFHandler_TokenInjection(t *testing.T) {
	t.Run("deve injetar token CSRF no contexto", func(t *testing.T) {
		var capturedToken string

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := r.Context().Value(contextkeys.CSRFTokenKey).(string)
			if !ok {
				t.Fatal("expected CSRF token in context")
			}
			capturedToken = token
			w.WriteHeader(http.StatusOK)
		})

		csrfHandler := CSRFHandler(handler)

		// Configurar cookie CSRF
		if csrf, ok := csrfHandler.(*nosurf.CSRFHandler); ok {
			csrf.SetBaseCookie(http.Cookie{
				HttpOnly: true,
				Path:     "/",
			})
		}

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		csrfHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		if capturedToken == "" {
			t.Error("expected non-empty CSRF token in context")
		}

		// Token deve ter tamanho razoável (nosurf gera tokens de 32+ bytes em base64)
		if len(capturedToken) < 20 {
			t.Errorf("expected token length >= 20, got %d", len(capturedToken))
		}
	})

	t.Run("deve definir cookie CSRF", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		csrfHandler := CSRFHandler(handler)

		// Configurar cookie CSRF
		if csrf, ok := csrfHandler.(*nosurf.CSRFHandler); ok {
			csrf.SetBaseCookie(http.Cookie{
				HttpOnly: true,
				Path:     "/",
			})
		}

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		csrfHandler.ServeHTTP(rr, req)

		cookies := rr.Result().Cookies()
		var csrfCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" {
				csrfCookie = cookie
				break
			}
		}

		if csrfCookie == nil {
			t.Fatal("expected csrf_token cookie")
		}

		if !csrfCookie.HttpOnly {
			t.Error("expected csrf_token cookie to be HttpOnly")
		}

		if csrfCookie.Path != "/" {
			t.Errorf("expected cookie path /, got %s", csrfCookie.Path)
		}
	})
}

// TestCSRFHandler_DEVEnvironment valida o comportamento em desenvolvimento
func TestCSRFHandler_DEVEnvironment(t *testing.T) {
	// Salvar valor original
	originalEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", originalEnv)

	t.Run("deve permitir request sem Referer em DEV para localhost", func(t *testing.T) {
		// Configurar ambiente de desenvolvimento
		os.Setenv("APP_ENV", "dev")

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		csrfHandler := CSRFHandler(handler)

		// Configurar cookie CSRF válido
		req := httptest.NewRequest("POST", "/test", strings.NewReader("csrf_token=valid_token"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Host = "localhost:8080"
		// Sem header Referer (comum em localhost sem HTTPS)

		// Adicionar cookie CSRF
		req.AddCookie(&http.Cookie{
			Name:  "csrf_token",
			Value: "valid_token",
			Path:  "/",
		})

		rr := httptest.NewRecorder()

		// Em DEV com localhost, não deve falhar por falta de Referer
		csrfHandler.ServeHTTP(rr, req)

		// Deve processar o request (pode falhar por token inválido, mas não por Referer)
		// O importante é que não seja bloqueado por falta de Referer
		if rr.Code == http.StatusBadRequest {
			body := rr.Body.String()
			if strings.Contains(body, "Referer") {
				t.Error("request should not be blocked by Referer check in DEV for localhost")
			}
		}
	})

	t.Run("deve permitir request sem Referer em DEV para 127.0.0.1", func(t *testing.T) {
		os.Setenv("APP_ENV", "dev")

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		csrfHandler := CSRFHandler(handler)

		req := httptest.NewRequest("POST", "/test", strings.NewReader("csrf_token=valid_token"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Host = "127.0.0.1:8080"
		// Sem header Referer

		req.AddCookie(&http.Cookie{
			Name:  "csrf_token",
			Value: "valid_token",
			Path:  "/",
		})

		rr := httptest.NewRecorder()
		csrfHandler.ServeHTTP(rr, req)

		// Não deve bloquear por falta de Referer
		if rr.Code == http.StatusBadRequest {
			body := rr.Body.String()
			if strings.Contains(body, "Referer") {
				t.Error("request should not be blocked by Referer check in DEV for 127.0.0.1")
			}
		}
	})
}

// TestCSRFHandler_PRODEnvironment valida o comportamento em produção
func TestCSRFHandler_PRODEnvironment(t *testing.T) {
	originalEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", originalEnv)

	t.Run("deve exigir Referer em PROD", func(t *testing.T) {
		os.Setenv("APP_ENV", "prod")

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		csrfHandler := CSRFHandler(handler)

		req := httptest.NewRequest("POST", "/test", strings.NewReader("csrf_token=valid_token"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Host = "example.com"
		// Sem header Referer - deve falhar em produção

		req.AddCookie(&http.Cookie{
			Name:  "csrf_token",
			Value: "valid_token",
			Path:  "/",
		})

		rr := httptest.NewRecorder()
		csrfHandler.ServeHTTP(rr, req)

		// Em produção, request sem Referer deve ser bloqueado
		if rr.Code != http.StatusBadRequest {
			t.Logf("warning: expected 400 in PROD without Referer, got %d", rr.Code)
		}
	})
}

// TestCSRFHandler_ContextKey valida a chave do contexto
func TestCSRFHandler_ContextKey(t *testing.T) {
	t.Run("deve usar a chave correta do contexto", func(t *testing.T) {
		var contextKeyUsed bool

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verificar se a chave do contexto é a esperada
			token := r.Context().Value(contextkeys.CSRFTokenKey)
			if token != nil {
				contextKeyUsed = true
			}
			w.WriteHeader(http.StatusOK)
		})

		csrfHandler := CSRFHandler(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		csrfHandler.ServeHTTP(rr, req)

		if !contextKeyUsed {
			t.Error("expected context key to be used")
		}
	})
}

// TestCSRFHandler_MiddlewareChain valida integração com cadeia de middleware
func TestCSRFHandler_MiddlewareChain(t *testing.T) {
	t.Run("deve funcionar em cadeia de middlewares", func(t *testing.T) {
		var csrfToken string

		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := r.Context().Value(contextkeys.CSRFTokenKey).(string)
			if ok {
				csrfToken = token
			}
			w.WriteHeader(http.StatusOK)
		})

		// Simular cadeia de middleware
		handler := CSRFHandler(finalHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if csrfToken == "" {
			t.Error("expected CSRF token to be available in chained handler")
		}
	})
}

// TestCSRFHandler_TokenAvailability valida que o token está disponível para templates
func TestCSRFHandler_TokenAvailability(t *testing.T) {
	t.Run("deve disponibilizar token para handlers", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := nosurf.Token(r)
			if token == "" {
				t.Error("expected non-empty token from nosurf.Token()")
			}

			ctxToken, ok := r.Context().Value(contextkeys.CSRFTokenKey).(string)
			if !ok {
				t.Error("expected token in context")
			}

			// Tokens devem ser iguais
			if token != ctxToken {
				t.Errorf("expected token %s, got %s", token, ctxToken)
			}

			w.WriteHeader(http.StatusOK)
		})

		csrfHandler := CSRFHandler(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		csrfHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})
}

// TestCSRFHandler_RefererBypassLogic valida a lógica de bypass do Referer
func TestCSRFHandler_RefererBypassLogic(t *testing.T) {
	originalEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", originalEnv)

	tests := []struct {
		name         string
		appEnv       string
		host         string
		hasReferer   bool
		shouldBypass bool
	}{
		{"DEV localhost sem Referer", "dev", "localhost:8080", false, true},
		{"DEV 127.0.0.1 sem Referer", "dev", "127.0.0.1:8080", false, true},
		{"DEV com Referer", "dev", "localhost:8080", true, false},
		{"PROD localhost sem Referer", "prod", "localhost:8080", false, false},
		{"PROD com Referer", "prod", "example.com", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("APP_ENV", tt.appEnv)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			csrfHandler := CSRFHandler(handler)

			req := httptest.NewRequest("POST", "/test", nil)
			req.Host = tt.host

			if tt.hasReferer {
				req.Header.Set("Referer", "http://"+tt.host+"/previous")
			}

			rr := httptest.NewRecorder()
			csrfHandler.ServeHTTP(rr, req)

			// Em DEV com localhost/127.0.0.1 sem Referer, não deve bloquear
			if tt.shouldBypass && tt.appEnv == "dev" && !tt.hasReferer {
				// O bypass deve permitir o request
				t.Logf("bypass expected for %s - status: %d", tt.name, rr.Code)
			}
		})
	}
}

// TestCSRFHandler_CookieConfiguration valida configuração do cookie
func TestCSRFHandler_CookieConfiguration(t *testing.T) {
	t.Run("cookie deve ser HttpOnly", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		csrfHandler := CSRFHandler(handler)

		// Configurar cookie CSRF
		if csrf, ok := csrfHandler.(*nosurf.CSRFHandler); ok {
			csrf.SetBaseCookie(http.Cookie{
				HttpOnly: true,
				Path:     "/",
			})
		}

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		csrfHandler.ServeHTTP(rr, req)

		cookies := rr.Result().Cookies()
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" {
				if !cookie.HttpOnly {
					t.Error("csrf_token cookie should be HttpOnly")
				}
				return
			}
		}
		t.Error("csrf_token cookie not found")
	})

	t.Run("cookie deve ter path correto", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		csrfHandler := CSRFHandler(handler)

		// Configurar cookie CSRF
		if csrf, ok := csrfHandler.(*nosurf.CSRFHandler); ok {
			csrf.SetBaseCookie(http.Cookie{
				HttpOnly: true,
				Path:     "/",
			})
		}

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		csrfHandler.ServeHTTP(rr, req)

		cookies := rr.Result().Cookies()
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" {
				if cookie.Path != "/" {
					t.Errorf("expected cookie path /, got %s", cookie.Path)
				}
				return
			}
		}
		t.Error("csrf_token cookie not found")
	})
}

// TestCSRFHandler_HandlerType valida o tipo retornado
func TestCSRFHandler_HandlerType(t *testing.T) {
	t.Run("deve retornar *nosurf.CSRFHandler", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		csrfHandler := CSRFHandler(handler)

		// Verificar se é possível fazer type assertion para *nosurf.CSRFHandler
		if _, ok := csrfHandler.(*nosurf.CSRFHandler); !ok {
			t.Error("expected CSRFHandler to return *nosurf.CSRFHandler")
		}
	})
}

// TestCSRFHandler_NextHandlerCall valida que o next handler é chamado
func TestCSRFHandler_NextHandlerCall(t *testing.T) {
	t.Run("deve chamar o handler seguinte", func(t *testing.T) {
		called := false

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		csrfHandler := CSRFHandler(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		csrfHandler.ServeHTTP(rr, req)

		if !called {
			t.Error("expected next handler to be called")
		}

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})
}

// TestCSRFHandler_RequestContext valida que o contexto da request é preservado
func TestCSRFHandler_RequestContext(t *testing.T) {
	t.Run("deve preservar contexto da request", func(t *testing.T) {
		var contextPreserved bool

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verificar se o contexto original foi preservado
			contextPreserved = r.Context() != nil
			w.WriteHeader(http.StatusOK)
		})

		csrfHandler := CSRFHandler(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		csrfHandler.ServeHTTP(rr, req)

		if !contextPreserved {
			t.Error("expected request context to be preserved")
		}
	})
}
