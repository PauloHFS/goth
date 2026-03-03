package rbac

import (
	"context"
	"net/http"
	"strings"
)

// ContextKey for user info
type ContextKey string

const (
	UserIDKey    ContextKey = "user_id"
	UserRolesKey ContextKey = "user_roles"
)

// RBACMiddleware creates middleware for RBAC authorization
type RBACMiddleware struct {
	enforcer *Enforcer
}

// NewRBACMiddleware creates new RBAC middleware
func NewRBACMiddleware(enforcer *Enforcer) *RBACMiddleware {
	return &RBACMiddleware{
		enforcer: enforcer,
	}
}

// RequirePermission creates middleware that requires specific permission
func (m *RBACMiddleware) RequirePermission(resource string, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user ID from context
			userID, ok := r.Context().Value(UserIDKey).(string)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check permission
			if !m.enforcer.CheckPermission(userID, resource, action) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole creates middleware that requires specific role
func (m *RBACMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(UserIDKey).(string)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !m.enforcer.HasRole(userID, role) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole creates middleware that requires any of the specified roles
func (m *RBACMiddleware) RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(UserIDKey).(string)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			hasRole := false
			for _, role := range roles {
				if m.enforcer.HasRole(userID, role) {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AdminOnly creates middleware that requires admin role
func (m *RBACMiddleware) AdminOnly() func(http.Handler) http.Handler {
	return m.RequireRole("admin")
}

// CanView creates middleware for view permission
func (m *RBACMiddleware) CanView(resource string) func(http.Handler) http.Handler {
	return m.RequirePermission(resource, "view")
}

// CanCreate creates middleware for create permission
func (m *RBACMiddleware) CanCreate(resource string) func(http.Handler) http.Handler {
	return m.RequirePermission(resource, "create")
}

// CanEdit creates middleware for edit permission
func (m *RBACMiddleware) CanEdit(resource string) func(http.Handler) http.Handler {
	return m.RequirePermission(resource, "edit")
}

// CanDelete creates middleware for delete permission
func (m *RBACMiddleware) CanDelete(resource string) func(http.Handler) http.Handler {
	return m.RequirePermission(resource, "delete")
}

// WithUser creates middleware that adds user to context
func (m *RBACMiddleware) WithUser(getUserID func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := getUserID(r)
			if userID != "" {
				ctx := context.WithValue(r.Context(), UserIDKey, userID)

				// Get user roles
				roles, _ := m.enforcer.GetRoles(userID)
				ctx = context.WithValue(ctx, UserRolesKey, roles)

				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ExtractUserFromSession extracts user ID from session (example implementation)
func ExtractUserFromSession(r *http.Request) string {
	// Implement based on your session management
	// Example with cookies:
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}

	// Look up session in database/cache
	// Return user ID
	// This is just a placeholder
	return cookie.Value
}

// ExtractUserFromHeader extracts user ID from header
func ExtractUserFromHeader(headerName string) func(*http.Request) string {
	return func(r *http.Request) string {
		return r.Header.Get(headerName)
	}
}

// ExtractUserFromJWT extracts user ID from JWT token
func ExtractUserFromJWT() func(*http.Request) string {
	return func(r *http.Request) string {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			return ""
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return ""
		}

		// Parse JWT and extract user ID
		// Implementation depends on your JWT library
		// This is a placeholder
		return ""
	}
}

// HasPermissionHandler creates HTTP handler that checks permission
func HasPermissionHandler(enforcer *Enforcer, resource string, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !enforcer.CheckPermission(userID, resource, action) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// GetPermissionsHandler returns user permissions
func GetPermissionsHandler(enforcer *Enforcer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		permissions, err := enforcer.GetUserPermissions(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Convert to JSON-friendly format
		result := make([]map[string]string, len(permissions))
		for i, perm := range permissions {
			if len(perm) >= 3 {
				result[i] = map[string]string{
					"resource": perm[1],
					"action":   perm[2],
				}
			}
		}

		// Write JSON response
		if _, err := w.Write([]byte(`{"permissions":`)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Simple JSON encoding (use proper JSON encoder in production)
		for i, perm := range result {
			if i > 0 {
				if _, err := w.Write([]byte(",")); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			if _, err := w.Write([]byte(`{"resource":"` + perm["resource"] + `","action":"` + perm["action"] + `"}`)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if _, err := w.Write([]byte(`}`)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
