package twofactor

import (
	"encoding/json"
	"net/http"
)

// Handler gerencia requisições HTTP para 2FA
type Handler struct {
	service *Service
}

// NewHandler cria um novo handler de 2FA
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterRoutes registra as rotas de 2FA
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /2fa/setup", h.SetupPage)
	mux.HandleFunc("POST /2fa/setup", h.Setup2FA)
	mux.HandleFunc("POST /2fa/enable", h.Enable2FA)
	mux.HandleFunc("POST /2fa/disable", h.Disable2FA)
	mux.HandleFunc("GET /2fa/backup-codes", h.RegenerateBackupCodes)
	mux.HandleFunc("GET /2fa/status", h.GetStatus)
}

// SetupPage exibe a página de configuração de 2FA
func (h *Handler) SetupPage(w http.ResponseWriter, r *http.Request) {
	// Implementar renderização da página de setup
	// components.TwoFASetupPage()
}

// Setup2FA inicia a configuração de 2FA
func (h *Handler) Setup2FA(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)
	userEmail := r.Context().Value("user_email").(string)

	secret, err := h.service.Setup2FA(userID, userEmail)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Retornar dados do QR Code
	qrCodeBase64, err := GenerateQRCodeBase64(secret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"secret":         FormatSecret(secret.Secret),
		"qr_code_base64": qrCodeBase64,
		"issuer":         secret.Issuer,
		"account_name":   secret.AccountName,
		"message":        "Scan the QR code with your authenticator app",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Enable2FA ativa o 2FA
func (h *Handler) Enable2FA(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	var req struct {
		Code string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.Enable2FA(userID, req.Code); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]string{
		"message": "2FA enabled successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Disable2FA desativa o 2FA
func (h *Handler) Disable2FA(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	var req struct {
		Code      string `json:"code"`
		UseBackup bool   `json:"use_backup"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.Disable2FA(userID, req.Code, req.UseBackup); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]string{
		"message": "2FA disabled successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// RegenerateBackupCodes gera novos códigos de backup
func (h *Handler) RegenerateBackupCodes(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	codes, err := h.service.RegenerateBackupCodes(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"codes":   codes,
		"message": "Backup codes regenerated. Store them in a safe place!",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetStatus retorna o status do 2FA
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	status, err := h.service.Get2FAStatus(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
