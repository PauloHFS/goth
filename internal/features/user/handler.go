package user

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/PauloHFS/goth/internal/db"
	httpErr "github.com/PauloHFS/goth/internal/platform/http"
	"github.com/PauloHFS/goth/internal/platform/http/middleware"
	"github.com/PauloHFS/goth/internal/view"
	"github.com/PauloHFS/goth/internal/view/pages"
	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
)

type Handler struct {
	userRepo UserRepository
	session  *scs.SessionManager
	db       *sql.DB
	queries  *db.Queries
}

func NewHandler(userRepo UserRepository, session *scs.SessionManager, dbConn *sql.DB, queries *db.Queries) *Handler {
	return &Handler{
		userRepo: userRepo,
		session:  session,
		db:       dbConn,
		queries:  queries,
	}
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by RequireAuth middleware)
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// List users for display
	users, total, err := h.userRepo.ListPaginated(r.Context(), ListParams{
		TenantID: "default",
		Search:   r.URL.Query().Get("search"),
		Page:     1,
		PerPage:  10,
	})
	if err != nil {
		users = []db.User{user}
		total = 1
	}

	pag := view.NewPagination(1, int(total), 10)
	templ.Handler(pages.Dashboard(user, users, pag)).ServeHTTP(w, r)
}

func (h *Handler) AvatarUpload(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by RequireAuth middleware)
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		httpErr.HandleError(w, r, httpErr.NewValidationError("failed to read avatar file", nil), "upload_avatar")
		return
	}
	defer func() { _ = file.Close() }()

	if header.Size > 2*1024*1024 {
		httpErr.HandleError(w, r, httpErr.NewValidationError("file too large (max 2MB)", nil), "upload_avatar")
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" {
		httpErr.HandleError(w, r, httpErr.NewValidationError("invalid file type", nil), "upload_avatar")
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		httpErr.HandleError(w, r, httpErr.NewValidationError("failed to read file data", nil), "upload_avatar")
		return
	}

	// Create avatars directory if not exists
	avatarDir := "storage/avatars"
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		httpErr.HandleError(w, r, fmt.Errorf("failed to create avatar directory: %w", err), "upload_avatar")
		return
	}

	// Save file to disk
	filename := fmt.Sprintf("%d%s", user.ID, ext)
	filePath := filepath.Join(avatarDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		httpErr.HandleError(w, r, fmt.Errorf("failed to save avatar file: %w", err), "upload_avatar")
		return
	}

	url := "/storage/avatars/" + filename

	if err := h.userRepo.UpdateAvatar(r.Context(), user.ID, url); err != nil {
		httpErr.HandleError(w, r, err, "update_avatar")
		return
	}

	// Redirect back to dashboard
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		// Handle profile update
		name := r.FormValue("name")
		bio := r.FormValue("bio")
		_ = name // TODO: Update user profile
		_ = bio
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	templ.Handler(pages.Profile(user)).ServeHTTP(w, r)
}

func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	templ.Handler(pages.Settings(user)).ServeHTTP(w, r)
}

func (h *Handler) Notifications(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	// TODO: Load actual notifications
	notifications := []pages.Notification{}
	templ.Handler(pages.Notifications(user, notifications)).ServeHTTP(w, r)
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	auth := middleware.RequireAuth(h.session, h.queries, http.HandlerFunc(h.Dashboard))
	mux.Handle("GET /dashboard", auth)

	profileAuth := middleware.RequireAuth(h.session, h.queries, http.HandlerFunc(h.Profile))
	mux.Handle("GET /profile", profileAuth)
	mux.Handle("POST /profile", profileAuth)

	settingsAuth := middleware.RequireAuth(h.session, h.queries, http.HandlerFunc(h.Settings))
	mux.Handle("GET /settings", settingsAuth)

	notificationsAuth := middleware.RequireAuth(h.session, h.queries, http.HandlerFunc(h.Notifications))
	mux.Handle("GET /notifications", notificationsAuth)

	avatarAuth := middleware.RequireAuth(h.session, h.queries, http.HandlerFunc(h.AvatarUpload))
	mux.Handle("POST /profile/avatar", avatarAuth)
}
