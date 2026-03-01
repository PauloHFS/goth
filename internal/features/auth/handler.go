package auth

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	httpErr "github.com/PauloHFS/goth/internal/platform/http"
	"github.com/PauloHFS/goth/internal/platform/metrics"
	"github.com/PauloHFS/goth/internal/routes"
	"github.com/PauloHFS/goth/internal/view/pages"
	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
)

type Handler struct {
	service     *Service
	session     *scs.SessionManager
	db          *sql.DB
	queries     *db.Queries
	auditLogger AuditLogger
}

// AuditLogger interface para logging de auditoria
type AuditLogger interface {
	LogLogin(ctx context.Context, userID int64, success bool, ipAddress, userAgent string)
	LogLogout(ctx context.Context, userID int64, ipAddress, userAgent string)
	LogRegister(ctx context.Context, email string, ipAddress, userAgent string)
	LogPasswordReset(ctx context.Context, email string, ipAddress, userAgent string)
	LogPasswordChange(ctx context.Context, userID int64, ipAddress, userAgent string)
}

func NewHandler(service *Service, session *scs.SessionManager, dbConn *sql.DB, queries *db.Queries, auditLogger AuditLogger) *Handler {
	return &Handler{
		service:     service,
		session:     session,
		db:          dbConn,
		queries:     queries,
		auditLogger: auditLogger,
	}
}

// RegisterForm exibe formulário de registro
// @Summary Show registration form
// @Description Displays the user registration form
// @Tags Authentication
// @Produce html
// @Success 200 {string} string "Registration form HTML"
// @Router /register [get]
func (h *Handler) RegisterForm(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.Register("")).ServeHTTP(w, r)
}

// RegisterSubmit processa novo registro de usuário
// @Summary Register new user
// @Description Creates a new user account
// @Tags Authentication
// @Accept x-www-form-urlencoded
// @Produce html
// @Param email formData string true "User email"
// @Param password formData string true "User password"
// @Success 303 "Redirect to login on success"
// @Failure 200 {string} string "Registration form with error"
// @Router /register [post]
func (h *Handler) RegisterSubmit(w http.ResponseWriter, r *http.Request) error {
	// Timeout de 5 segundos para operações de database
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	email := r.FormValue("email")
	password := r.FormValue("password")
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	err := h.service.Register(ctx, RegisterInput{
		Email:    email,
		Password: password,
		TenantID: tenantID,
	})

	// Audit log
	h.auditLogger.LogRegister(ctx, email, r.RemoteAddr, r.UserAgent())

	if err != nil {
		// Metric
		metrics.AuthRegistrationsTotal.WithLabelValues("failure").Inc()
		templ.Handler(pages.Register(err.Error())).ServeHTTP(w, r)
		return nil
	}

	// Metric
	metrics.AuthRegistrationsTotal.WithLabelValues("success").Inc()

	http.Redirect(w, r, routes.Login+"?message=Conta criada!", http.StatusSeeOther)
	return nil
}

// LoginForm exibe formulário de login
// @Summary Show login form
// @Description Displays the user login form
// @Tags Authentication
// @Produce html
// @Success 200 {string} string "Login form HTML"
// @Router /login [get]
func (h *Handler) LoginForm(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.Login("")).ServeHTTP(w, r)
}

// LoginSubmit processa login de usuário
// @Summary Login user
// @Description Authenticates user and creates session
// @Tags Authentication
// @Accept x-www-form-urlencoded
// @Produce html
// @Param email formData string true "User email"
// @Param password formData string true "User password"
// @Success 303 "Redirect to dashboard on success"
// @Failure 200 {string} string "Login form with error"
// @Router /login [post]
func (h *Handler) LoginSubmit(w http.ResponseWriter, r *http.Request) error {
	// Timeout de 5 segundos para operações de database
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	email := r.FormValue("email")
	password := r.FormValue("password")
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	user, err := h.service.Login(ctx, LoginInput{
		Email:    email,
		Password: password,
		TenantID: tenantID,
	})

	if err != nil {
		// Audit log - login falhou
		h.auditLogger.LogLogin(ctx, 0, false, r.RemoteAddr, r.UserAgent())
		// Metric
		metrics.AuthLoginsTotal.WithLabelValues("failure").Inc()
		templ.Handler(pages.Login(err.Error())).ServeHTTP(w, r)
		return nil
	}

	// Metric
	metrics.AuthLoginsTotal.WithLabelValues("success").Inc()

	// Session regeneration para prevenir session fixation
	if err := h.session.RenewToken(ctx); err != nil {
		h.auditLogger.LogLogin(ctx, user.ID, false, r.RemoteAddr, r.UserAgent())
		httpErr.HandleError(w, r, err, "session_renew")
		return nil
	}
	h.session.Put(ctx, "user_id", user.ID)

	// Audit log - login bem-sucedido
	h.auditLogger.LogLogin(ctx, user.ID, true, r.RemoteAddr, r.UserAgent())

	http.Redirect(w, r, routes.Dashboard, http.StatusSeeOther)
	return nil
}

// Logout destrói sessão do usuário
// @Summary Logout user
// @Description Destroys user session and redirects to login
// @Tags Authentication
// @Produce html
// @Success 303 "Redirect to login"
// @Router /logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) error {
	// Get user_id before destroying session
	userID := h.session.GetInt64(r.Context(), "user_id")

	if err := h.session.Destroy(r.Context()); err != nil {
		return err
	}

	// Audit log
	if userID > 0 {
		h.auditLogger.LogLogout(r.Context(), userID, r.RemoteAddr, r.UserAgent())
	}

	http.Redirect(w, r, routes.Login, http.StatusSeeOther)
	return nil
}

