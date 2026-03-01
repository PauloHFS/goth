package middleware

import (
	"context"
	"net/http"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/routes"
	"github.com/alexedwards/scs/v2"
)

// UserCache interface para cache de usuários
type UserCache interface {
	Get(userID int64) (db.User, bool)
	Set(userID int64, user db.User)
	Delete(userID int64)
	GetOrLoad(ctx context.Context, userID int64, loader func(context.Context, int64) (db.User, error)) (db.User, error)
}

func RequireAuth(sm *scs.SessionManager, queries *db.Queries, next http.Handler) http.Handler {
	return RequireAuthWithCache(sm, queries, nil, next)
}

// RequireAuthWithCache é como RequireAuth mas com cache opcional
func RequireAuthWithCache(sm *scs.SessionManager, queries *db.Queries, userCache UserCache, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := sm.GetInt64(r.Context(), "user_id")
		if userID == 0 {
			redirectLogin(w, r)
			return
		}

		var user db.User
		var err error

		// Try cache first if available
		if userCache != nil {
			user, err = userCache.GetOrLoad(r.Context(), userID, func(ctx context.Context, id int64) (db.User, error) {
				return queries.GetUserByID(ctx, id)
			})
		} else {
			// Direct DB query
			user, err = queries.GetUserByID(r.Context(), userID)
		}

		if err != nil {
			_ = sm.Destroy(r.Context())
			redirectLogin(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), contextkeys.UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func redirectLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", routes.Login)
	} else {
		http.Redirect(w, r, routes.Login, http.StatusSeeOther)
	}
}

// GetUser recupera o usuário do contexto de forma segura
func GetUser(ctx context.Context) (db.User, bool) {
	user, ok := ctx.Value(contextkeys.UserContextKey).(db.User)
	return user, ok
}
