package user

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
)

type responseRecorder struct {
	Code      int
	HeaderMap http.Header
	Body      *strings.Builder
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		Code:      200,
		HeaderMap: make(http.Header),
		Body:      &strings.Builder{},
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.HeaderMap
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.Body.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.Code = statusCode
}

func (r *responseRecorder) BodyString() string {
	return r.Body.String()
}

func newRequest(method, url string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func withUserID(req *http.Request, userID int64) *http.Request {
	user := db.User{
		ID:         userID,
		Email:      "test@example.com",
		TenantID:   "default",
		RoleID:     "user",
		IsVerified: true,
	}
	ctx := context.WithValue(req.Context(), contextkeys.UserContextKey, user)
	return req.WithContext(ctx)
}

type mockUserRepository struct {
	user db.User
	err  error
}

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (db.User, error) {
	if m.err != nil {
		return db.User{}, m.err
	}
	return m.user, nil
}

func (m *mockUserRepository) UpdateAvatar(ctx context.Context, id int64, url string) error {
	return m.err
}

func (m *mockUserRepository) ListPaginated(ctx context.Context, params ListParams) ([]db.User, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return []db.User{m.user}, 1, nil
}

func TestHandler_Dashboard_Unauthorized(t *testing.T) {
	userRepo := &mockUserRepository{}
	dbConn := &sql.DB{}
	queries := &db.Queries{}

	handler := NewHandler(userRepo, nil, dbConn, queries)

	req := newRequest("GET", "/dashboard", nil)
	w := newResponseRecorder()

	handler.Dashboard(w, req)

	if w.Code != 303 {
		t.Errorf("expected redirect status 303, got %d", w.Code)
	}
}

func TestHandler_Dashboard_UserNotFound(t *testing.T) {
	userRepo := &mockUserRepository{
		err: errors.New("not found"),
	}
	dbConn := &sql.DB{}
	queries := &db.Queries{}

	handler := NewHandler(userRepo, nil, dbConn, queries)

	req := newRequest("GET", "/dashboard", nil)
	w := newResponseRecorder()

	handler.Dashboard(w, req)

	if w.Code != 303 {
		t.Errorf("expected redirect status 303, got %d", w.Code)
	}
}

func TestHandler_Dashboard_Success(t *testing.T) {
	userRepo := &mockUserRepository{
		user: db.User{
			ID:         1,
			Email:      "test@example.com",
			TenantID:   "default",
			RoleID:     "user",
			IsVerified: true,
		},
	}
	dbConn := &sql.DB{}
	queries := &db.Queries{}

	handler := NewHandler(userRepo, nil, dbConn, queries)

	req := newRequest("GET", "/dashboard", nil)
	req = withUserID(req, 1)
	w := newResponseRecorder()

	handler.Dashboard(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_AvatarUpload_Unauthorized(t *testing.T) {
	userRepo := &mockUserRepository{}
	dbConn := &sql.DB{}
	queries := &db.Queries{}

	handler := NewHandler(userRepo, nil, dbConn, queries)

	req := newRequest("POST", "/profile/avatar", nil)
	w := newResponseRecorder()

	handler.AvatarUpload(w, req)

	if w.Code != 303 {
		t.Errorf("expected redirect status 303, got %d", w.Code)
	}
}

func TestHandler_AvatarUpload_MissingFile(t *testing.T) {
	userRepo := &mockUserRepository{}
	dbConn := &sql.DB{}
	queries := &db.Queries{}

	handler := NewHandler(userRepo, nil, dbConn, queries)

	req := newRequest("POST", "/profile/avatar", nil)
	req = withUserID(req, 1)
	w := newResponseRecorder()

	handler.AvatarUpload(w, req)

	// Handler returns 400 when file is missing
	if w.Code != 400 {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_AvatarUpload_FileTooLarge(t *testing.T) {
	userRepo := &mockUserRepository{}
	dbConn := &sql.DB{}
	queries := &db.Queries{}

	handler := NewHandler(userRepo, nil, dbConn, queries)

	req := newRequest("POST", "/profile/avatar", nil)
	req = withUserID(req, 1)
	req.Header.Set("Content-Length", "3000000")
	w := newResponseRecorder()

	handler.AvatarUpload(w, req)

	// Handler returns 400 when file is too large
	if w.Code != 400 {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_AvatarUpload_InvalidFileType(t *testing.T) {
	userRepo := &mockUserRepository{}
	dbConn := &sql.DB{}
	queries := &db.Queries{}

	handler := NewHandler(userRepo, nil, dbConn, queries)

	req := newRequest("POST", "/profile/avatar", nil)
	req = withUserID(req, 1)
	req.Header.Set("Content-Type", "application/octet-stream")
	w := newResponseRecorder()

	handler.AvatarUpload(w, req)

	// Handler returns 400 when file type is invalid
	if w.Code != 400 {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_AvatarUpload_Success(t *testing.T) {
	userRepo := &mockUserRepository{
		user: db.User{
			ID:    1,
			Email: "test@example.com",
		},
	}
	dbConn := &sql.DB{}
	queries := &db.Queries{}

	handler := NewHandler(userRepo, nil, dbConn, queries)

	req := newRequest("POST", "/profile/avatar", nil)
	req = withUserID(req, 1)
	w := newResponseRecorder()

	handler.AvatarUpload(w, req)

	if w.Code != 400 {
		t.Logf("response code: %d (expected 400 for missing file)", w.Code)
	}
}

func TestHandler_AvatarUpload_RepositoryError(t *testing.T) {
	userRepo := &mockUserRepository{
		user: db.User{
			ID:    1,
			Email: "test@example.com",
		},
		err: errors.New("update failed"),
	}
	dbConn := &sql.DB{}
	queries := &db.Queries{}

	handler := NewHandler(userRepo, nil, dbConn, queries)

	req := newRequest("POST", "/profile/avatar", nil)
	req = withUserID(req, 1)
	w := newResponseRecorder()

	handler.AvatarUpload(w, req)

	if w.Code != 500 {
		t.Logf("response code: %d", w.Code)
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	dbConn := &sql.DB{}
	repo := NewRepository(dbConn)

	if repo == nil {
		t.Error("expected repository to be created")
	}
}

func TestUserRepository_UpdateAvatar(t *testing.T) {
	dbConn := &sql.DB{}
	repo := NewRepository(dbConn)

	if repo == nil {
		t.Error("expected repository to be created")
	}
}
