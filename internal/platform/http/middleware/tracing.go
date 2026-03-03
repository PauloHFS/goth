package middleware

import (
	"net/http"

	"github.com/PauloHFS/goth/internal/platform/observability/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

// Tracing middleware cria um span de tracing para cada requisição HTTP
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extrair contexto de tracing dos headers HTTP (se existir)
		carrier := propagation.HeaderCarrier(r.Header)
		ctx := tracing.ExtractContext(r.Context(), carrier)

		// Criar novo span para a requisição
		ctx, span := tracing.StartSpan(ctx, "HTTP "+r.Method)
		defer span.End()

		// Adicionar atributos da requisição
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.host", r.Host),
			attribute.String("http.route", r.URL.Path),
			attribute.String("user_agent", r.UserAgent()),
		)

		// Chamar próximo handler com contexto atualizado
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
