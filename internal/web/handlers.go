package web

import (
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PauloHFS/goth/internal/config"
	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/logging"
	"github.com/PauloHFS/goth/internal/middleware"
	"github.com/PauloHFS/goth/internal/routes"
	"github.com/PauloHFS/goth/internal/validator"
	"github.com/PauloHFS/goth/internal/view"
	"github.com/PauloHFS/goth/internal/view/pages"
	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"golang.org/x/crypto/bcrypt"
)

type HandlerDeps struct {
	DB             *sql.DB
	Queries        *db.Queries
	SessionManager *scs.SessionManager
	Config         *config.Config
}

// AppHandler é um tipo customizado que permite retornar erros dos handlers
type AppHandler func(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error

// Handle envolve nosso AppHandler para conformidade com http.HandlerFunc
func Handle(deps HandlerDeps, h AppHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(deps, w, r); err != nil {
			logging.Get().Error("request failed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Any("error", err),
			)

			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func RegisterRoutes(mux *http.ServeMux, deps HandlerDeps) {
	// Auth Handlers
	mux.Handle("GET "+routes.Login, templ.Handler(pages.Login("")))
	mux.Handle("GET "+routes.Register, templ.Handler(pages.Register("")))

	mux.HandleFunc("POST "+routes.Register, Handle(deps, handleRegister))
	mux.HandleFunc("GET "+routes.ForgotPassword, func(w http.ResponseWriter, r *http.Request) {
		templ.Handler(pages.ForgotPassword("")).ServeHTTP(w, r)
	})
	mux.HandleFunc("POST "+routes.ForgotPassword, Handle(deps, handleForgotPassword))
	mux.HandleFunc("GET "+routes.ResetPassword, func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		templ.Handler(pages.ResetPassword(token, "")).ServeHTTP(w, r)
	})
	mux.HandleFunc("POST "+routes.ResetPassword, Handle(deps, handleResetPassword))
	mux.HandleFunc("GET "+routes.VerifyEmail, Handle(deps, handleVerifyEmail))
	mux.HandleFunc("POST "+routes.Login, Handle(deps, handleLogin))
	mux.HandleFunc("POST "+routes.Logout, Handle(deps, handleLogout))

	// Protected Routes
	mux.Handle("GET "+routes.Dashboard, middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleDashboard)))
	mux.Handle("GET "+routes.Admin, middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleAdmin)))
	mux.Handle("POST /profile/avatar", middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleAvatarUpload)))

	// Public Routes
	mux.HandleFunc("GET "+routes.Home, func(w http.ResponseWriter, r *http.Request) {
		logging.AddToEvent(r.Context(), slog.String("business_unit", "marketing"))
		_, _ = w.Write([]byte("GOTH Stack Running"))
	})
}

// --- Handler Implementations ---

