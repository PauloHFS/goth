package featureflags

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/PauloHFS/goth/internal/platform/http/middleware"
)

// Handler para feature flags API
type Handler struct {
	manager *Manager
}

// NewHandler cria novo handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registra as rotas da API
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/feature-flags", h.listFlags)
	mux.HandleFunc("GET /api/feature-flags/{name}", h.getFlag)
	mux.HandleFunc("POST /api/feature-flags", h.createFlag)
	mux.HandleFunc("PUT /api/feature-flags/{id}", h.updateFlag)
	mux.HandleFunc("DELETE /api/feature-flags/{id}", h.deleteFlag)
	mux.HandleFunc("POST /api/feature-flags/{id}/toggle", h.toggleFlag)
	mux.HandleFunc("GET /api/feature-flags/check/{name}", h.checkFlag)
}

// listFlags retorna todas as feature flags
// @Summary List all feature flags
// @Description Returns a list of all feature flags for the tenant
// @Tags Feature Flags
// @Produce json
// @Success 200 {array} FeatureFlag
// @Router /api/feature-flags [get]
func (h *Handler) listFlags(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	flags, err := h.manager.GetAll(r.Context(), tenantID)
	if err != nil {
		respondError(w, r, "Failed to list feature flags", http.StatusInternalServerError)
		return
	}

	if flags == nil {
		flags = []FeatureFlag{}
	}

	respondJSON(w, r, flags, http.StatusOK)
}

// getFlag retorna uma feature flag específica
// @Summary Get feature flag by name
// @Description Returns a specific feature flag by name
// @Tags Feature Flags
// @Produce json
// @Param name path string true "Feature flag name"
// @Success 200 {object} FeatureFlag
// @Success 404 {object} map[string]string
// @Router /api/feature-flags/{name} [get]
func (h *Handler) getFlag(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	flag, err := h.manager.GetAll(r.Context(), tenantID)
	if err != nil {
		respondError(w, r, "Failed to get feature flag", http.StatusInternalServerError)
		return
	}

	// Find by name
	var found *FeatureFlag
	for _, f := range flag {
		if f.Name == name {
			found = &f
			break
		}
	}

	if found == nil {
		respondError(w, r, "Feature flag not found", http.StatusNotFound)
		return
	}

	respondJSON(w, r, found, http.StatusOK)
}

// createFlag cria uma nova feature flag
// @Summary Create feature flag
// @Description Creates a new feature flag
// @Tags Feature Flags
// @Accept json
// @Produce json
// @Param flag body FeatureFlagInput true "Feature flag data"
// @Success 201 {object} FeatureFlag
// @Success 400 {object} map[string]string
// @Router /api/feature-flags [post]
func (h *Handler) createFlag(w http.ResponseWriter, r *http.Request) {
	var input FeatureFlagInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validation
	if input.Name == "" {
		respondError(w, r, "Name is required", http.StatusBadRequest)
		return
	}

	if input.TenantID == "" {
		input.TenantID = "default"
	}

	flag, err := h.manager.Create(r.Context(), input)
	if err != nil {
		respondError(w, r, "Failed to create feature flag", http.StatusInternalServerError)
		return
	}

	respondJSON(w, r, flag, http.StatusCreated)
}

// updateFlag atualiza uma feature flag
// @Summary Update feature flag
// @Description Updates an existing feature flag
// @Tags Feature Flags
// @Accept json
// @Produce json
// @Param id path int true "Feature flag ID"
// @Param flag body FeatureFlagInput true "Feature flag data"
// @Success 200 {object} FeatureFlag
// @Success 400 {object} map[string]string
// @Success 404 {object} map[string]string
// @Router /api/feature-flags/{id} [put]
func (h *Handler) updateFlag(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, r, "Invalid ID", http.StatusBadRequest)
		return
	}

	var input FeatureFlagInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	flag, err := h.manager.Update(r.Context(), id, input)
	if err != nil {
		respondError(w, r, "Failed to update feature flag", http.StatusInternalServerError)
		return
	}

	if flag == nil {
		respondError(w, r, "Feature flag not found", http.StatusNotFound)
		return
	}

	respondJSON(w, r, flag, http.StatusOK)
}

// deleteFlag remove uma feature flag
// @Summary Delete feature flag
// @Description Deletes a feature flag
// @Tags Feature Flags
// @Produce json
// @Param id path int true "Feature flag ID"
// @Success 204
// @Success 404 {object} map[string]string
// @Router /api/feature-flags/{id} [delete]
func (h *Handler) deleteFlag(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, r, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = h.manager.Delete(r.Context(), id)
	if err != nil {
		respondError(w, r, "Failed to delete feature flag", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// toggleFlag alterna o estado de uma feature flag
// @Summary Toggle feature flag
// @Description Toggles a feature flag enabled/disabled state
// @Tags Feature Flags
// @Produce json
// @Param id path int true "Feature flag ID"
// @Success 200 {object} FeatureFlag
// @Success 404 {object} map[string]string
// @Router /api/feature-flags/{id}/toggle [post]
func (h *Handler) toggleFlag(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, r, "Invalid ID", http.StatusBadRequest)
		return
	}

	flag, err := h.manager.Toggle(r.Context(), id)
	if err != nil {
		respondError(w, r, "Failed to toggle feature flag", http.StatusInternalServerError)
		return
	}

	if flag == nil {
		respondError(w, r, "Feature flag not found", http.StatusNotFound)
		return
	}

	respondJSON(w, r, flag, http.StatusOK)
}

// checkFlag verifica se uma feature flag está habilitada
// @Summary Check feature flag status
// @Description Checks if a feature flag is enabled
// @Tags Feature Flags
// @Produce json
// @Param name path string true "Feature flag name"
// @Success 200 {object} map[string]interface{}
// @Router /api/feature-flags/check/{name} [get]
func (h *Handler) checkFlag(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	enabled := h.manager.IsEnabled(r.Context(), name, tenantID)

	respondJSON(w, r, map[string]interface{}{"enabled": enabled, "name": name}, http.StatusOK)
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
