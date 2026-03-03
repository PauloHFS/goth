package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
)

// SessionTimeoutConfig configura o timeout de sessão por inatividade
type SessionTimeoutConfig struct {
	// Timeout define o tempo máximo de inatividade
	Timeout time.Duration
	// WarningThreshold define quando avisar o usuário (antes do timeout)
	WarningThreshold time.Duration
	// ExtendOnRequest estende o timeout a cada request
	ExtendOnRequest bool
	// SkipPaths define paths que não estendem o timeout
	SkipPaths []string
}

// DefaultSessionTimeoutConfig retorna a configuração padrão
// 30 minutos de timeout, alerta com 5 minutos de antecedência
func DefaultSessionTimeoutConfig() SessionTimeoutConfig {
	return SessionTimeoutConfig{
		Timeout:          30 * time.Minute,
		WarningThreshold: 5 * time.Minute,
		ExtendOnRequest:  true,
		SkipPaths:        []string{"/health", "/ready", "/metrics", "/static/", "/assets/"},
	}
}

// SessionTimeout middleware controla timeout de sessão por inatividade
// Compatível com alexedwards/scs/v2
func SessionTimeout(sessionManager *scs.SessionManager, config SessionTimeoutConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip paths configurados
			for _, path := range config.SkipPaths {
				if len(path) > 0 && len(r.URL.Path) >= len(path) {
					if r.URL.Path[:len(path)] == path {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			ctx := r.Context()
			now := time.Now()

			// Pegar último activity timestamp da sessão
			lastActivity := sessionManager.GetTime(ctx, "last_activity")

			// Se não existe, setar agora
			if lastActivity.IsZero() {
				sessionManager.Put(ctx, "last_activity", now)
				next.ServeHTTP(w, r)
				return
			}

			// Calcular tempo de inatividade
			idleTime := now.Sub(lastActivity)

			// Verificar se excedeu o timeout
			if idleTime > config.Timeout {
				// Sessão expirou por inatividade
				if err := sessionManager.Destroy(ctx); err != nil {
					http.Error(w, "Session error", http.StatusInternalServerError)
					return
				}

				// Redirect para login com mensagem
				http.Redirect(w, r, "/login?timeout=1", http.StatusTemporaryRedirect)
				return
			}

			// Verificar se está perto do timeout (warning)
			if idleTime > config.Timeout-config.WarningThreshold {
				// Adicionar header de warning para frontend
				w.Header().Set("X-Session-Warning", "true")
				w.Header().Set("X-Session-Remaining",
					fmt.Sprintf("%d", int((config.Timeout-idleTime).Seconds())))
			}

			// Estender timeout se configurado
			if config.ExtendOnRequest {
				sessionManager.Put(ctx, "last_activity", now)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetSessionWarning verifica se a sessão está perto de expirar
func GetSessionWarning(ctx context.Context, sessionManager *scs.SessionManager, warningThreshold time.Duration) bool {
	lastActivity := sessionManager.GetTime(ctx, "last_activity")
	if lastActivity.IsZero() {
		return false
	}

	idleTime := time.Since(lastActivity)
	return idleTime > warningThreshold
}

// GetSessionRemainingSeconds retorna segundos restantes antes do timeout
func GetSessionRemainingSeconds(ctx context.Context, sessionManager *scs.SessionManager, timeout time.Duration) int {
	lastActivity := sessionManager.GetTime(ctx, "last_activity")
	if lastActivity.IsZero() {
		return int(timeout.Seconds())
	}

	remaining := timeout - time.Since(lastActivity)
	if remaining < 0 {
		return 0
	}

	return int(remaining.Seconds())
}
