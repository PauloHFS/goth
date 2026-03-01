package user

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/PauloHFS/goth/internal/db"
	httpErr "github.com/PauloHFS/goth/internal/platform/http"
	httpMiddleware "github.com/PauloHFS/goth/internal/platform/http/middleware"
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
	userID := r.Context().Value("user_id")
	if userID == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID.(int64))
	if err != nil {
		httpErr.HandleError(w, r, httpErr.NewNotFoundError("user"), "get_user")
		return
	}

	users := []db.User{user}
	pag := view.NewPagination(1, 1, 10)
	templ.Handler(pages.Dashboard(user, users, pag)).ServeHTTP(w, r)
}

func (h *Handler) AvatarUpload(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
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

	_ = data

	filename := fmt.Sprintf("avatars/%d%s", userID.(int64), ext)
	url := "/storage/" + filename

	if err := h.userRepo.UpdateAvatar(r.Context(), userID.(int64), url); err != nil {
		httpErr.HandleError(w, r, err, "update_avatar")
		return
	}

	if _, err := w.Write([]byte("Avatar updated!")); err != nil {
		return
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	auth := httpMiddleware.RequireAuth(h.session, h.queries, http.HandlerFunc(h.Dashboard))
	mux.Handle("GET /dashboard", auth)
	mux.HandleFunc("POST /profile/avatar", h.AvatarUpload)
}
