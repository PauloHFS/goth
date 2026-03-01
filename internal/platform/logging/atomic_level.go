package logging

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
)

// AtomicLevel é um nível de log thread-safe que pode ser alterado dinamicamente
type AtomicLevel struct {
	level atomic.Int32
}

// NewAtomicLevel cria um novo AtomicLevel com o nível especificado
func NewAtomicLevel(level slog.Level) *AtomicLevel {
	var al AtomicLevel
	al.level.Store(int32(level))
	return &al
}

// Level retorna o nível atual
func (al *AtomicLevel) Level() slog.Level {
	return slog.Level(al.level.Load())
}

// SetLevel define um novo nível
func (al *AtomicLevel) SetLevel(level slog.Level) {
	al.level.Store(int32(level))
}

// ParseLevel converte string para slog.Level
func ParseLevel(levelStr string) slog.Level {
	switch levelStr {
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

// String converte slog.Level para string
func (al *AtomicLevel) String() string {
	level := al.Level()
	switch level {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// Enabled retorna true se o nível estiver habilitado
func (al *AtomicLevel) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= al.Level()
}

// Handle implementa slog.Handler
func (al *AtomicLevel) Handle(ctx context.Context, record slog.Record) error {
	if !al.Enabled(ctx, record.Level) {
		return nil
	}

	// Usar handler JSON para produção
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: al.Level(),
	})

	return handler.Handle(ctx, record)
}

// WithAttrs retorna um novo handler com atributos adicionais
func (al *AtomicLevel) WithAttrs(attrs []slog.Attr) slog.Handler {
	return al
}

// WithGroup retorna um novo handler com grupo
func (al *AtomicLevel) WithGroup(name string) slog.Handler {
	return al
}
