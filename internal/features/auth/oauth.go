package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/platform/config"
	"github.com/PauloHFS/goth/internal/platform/httpclient"
	"github.com/PauloHFS/goth/internal/routes"
	"github.com/alexedwards/scs/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

type OAuthProvider struct {
	Name         string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

func NewGoogleProvider(cfg *config.Config) *OAuthProvider {
	return &OAuthProvider{
		Name:         "google",
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
	}
}

func (p *OAuthProvider) Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		Scopes:       p.Scopes,
		Endpoint:     google.Endpoint,
	}
}

type OAuthHandler struct {
	queries *db.Queries
	cfg     *config.Config
	logger  *slog.Logger
	client  *httpclient.Client
}

func NewOAuthHandler(queries *db.Queries, cfg *config.Config, logger *slog.Logger) *OAuthHandler {
	// Criar client com circuit breaker para Google API
	clientConfig := httpclient.DefaultClientConfig("google-oauth")
	clientConfig.Timeout = 15 * time.Second
	clientConfig.CircuitBreaker.MaxFailures = 5
	clientConfig.CircuitBreaker.Timeout = 30 * time.Second
	clientConfig.Retry.MaxRetries = 2

	return &OAuthHandler{
		queries: queries,
		cfg:     cfg,
		logger:  logger,
		client:  httpclient.NewClient(clientConfig),
	}
}

func (h *OAuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request, provider *OAuthProvider) {
	state, err := generateStateToken(r.Context(), h.cfg.SessionSecret)
	if err != nil {
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}

	url := provider.Config().AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *OAuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request, provider *OAuthProvider, sm *scs.SessionManager, tenantID string) error {
	state := r.URL.Query().Get("state")
	if state == "" {
		return fmt.Errorf("missing state parameter")
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		return fmt.Errorf("missing code parameter")
	}

	token, err := provider.Config().Exchange(r.Context(), code)
	if err != nil {
		h.logger.Error("failed to exchange token", "error", err)
		return err
	}

	userInfo, err := h.getGoogleUserInfo(r.Context(), token.AccessToken)
	if err != nil {
		h.logger.Error("failed to get user info", "error", err)
		return err
	}

	user, err := h.queries.GetUserByProvider(r.Context(), db.GetUserByProviderParams{
		TenantID:   tenantID,
		Provider:   sql.NullString{String: "google", Valid: true},
		ProviderID: sql.NullString{String: userInfo.ID, Valid: true},
	})

	if err == sql.ErrNoRows {
		user, err = h.queries.CreateUserWithOAuth(r.Context(), db.CreateUserWithOAuthParams{
			TenantID:     tenantID,
			Email:        userInfo.Email,
			Provider:     sql.NullString{String: "google", Valid: true},
			ProviderID:   sql.NullString{String: userInfo.ID, Valid: true},
			PasswordHash: "",
			RoleID:       "user",
			IsVerified:   userInfo.VerifiedEmail,
			AvatarUrl:    sql.NullString{String: userInfo.Picture, Valid: true},
		})
		if err != nil {
			h.logger.Error("failed to create user", "error", err)
			return err
		}
	} else if err != nil {
		h.logger.Error("failed to get user", "error", err)
		return err
	} else {
		_, err = h.queries.UpdateUserWithOAuth(r.Context(), db.UpdateUserWithOAuthParams{
			Provider:   sql.NullString{String: "google", Valid: true},
			ProviderID: sql.NullString{String: userInfo.ID, Valid: true},
			AvatarUrl:  sql.NullString{String: userInfo.Picture, Valid: true},
			IsVerified: userInfo.VerifiedEmail,
			TenantID:   tenantID,
			Email:      userInfo.Email,
		})
		if err != nil {
			h.logger.Error("failed to update user", "error", err)
		}
	}

	sm.Put(r.Context(), "user_id", user.ID)
	http.Redirect(w, r, routes.Dashboard, http.StatusTemporaryRedirect)
	return nil
}

func (h *OAuthHandler) getGoogleUserInfo(ctx context.Context, accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Usar client com circuit breaker e retry
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google oauth: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// GetHTTPClient retorna o client HTTP com circuit breaker para métricas
func (h *OAuthHandler) GetHTTPClient() *httpclient.Client {
	return h.client
}

func generateStateToken(ctx context.Context, secret string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	hash := sha256.Sum256(append(b, []byte(secret)...))
	return base64.URLEncoding.EncodeToString(hash[:]), nil
}
