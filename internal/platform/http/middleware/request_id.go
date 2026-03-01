package middleware

import (
	"context"
	"net/http"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/google/uuid"
)

// RequestID middleware extrai ou gera um X-Request-ID e injeta no contexto.
// Se o Traefik já enviou um X-Request-ID (via headers), usa esse.
// Caso contrário, gera um novo UUID.
//
// O Request ID é:
// 1. Adicionado ao contexto para logging e tracing
// 2. Retornado no header da resposta para correlation
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Tentar pegar X-Request-ID do Traefik (ou cliente)
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			// Gerar novo UUID se não existir
			requestID = uuid.New().String()
		}

		// Injetar no contexto
		ctx := context.WithValue(r.Context(), contextkeys.RequestIDKey, requestID)

		// Adicionar header de resposta para correlation
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extrai o Request ID do contexto de forma type-safe
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(contextkeys.RequestIDKey).(string); ok {
		return id
	}
	return ""
}
