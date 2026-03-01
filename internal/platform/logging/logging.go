package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

type contextKey string

const (
	eventKey contextKey = "event"
)

type Event struct {
	mu    sync.Mutex
	attrs []slog.Attr
}

func (e *Event) Add(attrs ...slog.Attr) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.attrs = append(e.attrs, attrs...)
}

func (e *Event) Attrs() []any {
	e.mu.Lock()
	defer e.mu.Unlock()
	args := make([]any, len(e.attrs))
	for i, attr := range e.attrs {
		args[i] = attr
	}
	return args
}

// NewLogger cria logger com nível dinâmico
func NewLogger(level string) *slog.Logger {
	logLevel := parseLogLevel(level)

	// Usar atomic level se disponível
	var handler slog.Handler
	if globalAtomicLevel != nil {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: globalAtomicLevel,
		})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})
	}

	version := os.Getenv("VERSION")
	if version == "" {
		version = "dev"
	}

	return slog.New(handler).With(
		slog.String("version", version),
		slog.String("service", "goth-api"),
	)
}

// New é um alias para NewLogger para compatibilidade
func New(level string) *slog.Logger {
	return NewLogger(level)
}

func parseLogLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func NewEventContext(ctx context.Context) (context.Context, *Event) {
	e := &Event{}
	return context.WithValue(ctx, eventKey, e), e
}

func EventFromContext(ctx context.Context) *Event {
	if e, ok := ctx.Value(eventKey).(*Event); ok {
		return e
	}
	return nil
}

func AddToEvent(ctx context.Context, attrs ...slog.Attr) {
	if e := EventFromContext(ctx); e != nil {
		e.Add(attrs...)
	}
}
