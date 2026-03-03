package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	// globalAtomicLevel é o nível de log global da aplicação
	globalAtomicLevel *AtomicLevel
	levelFilePath     string
	levelMu           sync.RWMutex
)

// InitDynamicLogging inicializa o sistema de log dinâmico
func InitDynamicLogging(levelStr string) error {
	level := ParseLevel(levelStr)
	globalAtomicLevel = NewAtomicLevel(level)

	// Salvar caminho do arquivo de nível
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	levelFilePath = filepath.Join(cwd, ".log_level")

	// Tentar carregar nível salvo
	if err := loadLevelFromFile(); err != nil {
		// Salvar nível inicial
		return saveLevelToFile(levelStr)
	}

	return nil
}

// GetGlobalLevel retorna o nível de log global
func GetGlobalLevel() slog.Level {
	if globalAtomicLevel == nil {
		return slog.LevelInfo
	}
	return globalAtomicLevel.Level()
}

// SetGlobalLevel define o nível de log global
func SetGlobalLevel(levelStr string) error {
	if globalAtomicLevel == nil {
		return nil
	}

	level := ParseLevel(levelStr)
	globalAtomicLevel.SetLevel(level)

	// Salvar em arquivo para persistência
	if err := saveLevelToFile(levelStr); err != nil {
		return err
	}

	// Registrar mudança com tracing
	ctx := context.Background()
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String("log.level", levelStr),
		)
	}

	return nil
}

// saveLevelToFile salva o nível em arquivo para persistência
func saveLevelToFile(level string) error {
	levelMu.Lock()
	defer levelMu.Unlock()

	// Use 0600 for file permissions (owner rw only) - log level config is sensitive
	return os.WriteFile(levelFilePath, []byte(level), 0600)
}

// loadLevelFromFile carrega o nível do arquivo
func loadLevelFromFile() error {
	levelMu.RLock()
	defer levelMu.RUnlock()

	// Prevent path traversal attacks by cleaning the path
	cleanPath := filepath.Clean(levelFilePath)
	if !filepath.IsLocal(cleanPath) {
		return fmt.Errorf("invalid log level file path")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return err
	}

	level := string(data)
	if globalAtomicLevel != nil {
		globalAtomicLevel.SetLevel(ParseLevel(level))
	}

	return nil
}

// LogLevelHandler é o handler HTTP para gestão de log levels
type LogLevelHandler struct{}

// NewLogLevelHandler cria novo handler
func NewLogLevelHandler() *LogLevelHandler {
	return &LogLevelHandler{}
}

// RegisterRoutes registra as rotas da API
func (h *LogLevelHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/log-level", h.getLevel)
	mux.HandleFunc("PUT /api/log-level", h.setLevel)
}

// LevelResponse representa a resposta da API
type LevelResponse struct {
	Level string `json:"level"`
}

// LevelRequest representa a requisição da API
type LevelRequest struct {
	Level string `json:"level"`
}

// getLevel retorna o nível atual
// @Summary Get current log level
// @Description Returns the current application log level
// @Tags Logging
// @Produce json
// @Success 200 {object} LevelResponse
// @Router /api/log-level [get]
func (h *LogLevelHandler) getLevel(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, r, LevelResponse{
		Level: GetGlobalLevel().String(),
	}, http.StatusOK)
}

// setLevel define um novo nível
// @Summary Set log level
// @Description Sets the application log level dynamically
// @Tags Logging
// @Accept json
// @Produce json
// @Param level body LevelRequest true "New log level (debug, info, warn, error)"
// @Success 200 {object} LevelResponse
// @Failure 400 {object} map[string]string
// @Router /api/log-level [put]
func (h *LogLevelHandler) setLevel(w http.ResponseWriter, r *http.Request) {
	var req LevelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	validLevels := map[string]bool{
		"debug":   true,
		"info":    true,
		"warn":    true,
		"warning": true,
		"error":   true,
	}

	if !validLevels[req.Level] {
		respondError(w, r, "Invalid level. Must be one of: debug, info, warn, error", http.StatusBadRequest)
		return
	}

	if err := SetGlobalLevel(req.Level); err != nil {
		respondError(w, r, "Failed to set log level: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, r, LevelResponse(req), http.StatusOK)
}

func respondJSON(w http.ResponseWriter, r *http.Request, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, r *http.Request, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
