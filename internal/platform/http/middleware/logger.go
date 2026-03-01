package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/platform/logging"
	"github.com/PauloHFS/goth/internal/platform/metrics"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// AddLoggerToContext adiciona logger ao contexto
func AddLoggerToContext(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), contextkeys.LoggerKey, logger)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// Logger middleware com tracing correlation
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Pegar Request ID do contexto (já deve ter sido setado pelo middleware RequestID)
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			// Fallback: gerar se não existir (não deveria acontecer)
			requestID = uuid.New().String()
		}

		// Extrair Trace ID do span atual (se existir)
		span := trace.SpanFromContext(r.Context())
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()

		ctx, event := logging.NewEventContext(r.Context())
		event.Add(
			slog.String("request_id", requestID),
			slog.String("trace_id", traceID),
			slog.String("span_id", spanID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
		)

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r.WithContext(ctx))

		duration := time.Since(start)

		event.Add(
			slog.Int("status", rw.status),
			slog.Int("size", rw.size),
			duration_ms(duration),
		)

		// Prometheus metrics
		metrics.HttpRequestsTotal.WithLabelValues(r.URL.Path, r.Method, strconv.Itoa(rw.status)).Inc()

		level := slog.LevelInfo
		if rw.status >= 500 {
			level = slog.LevelError
		}

		logger := getLoggerFromContext(ctx)
		logger.Log(ctx, level, "request completed", event.Attrs()...)
	})
}

func duration_ms(d time.Duration) slog.Attr {
	return slog.Float64("duration_ms", float64(d.Nanoseconds())/1e6)
}

func getLoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(contextkeys.LoggerKey).(*slog.Logger); ok {
		return logger
	}
	return logging.New("info")
}