// ForgotPasswordForm exibe formulário de recuperação de senha
// @Summary Show forgot password form
// @Description Displays the password recovery form
// @Tags Authentication
// @Produce html
// @Success 200 {string} string "Forgot password form HTML"
// @Router /forgot-password [get]
func (h *Handler) ForgotPasswordForm(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.ForgotPassword("")).ServeHTTP(w, r)
}

// ForgotPasswordSubmit processa pedido de recuperação de senha
// @Summary Request password reset
// @Description Sends password reset email if user exists
// @Tags Authentication
// @Accept x-www-form-urlencoded
// @Produce html
// @Param email formData string true "User email"
// @Success 200 {string} string "Confirmation message"
// @Router /forgot-password [post]
func (h *Handler) ForgotPasswordSubmit(w http.ResponseWriter, r *http.Request) error {
	// Timeout de 5 segundos para operações de database
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	email := r.FormValue("email")
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	err := h.service.ForgotPassword(ctx, ForgotPasswordInput{
		Email:    email,
		TenantID: tenantID,
	})

	// Audit log (sempre log, mesmo se email não existir - para segurança)
	h.auditLogger.LogPasswordReset(ctx, email, r.RemoteAddr, r.UserAgent())

	if err != nil {
		templ.Handler(pages.ForgotPassword(err.Error())).ServeHTTP(w, r)
		return nil
	}

	templ.Handler(pages.ForgotPassword("Se o e-mail existir, um link será enviado.")).ServeHTTP(w, r)
	return nil
}

// ResetPasswordForm exibe formulário de reset de senha
// @Summary Show reset password form
// @Description Displays the password reset form with token
// @Tags Authentication
// @Produce html
// @Param token query string true "Reset token"
// @Success 200 {string} string "Reset password form HTML"
// @Router /reset-password [get]
func (h *Handler) ResetPasswordForm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	templ.Handler(pages.ResetPassword(token, "")).ServeHTTP(w, r)
}

// ResetPasswordSubmit processa reset de senha
// @Summary Reset password
// @Description Resets user password using valid token
// @Tags Authentication
// @Accept x-www-form-urlencoded
// @Produce html
// @Param token formData string true "Reset token"
// @Param password formData string true "New password"
// @Success 303 "Redirect to login on success"
// @Failure 200 {string} string "Reset form with error"
// @Router /reset-password [post]
func (h *Handler) ResetPasswordSubmit(w http.ResponseWriter, r *http.Request) error {
	// Timeout de 5 segundos para operações de database
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	token := r.FormValue("token")
	password := r.FormValue("password")

	err := h.service.ResetPassword(ctx, ResetPasswordInput{
		Token:    token,
		Password: password,
	})

	if err != nil {
		templ.Handler(pages.ResetPassword(token, err.Error())).ServeHTTP(w, r)
		return nil
	}

	// Audit log - password changed
	// Note: não temos user_id aqui, será logado como 0
	h.auditLogger.LogPasswordChange(r.Context(), 0, r.RemoteAddr, r.UserAgent())

	http.Redirect(w, r, routes.Login+"?message=Senha alterada com sucesso", http.StatusSeeOther)
	return nil
}

// VerifyEmail verifica e-mail do usuário
// @Summary Verify email
// @Description Verifies user email using token
// @Tags Authentication
// @Produce html
// @Param token query string true "Verification token"
// @Success 303 "Redirect to login with success message"
// @Router /verify-email [get]
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) error {
	// Timeout de 5 segundos para operações de database
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, routes.Login+"?error=token_invalido", http.StatusSeeOther)
		return nil
	}

	if err := h.service.VerifyEmail(ctx, VerifyEmailInput{Token: token}); err != nil {
		http.Redirect(w, r, routes.Login+"?error=token_expirado", http.StatusSeeOther)
		return nil
	}

	http.Redirect(w, r, routes.Login+"?message=E-mail verificado com sucesso", http.StatusSeeOther)
	return nil
}

type AppHandler func(http.ResponseWriter, *http.Request) error

func Handle(h AppHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			httpErr.HandleError(w, r, err, "auth_handler")
		}
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, rateLimiter func(http.Handler) http.Handler) {
	mux.Handle("GET "+routes.Register, http.HandlerFunc(h.RegisterForm))
	mux.Handle("POST "+routes.Register, rateLimiter(http.HandlerFunc(Handle(h.RegisterSubmit))))
	mux.Handle("GET "+routes.Login, http.HandlerFunc(h.LoginForm))
	mux.Handle("POST "+routes.Login, rateLimiter(http.HandlerFunc(Handle(h.LoginSubmit))))
	mux.Handle("POST "+routes.Logout, http.HandlerFunc(Handle(h.Logout)))
	mux.Handle("GET "+routes.ForgotPassword, http.HandlerFunc(h.ForgotPasswordForm))
	mux.Handle("POST "+routes.ForgotPassword, rateLimiter(http.HandlerFunc(Handle(h.ForgotPasswordSubmit))))
	mux.Handle("GET "+routes.ResetPassword, http.HandlerFunc(h.ResetPasswordForm))
	mux.Handle("POST "+routes.ResetPassword, rateLimiter(http.HandlerFunc(Handle(h.ResetPasswordSubmit))))
	mux.Handle("GET "+routes.VerifyEmail, http.HandlerFunc(Handle(h.VerifyEmail)))
}