func handleRegister(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	email := r.FormValue("email")
	password := r.FormValue("password")

	emailDomain := ""
	if idx := strings.Index(email, "@"); idx > 0 {
		emailDomain = email[idx+1:]
	}

	logging.AddToEvent(r.Context(),
		slog.String("operation", "register"),
		slog.String("email_domain", emailDomain),
	)

	validation := validator.ValidateRegistration(email, password)
	if !validation.Valid {
		errorMsg := ""
		for _, err := range validation.Errors {
			errorMsg += err.Message + " "
		}
		templ.Handler(pages.Register(strings.TrimSpace(errorMsg))).ServeHTTP(w, r)
		return nil
	}

	_, err := deps.Queries.GetUserByEmail(r.Context(), db.GetUserByEmailParams{
		TenantID: "default",
		Email:    email,
	})
	if err == nil {
		logging.AddToEvent(r.Context(),
			slog.String("outcome", "error"),
			slog.String("error_reason", "email_already_exists"),
		)
		templ.Handler(pages.Register("Este e-mail já está em uso")).ServeHTTP(w, r)
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	tx, err := deps.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	qtx := deps.Queries.WithTx(tx)

	user, err := qtx.CreateUser(r.Context(), db.CreateUserParams{
		TenantID:     "default",
		Email:        email,
		PasswordHash: string(hash),
		RoleID:       "user",
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	logging.AddToEvent(r.Context(),
		slog.Int64("created_user_id", user.ID),
	)

	tokenBytes := make([]byte, 32)
	if _, err := crypto_rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	if err := qtx.UpsertEmailVerification(r.Context(), db.UpsertEmailVerificationParams{
		Email:     email,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		return fmt.Errorf("failed to create verification: %w", err)
	}

	jobPayload, err := json.Marshal(map[string]string{
		"email": email,
		"token": token,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}

	if _, err := qtx.CreateJob(r.Context(), db.CreateJobParams{
		TenantID: sql.NullString{String: "default", Valid: true},
		Type:     "send_verification_email",
		Payload:  jobPayload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit registration: %w", err)
	}

	logging.AddToEvent(r.Context(),
		slog.String("outcome", "success"),
	)

	http.Redirect(w, r, routes.Login+"?message=Conta criada! Verifique seu e-mail.", http.StatusSeeOther)
	return nil
}

func handleForgotPassword(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	email := r.FormValue("email")

	if err := validator.ValidateEmail(email); err != nil {
		templ.Handler(pages.ForgotPassword(err.Error())).ServeHTTP(w, r)
		return nil
	}

	_, err := deps.Queries.GetUserByEmail(r.Context(), db.GetUserByEmailParams{
		TenantID: "default",
		Email:    email,
	})
	if err != nil {
		templ.Handler(pages.ForgotPassword("Se o e-mail existir, um link será enviado.")).ServeHTTP(w, r)
		return nil
	}

	tokenBytes := make([]byte, 32)
	if _, err := crypto_rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	tx, err := deps.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	qtx := deps.Queries.WithTx(tx)

	if err := qtx.UpsertPasswordReset(r.Context(), db.UpsertPasswordResetParams{
		Email:     email,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}); err != nil {
		return fmt.Errorf("failed to create password reset: %w", err)
	}

	jobPayload, err := json.Marshal(map[string]string{
		"email": email,
		"token": token,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}
	if _, err := qtx.CreateJob(r.Context(), db.CreateJobParams{
		TenantID: sql.NullString{String: "default", Valid: true},
		Type:     "send_password_reset_email",
		Payload:  jobPayload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit forgot password: %w", err)
	}

	templ.Handler(pages.ForgotPassword("Se o e-mail existir, um link será enviado.")).ServeHTTP(w, r)
	return nil
}

func handleResetPassword(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	token := r.FormValue("token")
	password := r.FormValue("password")

	if err := validator.ValidatePassword(password); err != nil {
		templ.Handler(pages.ResetPassword(token, err.Error())).ServeHTTP(w, r)
		return nil
	}

	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	reset, err := deps.Queries.GetPasswordResetByToken(r.Context(), tokenHash)
	if err != nil || reset.ExpiresAt.Before(time.Now()) {
		templ.Handler(pages.ResetPassword(token, "Link inválido ou expirado")).ServeHTTP(w, r)
		return nil
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	tx, err := deps.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	qtx := deps.Queries.WithTx(tx)

	err = qtx.UpdateUserPassword(r.Context(), db.UpdateUserPasswordParams{
		PasswordHash: string(newHash),
		Email:        reset.Email,
	})
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	if err := qtx.DeletePasswordReset(r.Context(), reset.Email); err != nil {
		logging.Get().Warn("failed to delete password reset token", "error", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit password reset: %w", err)
	}

	http.Redirect(w, r, routes.Login+"?message=Senha alterada com sucesso", http.StatusSeeOther)
	return nil
}

func handleVerifyEmail(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, routes.Login+"?error=token_invalido", http.StatusSeeOther)
		return nil
	}

	verification, err := deps.Queries.GetEmailVerificationByToken(r.Context(), token)
	if err != nil || verification.ExpiresAt.Before(time.Now()) {
		http.Redirect(w, r, routes.Login+"?error=token_expirado", http.StatusSeeOther)
		return nil
	}

	tx, err := deps.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	qtx := deps.Queries.WithTx(tx)

	err = qtx.VerifyUser(r.Context(), verification.Email)
	if err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}

	if err := qtx.DeleteEmailVerification(r.Context(), verification.Email); err != nil {
		logging.Get().Warn("failed to delete email verification token", "error", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit email verification: %w", err)
	}

	http.Redirect(w, r, routes.Login+"?message=E-mail verificado com sucesso", http.StatusSeeOther)
	return nil
}

func handleLogin(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	email := r.FormValue("email")
	password := r.FormValue("password")

	emailDomain := ""
	if idx := strings.Index(email, "@"); idx > 0 {
		emailDomain = email[idx+1:]
	}

	logging.AddToEvent(r.Context(),
		slog.String("operation", "login"),
		slog.String("email_domain", emailDomain),
	)

	if email == "" || password == "" {
		logging.AddToEvent(r.Context(),
			slog.String("outcome", "error"),
			slog.String("error_reason", "missing_credentials"),
		)
		templ.Handler(pages.Login("Email e senha são obrigatórios")).ServeHTTP(w, r)
		return nil
	}

	user, err := deps.Queries.GetUserByEmail(r.Context(), db.GetUserByEmailParams{
		TenantID: "default",
		Email:    email,
	})

	if err != nil {
		logging.AddToEvent(r.Context(),
			slog.String("outcome", "error"),
			slog.String("error_reason", "user_not_found"),
		)
		templ.Handler(pages.Login("Usuário ou senha inválidos")).ServeHTTP(w, r)
		return nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		logging.AddToEvent(r.Context(),
			slog.String("outcome", "error"),
			slog.String("error_reason", "invalid_password"),
			slog.Int64("user_id", user.ID),
		)
		templ.Handler(pages.Login("Usuário ou senha inválidos")).ServeHTTP(w, r)
		return nil
	}

	logging.AddToEvent(r.Context(),
		slog.String("outcome", "success"),
		slog.Int64("user_id", user.ID),
		slog.String("user_role", user.RoleID),
	)

	deps.SessionManager.Put(r.Context(), "user_id", user.ID)
	http.Redirect(w, r, routes.Dashboard, http.StatusSeeOther)
	return nil
}

func handleLogout(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	if err := deps.SessionManager.Destroy(r.Context()); err != nil {
		return fmt.Errorf("failed to destroy session: %w", err)
	}
	http.Redirect(w, r, routes.Login, http.StatusSeeOther)
	return nil
}

func handleDashboard(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, _ := r.Context().Value(contextkeys.UserContextKey).(db.User)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	search := r.URL.Query().Get("search")

	paging := db.PagingParams{
		Page:    page,
		PerPage: 5,
	}

	users, err := deps.Queries.ListUsersPaginated(r.Context(), db.ListUsersPaginatedParams{
		TenantID: "default",
		Column2:  sql.NullString{String: search, Valid: true},
		Column3:  sql.NullString{String: search, Valid: true},
		Limit:    int64(paging.Limit()),
		Offset:   int64(paging.Offset()),
	})
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	totalUsers, err := deps.Queries.CountUsers(r.Context(), "default")
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	result := db.PagedResult[db.User]{
		Items:       users,
		TotalItems:  int(totalUsers),
		CurrentPage: paging.Page,
		PerPage:     paging.PerPage,
	}

	pagHelper := view.NewPagination(result.CurrentPage, result.TotalItems, result.PerPage)
	templ.Handler(pages.Dashboard(user, result.Items, pagHelper)).ServeHTTP(w, r)
	return nil
}

func handleAvatarUpload(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	logging.AddToEvent(r.Context(),
		slog.String("operation", "avatar_upload"),
		slog.Int64("user_id", user.ID),
	)

	if err := r.ParseMultipartForm(2 << 20); err != nil {
		logging.AddToEvent(r.Context(),
			slog.String("outcome", "error"),
			slog.String("error_reason", "parse_form_failed"),
		)
		return fmt.Errorf("failed to parse multipart form: %w", err)
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		logging.AddToEvent(r.Context(),
			slog.String("outcome", "error"),
			slog.String("error_reason", "no_file_provided"),
		)
		http.Error(w, "invalid file", http.StatusBadRequest)
		return nil
	}
	defer file.Close()

	if err := validator.ValidateUpload(header.Filename, header.Header.Get("Content-Type"), 2<<20); err != nil {
		logging.AddToEvent(r.Context(),
			slog.String("outcome", "error"),
			slog.String("error_reason", "validation_failed"),
			slog.String("filename", header.Filename),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	filename := fmt.Sprintf("%d_%d%s", user.ID, time.Now().Unix(), ext)
	dstPath := filepath.Join("storage", "avatars", filename)

	logging.AddToEvent(r.Context(),
		slog.String("file_extension", ext),
		slog.Int64("file_size", header.Size),
	)

	dst, err := os.Create(dstPath)
	if err != nil {
		logging.AddToEvent(r.Context(),
			slog.String("outcome", "error"),
			slog.String("error_reason", "file_creation_failed"),
		)
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	avatarURL := "/storage/avatars/" + filename
	if err := deps.Queries.UpdateUserAvatar(r.Context(), db.UpdateUserAvatarParams{
		AvatarUrl: sql.NullString{String: avatarURL, Valid: true},
		ID:        user.ID,
	}); err != nil {
		logging.Get().Warn("failed to update avatar in database", "error", err)
	}

	jobPayload, _ := json.Marshal(map[string]string{"image": avatarURL})
	if _, err := deps.Queries.CreateJob(r.Context(), db.CreateJobParams{
		TenantID: sql.NullString{String: fmt.Sprintf("%d", user.ID), Valid: true},
		Type:     "process_ai",
		Payload:  jobPayload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}); err != nil {
		logging.AddToEvent(r.Context(),
			slog.String("outcome", "partial_success"),
			slog.String("error_reason", "job_creation_failed"),
		)
	}

	logging.AddToEvent(r.Context(),
		slog.String("outcome", "success"),
		slog.String("avatar_url", avatarURL),
	)

	http.Redirect(w, r, routes.Dashboard, http.StatusSeeOther)
	return nil
}

func handleAdmin(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, _ := r.Context().Value(contextkeys.UserContextKey).(db.User)

	if user.RoleID != "admin" {
		http.Redirect(w, r, routes.Dashboard, http.StatusForbidden)
		return nil
	}

	totalUsers, _ := deps.Queries.CountUsers(r.Context(), "default")

	var totalDeadLetters int64
	if deps.Queries != nil {
		count, err := deps.Queries.CountDeadLetterJobs(r.Context())
		if err == nil {
			totalDeadLetters = count
		}
	}

	stats := pages.AdminStats{
		TotalUsers:       totalUsers,
		TotalDeadLetters: totalDeadLetters,
		ActiveJobs:       0,
		FailedJobs:       0,
		RateLimits: map[string]pages.RateLimitStatus{
			"default": {RequestsPerSecond: 10, Burst: 20, CurrentCount: 0},
			"auth":    {RequestsPerSecond: 5, Burst: 10, CurrentCount: 0},
			"api":     {RequestsPerSecond: 20, Burst: 40, CurrentCount: 0},
			"webhook": {RequestsPerSecond: 100, Burst: 200, CurrentCount: 0},
			"upload":  {RequestsPerSecond: 2, Burst: 5, CurrentCount: 0},
		},
		EmailProviders: []pages.EmailProviderStatus{
			{Name: "SMTP", Type: "smtp", Enabled: true, LastUsed: "N/A"},
		},
	}

	templ.Handler(pages.Admin(user, stats)).ServeHTTP(w, r)
	return nil
}
