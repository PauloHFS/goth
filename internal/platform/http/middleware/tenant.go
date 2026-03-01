package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/PauloHFS/goth/internal/contextkeys"
)

const DefaultTenantID = "default"

func TenantExtractor(defaultTenantID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := extractTenantID(r)

			if tenantID == "" {
				tenantID = defaultTenantID
			}

			ctx := context.WithValue(r.Context(), contextkeys.TenantContextKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractTenantID(r *http.Request) string {
	// Priority 1: X-Tenant-ID header (for API/mobile clients)
	if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
		return strings.TrimSpace(tenantID)
	}

	// Priority 2: Subdomain (e.g., acme.localhost:8080)
	host := r.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		// Check if it's localhost or similar development domain
		isLocalhost := len(parts) >= 2 && (parts[0] == "localhost" || parts[len(parts)-2] == "localhost")

		if !isLocalhost {
			// e.g., acme.example.com -> tenant = acme
			// e.g., app.acme.example.com -> tenant = app.acme (full subdomain)
			if len(parts) >= 3 {
				tenantID := strings.Join(parts[:len(parts)-2], ".")
				if tenantID != "" && tenantID != "www" {
					return tenantID
				}
			}
		}
	}

	// Priority 3: Path-based (e.g., /tenant/dashboard)
	// This is less common but can be useful
	path := r.URL.Path
	if strings.HasPrefix(path, "/tenant/") {
		parts := strings.Split(path[8:], "/")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
	}

	return ""
}

func GetTenantID(ctx context.Context) string {
	if tenantID, ok := ctx.Value(contextkeys.TenantContextKey).(string); ok {
		return tenantID
	}
	return DefaultTenantID
}
