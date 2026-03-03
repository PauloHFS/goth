package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/justinas/nosurf"
)

// CSRFHandler cria um middleware CSRF com injeção automática de token no contexto.
//
// Arquitetura:
//
//	Request → nosurf → injectHandler → mux → Handler Final
//	             ↓            ↓
//	         Valida CSRF    Injeta token
//	         Cookie         no contexto
//	         Referer*       (*bypass em dev)
//
// Comportamento por ambiente:
//   - Desenvolvimento (APP_ENV=dev): Bypass do Referer para localhost/127.0.0.1
//   - Produção (APP_ENV=prod): Validação completa (Referer + Token)
//
// ⚠️ IMPORTANTE: Em produção, é OBRIGATÓRIO o uso de HTTPS para que o header
// Referer seja enviado pelo browser. Sem HTTPS, o CSRF validation falhará.
func CSRFHandler(next http.Handler) http.Handler {
	// Handler que injeta o token no contexto APÓS o nosurf processar
	injectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := nosurf.Token(r)
		ctx := context.WithValue(r.Context(), contextkeys.CSRFTokenKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})

	// nosurf envolve injectHandler
	csrf := nosurf.New(injectHandler)

	// Configurar para aceitar token do form
	csrf.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log de erro CSRF para debug
		http.Error(w, "CSRF token validation failed", http.StatusBadRequest)
	}))

	// Bypass do Referer APENAS em desenvolvimento (APP_ENV=dev)
	// Isso é necessário porque browsers não enviam Referer em localhost sem HTTPS
	if os.Getenv("APP_ENV") == "dev" {
		csrf.ExemptFunc(func(r *http.Request) bool {
			if r.Referer() == "" {
				host := r.Host
				return strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1")
			}
			return false
		})
	}

	return csrf
}
