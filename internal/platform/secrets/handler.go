package secrets

import (
	"encoding/json"
	"net/http"

	"github.com/PauloHFS/goth/internal/platform/http/middleware"
)

// Handler para gestão de segredos
type Handler struct {
	manager *Manager
	env     string
}

// NewHandler cria novo handler
func NewHandler(manager *Manager, env string) *Handler {
	return &Handler{
		manager: manager,
		env:     env,
	}
}

// RegisterRoutes registra rotas da API
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/secrets/status", h.getStatus)
	mux.HandleFunc("POST /api/v1/secrets/reload", h.reloadSecrets)
	mux.HandleFunc("POST /api/v1/secrets/rotate", h.rotateSecret)
}

// getStatus retorna status do secret manager
// @Summary Get secrets status
// @Description Returns the current status of the secret manager
// @Tags Secrets
// @Produce json
// @Success 200 {object} Status
// @Router /api/v1/secrets/status [get]
func (h *Handler) getStatus(w http.ResponseWriter, r *http.Request) {
	status := h.manager.GetStatus(h.env)
	respondJSON(w, r, status, http.StatusOK)
}

// reloadSecrets força reload manual dos segredos
// @Summary Reload secrets
// @Description Forces a manual reload of secrets from .env file
// @Tags Secrets
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/secrets/reload [post]
func (h *Handler) reloadSecrets(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.ManualReload(); err != nil {
		respondError(w, r, "Failed to reload secrets: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, r, map[string]string{"message": "Secrets reloaded successfully"}, http.StatusOK)
}

// rotateSecret rotaciona um segredo específico
// @Summary Rotate secret
// @Description Rotates a specific secret (in-memory only)
// @Tags Secrets
// @Accept json
// @Produce json
// @Param secret body RotateRequest true "Secret type and new value"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v1/secrets/rotate [post]
func (h *Handler) rotateSecret(w http.ResponseWriter, r *http.Request) {
	var req RotateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		respondError(w, r, "Secret type is required", http.StatusBadRequest)
		return
	}

	if req.Value == "" {
		respondError(w, r, "Secret value is required", http.StatusBadRequest)
		return
	}

	// Validar tipo de segredo
	validTypes := h.manager.GetAllTypes()
	found := false
	for _, t := range validTypes {
		if string(t) == req.Type {
			found = true
			break
		}
	}

	if !found {
		respondError(w, r, "Invalid secret type", http.StatusBadRequest)
		return
	}

	// Rotacionar segredo
	h.manager.RotateSecret(SecretType(req.Type), req.Value)

	respondJSON(w, r, map[string]string{
		"message": "Secret rotated successfully (in-memory only, update .env to persist)",
		"type":    req.Type,
	}, http.StatusOK)
}

// RotateRequest representa request para rotação de segredo
type RotateRequest struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Helper functions
func respondJSON(w http.ResponseWriter, r *http.Request, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", middleware.GetRequestID(r.Context()))
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, r *http.Request, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", middleware.GetRequestID(r.Context()))
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
